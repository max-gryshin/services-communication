package service

import (
	"context"
	mygrpc "fiveServices/grpc"
	"fiveServices/internal/grpclog"
	"fiveServices/internal/server"
	"fiveServices/internal/utils"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"io"
	"strconv"
	"sync"
	"time"
)

type Node struct {
	serviceID     int
	portsToLookUp []int
	opts          []grpc.DialOption
	frequency     time.Duration
	timeout       time.Duration
	server        *server.Server
}

func NewNode(id int, portsToLookUp []int, frequency time.Duration, server *server.Server) *Node {
	nf := Node{
		serviceID:     id,
		portsToLookUp: portsToLookUp,
		frequency:     frequency,
		timeout:       time.Second * 10,
		server:        server,
	}
	nf.opts = append(
		nf.opts,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	go func() {
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
				nf.server.Mutex.Lock()
				if status, ok := nf.server.StatusMap[p]; ok {
					if status == mygrpc.HealthCheckResponse_SERVING {
						nf.server.Mutex.Unlock()
						continue
					}
				}
				nf.server.Mutex.Unlock()
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
						nf.server.Mutex.Lock()
						nf.server.StatusMap[port] = mygrpc.HealthCheckResponse_NOT_SERVING
						nf.server.Mutex.Unlock()
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

	return &nf
}

func (n *Node) LookUp() {
	for port, neighborSync := range n.server.StatusMap {
		if neighborSync == mygrpc.HealthCheckResponse_SERVING || strconv.Itoa(n.serviceID) == port {
			continue
		}
		p := port
		go func(port string) {
			conn, err := grpc.Dial(utils.GetServiceName(port), n.opts...)
			if err != nil {
				grpclog.Error("fail to dial: " + err.Error())
			}
			n.server.Mutex.Lock()
			client := mygrpc.NewMyServiceClient(conn)
			clientHealth := mygrpc.NewHealthClient(conn)
			resp, err := clientHealth.Connected(
				context.Background(),
				&mygrpc.HealthCheckRequest{Service: strconv.Itoa(n.serviceID)})
			if resp == nil || resp.Ok == false {
				grpclog.Info("problems to is say it connected")
				return
			}
			n.server.StatusMap[port] = mygrpc.HealthCheckResponse_SERVING
			n.communicate(client)
			n.server.StatusMap[port] = mygrpc.HealthCheckResponse_NOT_SERVING
			n.server.Mutex.Unlock()
			defer func(conn *grpc.ClientConn) {
				err := conn.Close()
				if err != nil {
					grpclog.Error(err)
				}
				n.server.Mutex.Lock()
				delete(n.server.StatusMap, port)
				n.server.Mutex.Unlock()
			}(conn)
		}(p)
	}
}

func (n *Node) communicate(client mygrpc.MyServiceClient) {
	ctx, cancel := context.WithTimeout(context.Background(), n.timeout)
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
			incM := "Got message " + myMessage.Message + " from " + strconv.Itoa(int(myMessage.ServiceID)) + " service"
			grpclog.Info(incM)
			fmt.Println(incM)
		}
	}()
	for _, message := range utils.GetRandStrings() {
		m := &mygrpc.MyMessage{
			ServiceID: int32(n.serviceID),
			Message:   message,
		}
		if err = stream.Send(m); err != nil {
			grpclog.Error("SendMessage failed " + err.Error())
		}
		sendM := "Client: sending message " + m.Message
		grpclog.Info(sendM)
		fmt.Println(sendM)
	}
	err = stream.CloseSend()
	if err != nil {
		return
	}
	<-waitc
}
