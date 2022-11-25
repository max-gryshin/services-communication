package server

import (
	"context"
	mygrpc "fiveServices/grpc"
	"fiveServices/internal/grpclog"
	"fiveServices/internal/utils"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"io"
	"net"
	"strconv"
	"sync"
	"time"
)

type Server struct {
	port                     int
	frequencyOfCommunication time.Duration
	mu                       sync.Mutex
	StatusMap                map[string]mygrpc.HealthCheckResponse_ServingStatus
	mygrpc.UnimplementedMyServiceServer
	mygrpc.UnimplementedHealthServer
}

func NewServer(port int, freq time.Duration) *Server {
	statusMap := make(map[string]mygrpc.HealthCheckResponse_ServingStatus)
	statusMap[strconv.Itoa(port)] = mygrpc.HealthCheckResponse_NOT_SERVING

	s := Server{
		port:                     port,
		frequencyOfCommunication: freq,
		StatusMap:                statusMap,
		mu:                       sync.Mutex{},
	}

	return &s
}

func (s *Server) Serve(port int) {
	fmt.Printf("New Server up: %d \n", port)
	lis, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		grpclog.Fatal("failed to listen: %s" + err.Error())
	}
	grpcServer := grpc.NewServer()

	mygrpc.RegisterHealthServer(grpcServer, s)
	mygrpc.RegisterMyServiceServer(grpcServer, s)
	reflection.Register(grpcServer)
	grpclog.Info("Server listening at" + lis.Addr().String())
	fmt.Printf("Server listening at %s \n", lis.Addr().String())
	if err = grpcServer.Serve(lis); err != nil {
		grpclog.Fatal("failed to serve: %s" + err.Error())
	}
}

func (s *Server) Check(ctx context.Context, in *mygrpc.HealthCheckRequest) (*mygrpc.HealthCheckResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if in.Service == "" {
		// check the Server overall health status.
		return &mygrpc.HealthCheckResponse{
			Status: mygrpc.HealthCheckResponse_SERVING,
		}, nil
	}
	if servingStatus, ok := s.StatusMap[in.Service]; ok {
		return &mygrpc.HealthCheckResponse{
			Status: servingStatus,
		}, nil
	}
	return nil, status.Error(codes.NotFound, "unknown service")
}

func (s *Server) Connected(ctx context.Context, in *mygrpc.HealthCheckRequest) (*mygrpc.ServeResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.StatusMap[in.Service]; ok {
		s.StatusMap[in.Service] = mygrpc.HealthCheckResponse_SERVING
		return &mygrpc.ServeResponse{Ok: true}, nil
	}
	return nil, status.Error(codes.NotFound, "unknown service")
}

func (s *Server) SendRandString(stream mygrpc.MyService_SendRandStringServer) error {
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
		grpclog.Info(
			"Incoming message " + incomingMessage.Message +
				"from " + strconv.Itoa(int(incomingMessage.ServiceID)) + "service",
		)
		m := utils.RandStringBytesMask(20)
		if err = stream.Send(&mygrpc.MyMessage{
			ServiceID: int32(s.port),
			Message:   m,
		}); err != nil {
			return err
		}
		grpclog.Info("Outgoing message " + m + ", from " + strconv.Itoa(s.port) + " service")
	}

	return nil
}
