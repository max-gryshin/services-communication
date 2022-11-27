package service

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"io"
	serviceGrpc "servicesCommunication/grpc"
	"servicesCommunication/internal/grpclog"
	"servicesCommunication/internal/server"
	"servicesCommunication/internal/utils"
	"strconv"
	"sync"
	"time"
)

type Node struct {
	serviceID     string
	portsToLookUp []int
	opts          []grpc.DialOption
	frequency     time.Duration
	timeout       time.Duration
	server        *server.Server
}

func NewNode(id string, portsToLookUp []int, frequency time.Duration, server *server.Server) *Node {
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
				port := strconv.Itoa(p)
				nf.server.Mutex.Lock()
				if status, ok := nf.server.StatusMap[port]; ok {
					if status == serviceGrpc.HealthCheckResponse_SERVING_CONNECTED ||
						status == serviceGrpc.HealthCheckResponse_SERVING_NOT_CONNECTED {
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
						fmt.Println(err.Error())
					} else {
						client := serviceGrpc.NewServiceCommunicatorClient(conn)
						resp, err := client.HealthCheck(
							context.Background(),
							&serviceGrpc.HealthCheckRequest{})
						if resp == nil {
							return
						}
						if err != nil {
							grpcConnectionErrMessage := "can't connect grpc server: " + port + ", code: " + err.Error()
							grpclog.Info(grpcConnectionErrMessage)
							fmt.Println(grpcConnectionErrMessage)
						}
						nf.server.Mutex.Lock()
						nf.server.StatusMap[port] = serviceGrpc.HealthCheckResponse_SERVING_NOT_CONNECTED
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
				}(&wg, port)
			}
			wg.Wait()
		}
	}()
	ch <- struct{}{}

	return &nf
}

func (n *Node) LookUp() {
	//fmt.Println(n.server.StatusMap)
	for port, neighborSync := range n.server.StatusMap {
		if neighborSync == serviceGrpc.HealthCheckResponse_SERVING_CONNECTED || n.serviceID == port {
			continue
		}
		p := port
		go func(port string) {
			//fmt.Println("New Connection with: " + port)
			conn, err := grpc.Dial(utils.GetServiceName(port), n.opts...)
			if err != nil {
				fmt.Println("fail to dial: " + err.Error())
				grpclog.Error("fail to dial: " + err.Error())
			}
			client := serviceGrpc.NewServiceCommunicatorClient(conn)
			if n.connected(client) {
				return
			}
			n.server.Mutex.Lock()
			n.server.StatusMap[port] = serviceGrpc.HealthCheckResponse_SERVING_CONNECTED
			n.communicate(client, port)
			n.disconnected(client)
			n.server.StatusMap[port] = serviceGrpc.HealthCheckResponse_SERVING_NOT_CONNECTED
			//n.server.Mutex.Unlock()
			defer func(conn *grpc.ClientConn) {
				err := conn.Close()
				if err != nil {
					grpclog.Error(err)
				}
				//n.server.Mutex.Lock()
				fmt.Println("End connection with " + port)
				n.server.StatusMap[port] = serviceGrpc.HealthCheckResponse_NOT_SERVING
				n.server.Mutex.Unlock()
			}(conn)
		}(p)
	}
}

func (n *Node) communicate(client serviceGrpc.ServiceCommunicatorClient, serviceID string) {
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
			if myMessage.Message != "" {
				incM := "Got message " + myMessage.Message + " from " + myMessage.ServiceName + " service"
				grpclog.Info(incM)
				fmt.Println(incM)
			}
		}
	}()
	for _, message := range utils.GetRandStrings() {
		m := &serviceGrpc.Message{
			ServiceName: n.serviceID,
			Message:     message,
		}
		if err = stream.Send(m); err != nil {
			grpclog.Error("SendMessage failed " + err.Error())
		}
		if m.Message != "" {
			sendM := "Client: sending message " + m.Message + " to " + serviceID
			grpclog.Info(sendM)
			fmt.Println(sendM)
		}
	}
	err = stream.CloseSend()
	if err != nil {
		return
	}
	<-waitc
}

func (n *Node) connected(client serviceGrpc.ServiceCommunicatorClient) bool {
	resp, err := client.Connected(
		context.Background(),
		&serviceGrpc.HealthCheckRequest{Service: n.serviceID})
	if resp == nil {
		grpclog.Info("problems to is say it connected")
		fmt.Println("problems to is say it connected")
		return false
	}
	if err != nil {
		fmt.Println("problems with marking a connect:" + err.Error())
	}

	return resp.Ok
}

func (n *Node) disconnected(client serviceGrpc.ServiceCommunicatorClient) {
	resp, err := client.Disconnected(
		context.Background(),
		&serviceGrpc.HealthCheckRequest{Service: n.serviceID})
	if resp == nil || resp.Ok == false {
		grpclog.Info("problems to is say it connected")
		fmt.Println("problems to is say it connected")
		return
	}
	if err != nil {
		fmt.Println("problems with marking a disconnect:" + err.Error())
	}
}
