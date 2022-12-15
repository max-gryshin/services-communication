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
	"sync"
	"time"
)

type Node struct {
	serviceID string
	Nodes     []string
	opts      []grpc.DialOption
	frequency time.Duration
	timeout   time.Duration
	server    *server.Server
}

func NewNode(id string, Nodes []string, frequency time.Duration, server *server.Server) *Node {
	nf := Node{
		serviceID: id,
		Nodes:     Nodes,
		frequency: frequency,
		timeout:   time.Second * 10,
		server:    server,
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
			for _, node := range nf.Nodes {
				if status, ok := nf.server.StatusMap[node]; ok {
					if status == serviceGrpc.HealthCheckResponse_SERVING_CONNECTED ||
						status == serviceGrpc.HealthCheckResponse_SERVING_NOT_CONNECTED {
						continue
					}
				}
				wg.Add(1)
				count++
				go func(waitGroup *sync.WaitGroup, serviceName string) {
					conn, err := grpc.Dial(serviceName, nf.opts...)
					if err != nil {
						grpclog.Error(err.Error())
					} else {
						client := serviceGrpc.NewServiceCommunicatorClient(conn)
						resp, err := client.HealthCheck(
							context.Background(),
							&serviceGrpc.HealthCheckRequest{})

						if err != nil {
							grpclog.Error(fmt.Sprintf("Can not connect grpc server: %s, code: %s", serviceName, err.Error()))
							return
						}
						if resp == nil {
							grpclog.Error("grpc server response is nil")
							return
						}
						nf.server.Mutex.Lock()
						nf.server.StatusMap[serviceName] = serviceGrpc.HealthCheckResponse_SERVING_NOT_CONNECTED
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
				}(&wg, node)
			}
			wg.Wait()
		}
	}()
	return &nf
}

func (n *Node) LookUp() {
	// wait group will wait until we finish look up process
	wg := sync.WaitGroup{}
	for serviceName, neighborSync := range n.server.StatusMap {
		if neighborSync == serviceGrpc.HealthCheckResponse_SERVING_CONNECTED {
			continue
		}
		wg.Add(1)
		go func(wg *sync.WaitGroup, serviceName string) {
			conn, err := grpc.Dial(serviceName, n.opts...)
			if err != nil {
				grpclog.Error(err)
				return
			}
			client := serviceGrpc.NewServiceCommunicatorClient(conn)
			if n.connected(client) {
				grpclog.Info(fmt.Sprintf("Already connected with %s", serviceName))
				return
			}
			n.server.Mutex.Lock()
			n.server.StatusMap[serviceName] = serviceGrpc.HealthCheckResponse_SERVING_CONNECTED
			n.communicate(client, serviceName)
			n.disconnected(client)
			n.server.StatusMap[serviceName] = serviceGrpc.HealthCheckResponse_SERVING_NOT_CONNECTED
			defer func(conn *grpc.ClientConn) {
				wg.Done()
				err = conn.Close()
				if err != nil {
					grpclog.Error(err)
				}
				grpclog.Info(fmt.Sprintf("End connection with %s", serviceName))
				n.server.Mutex.Unlock()
			}(conn)
		}(&wg, serviceName)
		wg.Wait()
	}
}

func (n *Node) communicateByStream(client serviceGrpc.ServiceCommunicatorClient, serviceID string) {
	ctx, cancel := context.WithTimeout(context.Background(), n.timeout)
	defer cancel()
	stream, err := client.SendRandStringStream(ctx)
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
				grpclog.Info(fmt.Sprintf("Client: receive %s from %s", myMessage.Message, myMessage.ServiceName))
			}
		}
	}()
	for _, message := range utils.GetRandStrings(2, 15) {
		m := &serviceGrpc.Message{
			ServiceName: n.serviceID,
			Message:     message,
		}
		if err = stream.Send(m); err != nil {
			grpclog.Error(fmt.Sprintf("Client: failed %s", err.Error()))
		}
		if m.Message != "" {
			grpclog.Info(fmt.Sprintf("Client: sent %s to %s", m.Message, serviceID))
		}
	}
	err = stream.CloseSend()
	if err != nil {
		return
	}
	<-waitc
}

func (n *Node) communicate(client serviceGrpc.ServiceCommunicatorClient, serviceID string) {
	ctx, cancel := context.WithTimeout(context.Background(), n.timeout)
	defer cancel()
	m := &serviceGrpc.Message{
		ServiceName: n.serviceID,
		Message:     utils.RandStringBytesMask(15),
	}
	grpclog.Info(fmt.Sprintf("Client: sent %s to %s", m.Message, serviceID))
	responseMessage, err := client.SendRandString(ctx, m)
	if err != nil {
		grpclog.Error(err)
	}
	grpclog.Info(fmt.Sprintf("Client: receive %s from %s", responseMessage.Message, responseMessage.ServiceName))
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
