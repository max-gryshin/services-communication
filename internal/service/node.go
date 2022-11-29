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
						grpclog.Error(err.Error())
					} else {
						client := serviceGrpc.NewServiceCommunicatorClient(conn)
						resp, err := client.HealthCheck(
							context.Background(),
							&serviceGrpc.HealthCheckRequest{})
						if resp == nil {
							return
						}
						if err != nil {
							grpclog.Error(fmt.Sprintf("Can not connect grpc server: %s, code: %s", port, err.Error()))
						}
						nf.server.Mutex.Lock()
						nf.server.StatusMap[port] = serviceGrpc.HealthCheckResponse_SERVING_NOT_CONNECTED
						nf.server.Mutex.Unlock()
					}
					defer func() {
						waitGroup.Done()
						if conn != nil {
							err = conn.Close()
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
	return &nf
}

func (n *Node) LookUp() {
	for port, neighborSync := range n.server.StatusMap {
		if neighborSync == serviceGrpc.HealthCheckResponse_SERVING_CONNECTED || n.serviceID == port {
			continue
		}
		p := port
		go func(port string) {
			conn, err := grpc.Dial(utils.GetServiceName(port), n.opts...)
			if err != nil {
				grpclog.Error(err)
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
			defer func(conn *grpc.ClientConn) {
				err = conn.Close()
				if err != nil {
					grpclog.Error(err)
				}
				grpclog.Info(fmt.Sprintf("End connection with %s", port))
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
		grpclog.Error(err)
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
				grpclog.Error(errRecv)
			}
			if myMessage == nil {
				continue
			}
			if myMessage.Message != "" {
				grpclog.Info(fmt.Sprintf("Got message  %s from %s", myMessage.Message, myMessage.ServiceName))
			}
		}
	}()
	for _, message := range utils.GetRandStrings() {
		m := &serviceGrpc.Message{
			ServiceName: n.serviceID,
			Message:     message,
		}
		if err = stream.Send(m); err != nil {
			grpclog.Error(fmt.Sprintf("SendMessage failed %s", err.Error()))
		}
		if m.Message != "" {
			grpclog.Info(fmt.Sprintf("Client: sending message %s to %s", m.Message, serviceID))
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
		grpclog.Error("Can not notify about connect")
		return false
	}
	if err != nil {
		grpclog.Error(fmt.Sprintf("Marking a connect: %s", err.Error()))
	}

	return resp.Ok
}

func (n *Node) disconnected(client serviceGrpc.ServiceCommunicatorClient) {
	resp, err := client.Disconnected(
		context.Background(),
		&serviceGrpc.HealthCheckRequest{Service: n.serviceID})
	if resp == nil || resp.Ok == false {
		grpclog.Error("Can not notify about disconnect")
		return
	}
	if err != nil {
		grpclog.Error(fmt.Sprintf("Marking a disconnect: %s", err.Error()))
	}
}
