package grpc

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	grpcrecovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	serviceGrpc "github.com/max-gryshin/services-communication/api"
	"github.com/max-gryshin/services-communication/internal/config"
	"github.com/max-gryshin/services-communication/internal/log"
	"github.com/max-gryshin/services-communication/internal/service/node"
)

type Resources struct {
	Node *node.Node
}

type GRPCServer struct {
	ID                        string
	grpcServer                *grpc.Server
	listener                  net.Listener
	gracefullyShutdownTimeout time.Duration
	cancelFunc                context.CancelFunc
}

func New(
	cfg *config.Config,
	resources *Resources,
) *GRPCServer {
	log.Info("New GRPCServer up: " + cfg.ServerConfig.Port)
	listener, err := net.Listen("tcp", ":"+cfg.ServerConfig.Port)
	if err != nil {
		log.Fatal("failed to listen: " + err.Error())
	}

	interceptorsChain := []grpc.StreamServerInterceptor{
		grpcrecovery.StreamServerInterceptor(),
	}
	s := &GRPCServer{
		ID: cfg.App.Name,
		grpcServer: grpc.NewServer(
			grpc.ChainStreamInterceptor(interceptorsChain...),
			grpc.MaxRecvMsgSize(50<<20),                           // 50Mb
			grpc.MaxConcurrentStreams(uint32(runtime.NumCPU()/2)), // half of num cpu
		),
		listener:                  listener,
		gracefullyShutdownTimeout: time.Duration(cfg.GracefullyShutdownTimeoutMs) * time.Millisecond,
	}
	// another nodes need a time to start
	time.Sleep(time.Second * time.Duration(cfg.ServerConfig.NodeCountByDefault))
	serviceGrpc.RegisterNodeServer(s.grpcServer, resources.Node)
	if cfg.Environment == config.DevEnvironment {
		reflection.Register(s.grpcServer)
	}
	return s
}

func (s *GRPCServer) Run() {
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.Serve()
	}()

	wg.Wait()
}

func (s *GRPCServer) Serve() {
	if err := s.grpcServer.Serve(s.listener); err != nil {
		log.Fatal("failed to serve: " + err.Error())
	}
}

func (s *GRPCServer) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.gracefullyShutdownTimeout)
	defer cancel()

	done := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		log.Warn("grpc: gracefully stopped")
	case <-ctx.Done():
		s.grpcServer.Stop()
		return fmt.Errorf("grpc: error during shutdown server: %w", ctx.Err())
	}

	return nil
}
