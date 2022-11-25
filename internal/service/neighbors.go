package service

import (
	"context"
	mygrpc "fiveServices/grpc"
	"fiveServices/internal/grpclog"
	"fiveServices/internal/server"
	"fiveServices/internal/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"io"
	"strconv"
	"sync"
	"time"
)

type Neighbors struct {
	serviceID              int
	portsToLookUp          []int
	opts                   []grpc.DialOption
	frequency              time.Duration
	timeout                time.Duration
	neighborsToCommunicate map[int]*neighborsSync
	m                      sync.Mutex
	server                 *server.Server
}

type neighborsSync struct {
	isAvailable bool
	isRunning   bool
}

func NewNeighbors(id int, portsToLookUp []int, frequency time.Duration, server *server.Server) *Neighbors {
	nf := Neighbors{
		serviceID:              id,
		portsToLookUp:          portsToLookUp,
		frequency:              frequency,
		timeout:                time.Second * 1,
		neighborsToCommunicate: make(map[int]*neighborsSync),
		m:                      sync.Mutex{},
		server:                 server,
	}
	nf.opts = append(
		nf.opts,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	ch := make(chan struct{})
	go func() {
		<-ch
		defer func() {
			close(ch)
		}()
		wg := sync.WaitGroup{}
		ticker := time.NewTicker(nf.frequency)
		count := 0
		for range ticker.C {
			if count > 1e3 { // stop condition
				ticker.Stop()
				break
			}
			for _, p := range nf.portsToLookUp {
				p := strconv.Itoa(p)
				nf.m.Lock()
				if status, ok := nf.server.StatusMap[p]; ok {
					if status == mygrpc.HealthCheckResponse_SERVING {
						nf.m.Unlock()
						continue
					}
				}
				nf.m.Unlock()
				wg.Add(1)
				count++
				go func(waitGroup *sync.WaitGroup, port string) {
					conn, err := grpc.Dial(utils.GetServiceName(port), nf.opts...)
					if err != nil {
						grpclog.Info(err)
					} else {
						client := mygrpc.NewHealthClient(conn)
						resp, err := client.Check(
							context.Background(),
							&mygrpc.HealthCheckRequest{})
						if resp == nil {
							// don't understand why defer do not work
							waitGroup.Done()
							if conn != nil {
								err := conn.Close()
								if err != nil {
									grpclog.Error(err)
								}
							}
							return
						}
						// could be better
						if resp.Status != mygrpc.HealthCheckResponse_SERVING ||
							resp.Status != mygrpc.HealthCheckResponse_NOT_SERVING ||
							err != nil {
							grpclog.Info("can't connect grpc server: %v, code: %v\n", err, grpc.Code(err))
						}
						nf.m.Lock()
						nf.server.StatusMap[port] = mygrpc.HealthCheckResponse_NOT_SERVING
						nf.m.Unlock()
					}
					defer func() {
						waitGroup.Done()
						if conn != nil {
							err := conn.Close()
							if err != nil {
								grpclog.Error(err)
							}
						}
					}()
				}(&wg, p)
			}
			wg.Wait()
		}
	}()
	ch <- struct{}{}

	return &nf
}

func (nf *Neighbors) LookUp() {
	for port, neighborSync := range nf.server.StatusMap {
		if neighborSync == mygrpc.HealthCheckResponse_SERVING || strconv.Itoa(nf.serviceID) == port {
			continue
		}
		p := port
		go func(port string) {
			conn, err := grpc.Dial(utils.GetServiceName(port), nf.opts...)
			if err != nil {
				grpclog.Error("fail to dial: " + err.Error())
			}
			nf.m.Lock()
			client := mygrpc.NewMyServiceClient(conn)
			clientHealth := mygrpc.NewHealthClient(conn)
			resp, err := clientHealth.Connected(
				context.Background(),
				&mygrpc.HealthCheckRequest{Service: strconv.Itoa(nf.serviceID)})
			if resp == nil || resp.Ok == false {
				grpclog.Info("problems to is say it connected")
				return
			}
			nf.server.StatusMap[port] = mygrpc.HealthCheckResponse_SERVING
			nf.communicate(client)
			nf.m.Unlock()
			defer func(conn *grpc.ClientConn) {
				err := conn.Close()
				if err != nil {
					grpclog.Error(err)
				}
				nf.m.Lock()
				delete(nf.server.StatusMap, port)
				nf.m.Unlock()
			}(conn)
		}(p)
	}
}

func (nf *Neighbors) communicate(client mygrpc.MyServiceClient) {
	ctx, cancel := context.WithTimeout(context.Background(), nf.timeout)
	defer cancel()
	stream, err := client.SendRandString(ctx)
	if err != nil {
		grpclog.Error("client.SendRandString failed:v" + err.Error())
	}
	waitc := make(chan struct{})
	go func() {
		for {
			myMessage, errRecv := stream.Recv()
			if errRecv == io.EOF {
				// read done.
				close(waitc)
				return
			}
			if errRecv != nil {
				grpclog.Error("client.Recv failed: " + errRecv.Error())
			}
			if myMessage == nil {
				continue
			}
			grpclog.Info("Got message " + myMessage.Message + " from " + strconv.Itoa(int(myMessage.ServiceID)) + " service")
		}
	}()
	for _, message := range utils.GetRandStringSlice() {
		m := &mygrpc.MyMessage{
			ServiceID: int32(nf.serviceID),
			Message:   message,
		}
		if err = stream.Send(m); err != nil {
			grpclog.Error("SendMessage failed " + err.Error())
		}
		grpclog.Info("Client: sending message " + m.Message)
	}
	err = stream.CloseSend()
	if err != nil {
		return
	}
	<-waitc
}
