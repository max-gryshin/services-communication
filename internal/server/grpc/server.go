package grpc

import (
	"context"
	"net"
	"runtime"
	"time"

	grpcrecovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	serviceGrpc "github.com/max-gryshin/services-communication/api"
	"github.com/max-gryshin/services-communication/internal/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type GRPCServer struct {
	ID         string
	Period     time.Duration
	grpcServer *grpc.Server
	cancelFunc context.CancelFunc
	serviceGrpc.UnimplementedNodeServer
}

func New(
	id string,
	period time.Duration,
) *GRPCServer {
	unaryInterceptorsChain := []grpc.StreamServerInterceptor{
		grpcrecovery.StreamServerInterceptor(),
	}

	return &GRPCServer{
		ID:     id,
		Period: period,
		grpcServer: grpc.NewServer(
			grpc.ChainStreamInterceptor(unaryInterceptorsChain...),
			grpc.MaxRecvMsgSize(50<<20),                           // 50Mb
			grpc.MaxConcurrentStreams(uint32(runtime.NumCPU()/2)), // half of num cpu
		),
	}
}

func (s *GRPCServer) Serve(port string) {
	log.Info("New GRPCServer up: " + s.ID)
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatal("failed to listen: " + err.Error())
	}
	serviceGrpc.RegisterNodeServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)
	if err = s.grpcServer.Serve(lis); err != nil {
		log.Fatal("failed to serve: " + err.Error())
	}
}
