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
	id                       string
	frequencyOfCommunication time.Duration
	Mutex                    sync.Mutex
	StatusMap                map[string]serviceGrpc.HealthCheckResponse_Status
	serviceGrpc.UnimplementedServiceCommunicatorServer
}

func NewServer(id string, freq time.Duration) *Server {
	statusMap := make(map[string]serviceGrpc.HealthCheckResponse_Status)
	statusMap[id] = serviceGrpc.HealthCheckResponse_NOT_SERVING

	s := Server{
		id:                       id,
		frequencyOfCommunication: freq,
		StatusMap:                statusMap,
		Mutex:                    sync.Mutex{},
	}

	return &s
}

func (s *Server) Serve(port string) {
	grpclog.Info("New Server up: " + s.id)
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		grpclog.Fatal("failed to listen: %s" + err.Error())
	}
	grpcServer := grpc.NewServer()
	serviceGrpc.RegisterServiceCommunicatorServer(grpcServer, s)
	reflection.Register(grpcServer)
	if err = grpcServer.Serve(lis); err != nil {
		grpclog.Fatal("failed to serve: %s" + err.Error())
	}
}

func (s *Server) HealthCheck(ctx context.Context, in *serviceGrpc.HealthCheckRequest) (*serviceGrpc.HealthCheckResponse, error) {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	if in.Service == "" {
		// check the Server overall health status.
		return &serviceGrpc.HealthCheckResponse{
			Status: serviceGrpc.HealthCheckResponse_SERVING_NOT_CONNECTED,
		}, nil
	}
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

func (s *Server) SendRandString(stream serviceGrpc.ServiceCommunicator_SendRandStringServer) error {
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
