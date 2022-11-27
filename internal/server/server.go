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
	port                     string
	frequencyOfCommunication time.Duration
	Mutex                    sync.Mutex
	StatusMap                map[string]serviceGrpc.HealthCheckResponse_Status
	serviceGrpc.UnimplementedServiceCommunicatorServer
}

func NewServer(port string, freq time.Duration) *Server {
	statusMap := make(map[string]serviceGrpc.HealthCheckResponse_Status)
	statusMap[port] = serviceGrpc.HealthCheckResponse_NOT_SERVING

	s := Server{
		port:                     port,
		frequencyOfCommunication: freq,
		StatusMap:                statusMap,
		Mutex:                    sync.Mutex{},
	}

	return &s
}

func (s *Server) Serve(port string) {
	fmt.Printf("New Server up: %s \n", port)
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		grpclog.Fatal("failed to listen: %s" + err.Error())
	}
	grpcServer := grpc.NewServer()
	serviceGrpc.RegisterServiceCommunicatorServer(grpcServer, s)
	reflection.Register(grpcServer)
	grpclog.Info("Server listening at" + lis.Addr().String())
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
			fmt.Println("it is already connected")
			return &serviceGrpc.ServeResponse{Ok: true}, nil
		}
		s.StatusMap[in.Service] = serviceGrpc.HealthCheckResponse_SERVING_CONNECTED
		fmt.Println("we said that " + in.Service + " is connected")
		fmt.Println(s.StatusMap)
		return &serviceGrpc.ServeResponse{Ok: false}, nil
	}
	fmt.Println("Whoops!")
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
		if incomingMessage == nil {
			break
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if incomingMessage.Message != "" {
			incM := "Incoming message " + incomingMessage.Message + " from " + incomingMessage.ServiceName + " service"
			grpclog.Info(incM)
			fmt.Println(incM)
		}
		for _, randStr := range utils.GetRandStrings() {
			if err = stream.Send(&serviceGrpc.Message{
				ServiceName: s.port,
				Message:     randStr,
			}); err != nil {
				return err
			}
			if randStr != "" {
				outM := "Outgoing message " + randStr + ", from " + s.port + " service to " + incomingMessage.ServiceName
				grpclog.Info(outM)
				fmt.Println(outM)
			}
		}
	}

	return nil
}
