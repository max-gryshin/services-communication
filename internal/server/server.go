package server

import (
	"context"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"io"
	"net"
	serviceGrpc "servicesCommunication/grpc"
	"servicesCommunication/internal/grpclog"
	"servicesCommunication/internal/utils"
	"sync"
	"time"
)

type Server struct {
	id         string
	period     time.Duration
	Mutex      sync.RWMutex
	StatusMap  map[string]serviceGrpc.HealthCheckResponse_Status
	grpcServer *grpc.Server
	cancelFunc context.CancelFunc
	serviceGrpc.UnimplementedServiceCommunicatorServer
}

func NewServer(id string, nodes []string, period time.Duration) *Server {
	statusMap := make(map[string]serviceGrpc.HealthCheckResponse_Status)
	for _, node := range nodes {
		statusMap[node] = serviceGrpc.HealthCheckResponse_NOT_SERVING
	}
	s := Server{
		id:         id,
		period:     period,
		StatusMap:  statusMap,
		Mutex:      sync.RWMutex{},
		grpcServer: grpc.NewServer(),
	}

	return &s
}

func (s *Server) Serve(port string) {
	grpclog.Info("New Server up: " + s.id)
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		grpclog.Fatal("failed to listen: " + err.Error())
	}
	serviceGrpc.RegisterServiceCommunicatorServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)
	if err = s.grpcServer.Serve(lis); err != nil {
		grpclog.Fatal("failed to serve: " + err.Error())
	}
}

func (s *Server) HealthCheck(ctx context.Context, in *serviceGrpc.HealthCheckRequest) (*serviceGrpc.HealthCheckResponse, error) {
	s.Mutex.RLock()
	defer s.Mutex.RUnlock()
	if servingStatus, ok := s.StatusMap[in.Service]; ok {
		return &serviceGrpc.HealthCheckResponse{
			Status: servingStatus,
		}, nil
	}
	return nil, status.Error(codes.NotFound, "unknown service")
}

func (s *Server) Connected(ctx context.Context, in *serviceGrpc.HealthCheckRequest) (*serviceGrpc.ServeResponse, error) {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	if serviceStatus, ok := s.StatusMap[in.Service]; ok {
		if serviceStatus == serviceGrpc.HealthCheckResponse_SERVING_CONNECTED {
			return &serviceGrpc.ServeResponse{Ok: true}, nil
		}
		s.StatusMap[in.Service] = serviceGrpc.HealthCheckResponse_SERVING_CONNECTED
		return &serviceGrpc.ServeResponse{Ok: false}, nil
	}
	return nil, status.Error(codes.NotFound, "unknown service")
}

func (s *Server) Disconnected(ctx context.Context, in *serviceGrpc.HealthCheckRequest) (*serviceGrpc.ServeResponse, error) {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	if _, ok := s.StatusMap[in.Service]; ok {
		s.StatusMap[in.Service] = serviceGrpc.HealthCheckResponse_SERVING_NOT_CONNECTED
		return &serviceGrpc.ServeResponse{Ok: true}, nil
	}
	return nil, status.Error(codes.NotFound, "unknown service")
}

func (s *Server) SendRandStringStream(stream serviceGrpc.ServiceCommunicator_SendRandStringStreamServer) error {
	for {
		incomingMessage, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if incomingMessage.Message != "" {
			grpclog.Info(fmt.Sprintf("Server: receive %s from %s ", incomingMessage.Message, incomingMessage.ServiceName))
			for _, randStr := range utils.GetRandStrings(2, 10) {
				if randStr == "" {
					continue
				}
				newM := incomingMessage.Message + "-" + randStr
				if err = stream.Send(&serviceGrpc.Message{
					ServiceName: s.id,
					Message:     newM,
				}); err != nil {
					return err
				}
				grpclog.Info(fmt.Sprintf("Server: sent %s, from %s service to %s", newM, s.id, incomingMessage.ServiceName))
			}
		}
	}

	return nil
}

func (s *Server) SendRandString(ctx context.Context, in *serviceGrpc.Message) (*serviceGrpc.Message, error) {
	grpclog.Info(fmt.Sprintf("Server: receive %s from %s ", in.Message, in.ServiceName))
	newM := in.Message + "-" + utils.RandStringBytesMask(10)
	grpclog.Info(fmt.Sprintf("Server: sent %s, from %s service to %s", newM, s.id, in.ServiceName))
	return &serviceGrpc.Message{
		ServiceName: s.id,
		Message:     newM,
	}, nil
}
