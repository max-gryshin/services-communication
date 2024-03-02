package node

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"golang.org/x/exp/maps"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	grpcApi "github.com/max-gryshin/services-communication/api"
	"github.com/max-gryshin/services-communication/internal/config"
	"github.com/max-gryshin/services-communication/internal/log"
	"github.com/max-gryshin/services-communication/internal/utils"
)

type Node struct {
	ID           string
	Nodes        []string
	Mutex        sync.RWMutex
	NodeStatuses map[string]grpcApi.HealthCheckResponse_Status
	opts         []grpc.DialOption
	grpcApi.UnimplementedNodeServer
}

func New(
	ctx context.Context,
	config *config.Config,
) *Node {
	nStatuses := make(map[string]grpcApi.HealthCheckResponse_Status)
	for _, node := range config.Nodes {
		nStatuses[node] = grpcApi.HealthCheckResponse_NOT_SERVING
	}
	n := &Node{
		ID:           config.App.Name,
		NodeStatuses: nStatuses,
		Mutex:        sync.RWMutex{},
	}
	n.opts = append(n.opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	go func(ctx context.Context) {
		wg := sync.WaitGroup{}
		ticker := time.NewTicker(config.ServerConfig.FrequencyCommunication)
	LOOP:
		for {
			select {
			// todo: have a deal with it
			case <-ctx.Done():
				ticker.Stop()
				log.Info("SEARCH FOR NEW NODES TICKER STOPPED")
				break LOOP
			case <-ticker.C:
				for _, node := range maps.Keys(n.NodeStatuses) {
					if status, ok := n.NodeStatuses[node]; ok {
						if status == grpcApi.HealthCheckResponse_SERVING_CONNECTED ||
							status == grpcApi.HealthCheckResponse_SERVING_NOT_CONNECTED {
							continue
						}
					}
					wg.Add(1)
					go func(ctx context.Context, waitGroup *sync.WaitGroup, serviceName string) {
						conn, err := grpc.DialContext(ctx, serviceName, n.opts...)
						defer func() {
							waitGroup.Done()
							if conn != nil {
								err = conn.Close()
								if err != nil {
									log.Error(err)
								}
							}
						}()
						if err != nil {
							log.Error(err.Error())
						}
						client := grpcApi.NewNodeClient(conn)
						resp, err := client.HealthCheck(ctx, &grpcApi.HealthCheckRequest{})
						if err != nil {
							log.Error(
								fmt.Sprintf("Can not connect api server: %s, code: %s", serviceName, err.Error()),
							)
							return
						}
						if resp == nil {
							log.Error("api server response is nil")
							return
						}
						n.Mutex.Lock()
						n.NodeStatuses[serviceName] = grpcApi.HealthCheckResponse_SERVING_NOT_CONNECTED
						n.Mutex.Unlock()
					}(ctx, &wg, node)
				}
				wg.Wait()
			}
		}
	}(ctx)
	go func(ctx context.Context) {
		ticker := time.NewTicker(config.ServerConfig.FrequencyCommunication)
	LOOP:
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				break LOOP
			case <-ticker.C:
				n.LookUp(ctx)
			}
		}
	}(ctx)

	return n
}

func (n *Node) HealthCheck(_ context.Context, in *grpcApi.HealthCheckRequest) (*grpcApi.HealthCheckResponse, error) {
	n.Mutex.RLock()
	defer n.Mutex.RUnlock()
	if servingStatus, ok := n.NodeStatuses[in.Node]; ok {
		return &grpcApi.HealthCheckResponse{
			Status: servingStatus,
		}, nil
	}
	return nil, status.Error(codes.NotFound, "unknown service")
}

func (n *Node) Connected(_ context.Context, in *grpcApi.HealthCheckRequest) (*grpcApi.ServeResponse, error) {
	n.Mutex.Lock()
	defer n.Mutex.Unlock()
	if serviceStatus, ok := n.NodeStatuses[in.Node]; ok {
		if serviceStatus == grpcApi.HealthCheckResponse_SERVING_CONNECTED {
			return &grpcApi.ServeResponse{Ok: true}, nil
		}
		n.NodeStatuses[in.Node] = grpcApi.HealthCheckResponse_SERVING_CONNECTED
		return &grpcApi.ServeResponse{Ok: false}, nil
	}
	return nil, status.Error(codes.NotFound, "unknown service")
}

func (n *Node) Disconnected(
	_ context.Context,
	in *grpcApi.HealthCheckRequest,
) (*grpcApi.ServeResponse, error) {
	n.Mutex.Lock()
	defer n.Mutex.Unlock()
	if _, ok := n.NodeStatuses[in.Node]; ok {
		n.NodeStatuses[in.Node] = grpcApi.HealthCheckResponse_SERVING_NOT_CONNECTED
		return &grpcApi.ServeResponse{Ok: true}, nil
	}
	return nil, status.Error(codes.NotFound, "unknown service")
}

func (n *Node) SendStream(stream grpcApi.Node_SendStreamServer) error {
	for {
		incomingMessage, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if incomingMessage.Message != "" {
			log.Info(
				fmt.Sprintf("GRPCServer: receive %s from %s ", incomingMessage.Message, incomingMessage.Name),
			)
			for _, randStr := range utils.GetRandStrings(2, 10) {
				if randStr == "" {
					continue
				}
				newM := incomingMessage.Message + "-" + randStr
				if err = stream.Send(&grpcApi.Message{
					Name:    n.ID,
					Message: newM,
				}); err != nil {
					return err
				}
				log.Info(
					fmt.Sprintf("GRPCServer: sent %s, from %s service to %s", newM, n.ID, incomingMessage.Name),
				)
			}
		}
	}

	return nil
}

func (n *Node) Send(_ context.Context, in *grpcApi.Message) (*grpcApi.Message, error) {
	log.Info(fmt.Sprintf("GRPCServer: receive %s from %s ", in.Message, in.Name))
	newM := in.Message + "-" + utils.RandStringBytesMask(10)
	log.Info(fmt.Sprintf("GRPCServer: sent %s, from %s service to %s", newM, n.ID, in.Name))
	return &grpcApi.Message{
		Name:    n.ID,
		Message: newM,
	}, nil
}

func (n *Node) LookUp(ctx context.Context) {
	// wait group will wait until we finish look up process
	wg := sync.WaitGroup{}
	for serviceName, neighborSync := range n.NodeStatuses {
		if neighborSync == grpcApi.HealthCheckResponse_SERVING_CONNECTED {
			continue
		}
		wg.Add(1)
		go func(ctx context.Context, wg *sync.WaitGroup, serviceName string) {
			conn, err := grpc.DialContext(ctx, serviceName, n.opts...)
			if err != nil {
				log.Error(err)
				return
			}
			client := grpcApi.NewNodeClient(conn)
			if n.connected(ctx, client) {
				log.Info(fmt.Sprintf("Already connected with %s", serviceName))
				return
			}
			n.Mutex.Lock()
			n.NodeStatuses[serviceName] = grpcApi.HealthCheckResponse_SERVING_CONNECTED
			n.communicate(ctx, client, serviceName)
			n.disconnected(ctx, client)
			n.NodeStatuses[serviceName] = grpcApi.HealthCheckResponse_SERVING_NOT_CONNECTED
			defer func(conn *grpc.ClientConn) {
				wg.Done()
				err = conn.Close()
				if err != nil {
					log.Error(err)
				}
				log.Info(fmt.Sprintf("End connection with %s", serviceName))
				n.Mutex.Unlock()
			}(conn)
		}(ctx, &wg, serviceName)
		wg.Wait()
	}
}

func (n *Node) communicateByStream(
	ctx context.Context,
	client grpcApi.NodeClient,
	serviceID string,
) {
	stream, err := client.SendStream(ctx)
	if err != nil {
		log.Error(err)
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
				log.Error(errRecv)
			}
			if myMessage == nil {
				continue
			}
			if myMessage.Message != "" {
				log.Info(fmt.Sprintf("Client: receive %s from %s", myMessage.Message, myMessage.Name))
			}
		}
	}()
	for _, message := range utils.GetRandStrings(2, 15) {
		m := &grpcApi.Message{
			Name:    n.ID,
			Message: message,
		}
		if err = stream.Send(m); err != nil {
			log.Error(fmt.Sprintf("Client: failed %s", err.Error()))
		}
		if m.Message != "" {
			log.Info(fmt.Sprintf("Client: sent %s to %s", m.Message, serviceID))
		}
	}
	err = stream.CloseSend()
	if err != nil {
		return
	}
	<-waitc
}

func (n *Node) communicate(
	ctx context.Context,
	client grpcApi.NodeClient,
	serviceID string,
) {
	m := &grpcApi.Message{
		Name:    n.ID,
		Message: utils.RandStringBytesMask(15),
	}
	responseMessage, err := client.Send(ctx, m)
	if responseMessage == nil {
		log.Error(fmt.Sprintf("resp from %s is nil", serviceID))
	}
	if err != nil {
		log.Error(err)
	}
	log.Info(fmt.Sprintf("Client: sent %s to %s", m.Message, serviceID))
	log.Info(fmt.Sprintf("Client: receive %s from %s", responseMessage.Message, responseMessage.Name))
}

func (n *Node) connected(ctx context.Context, client grpcApi.NodeClient) bool {
	resp, err := client.Connected(ctx, &grpcApi.HealthCheckRequest{Node: n.ID})
	if resp == nil {
		log.Error("Can not notify about connect")
		return false
	}
	if err != nil {
		log.Error(fmt.Sprintf("Marking a connect: %s", err.Error()))
	}

	return resp.Ok
}

func (n *Node) disconnected(ctx context.Context, client grpcApi.NodeClient) {
	resp, err := client.Disconnected(ctx, &grpcApi.HealthCheckRequest{Node: n.ID})
	if resp == nil || resp.Ok == false {
		log.Error("Can not notify about disconnect")
		return
	}
	if err != nil {
		log.Error(fmt.Sprintf("Marking a disconnect: %s", err.Error()))
	}
}
