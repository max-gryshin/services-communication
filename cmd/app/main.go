package main

import (
	"context"
	"log"
	"syscall"

	"github.com/max-gryshin/services-communication/internal/closer"
	"github.com/max-gryshin/services-communication/internal/config"
	grpcLog "github.com/max-gryshin/services-communication/internal/log"
	"github.com/max-gryshin/services-communication/internal/server/grpc"
	"github.com/max-gryshin/services-communication/internal/service/node"
)

func main() {
	configs := config.New()

	grpcLog.Setup(&configs.App)
	defer func() {
		err := grpcLog.Close()
		if err != nil {
			log.Print(err)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	incomeConnectionsCloser := closer.New(syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL)
	outcomeConnectionsCloser := closer.New()

	resources := &grpc.Resources{
		Node: node.New(ctx, configs),
	}
	grpcServer := grpc.New(
		configs,
		resources,
	)
	grpcServer.Run()

	incomeConnectionsCloser.Add(grpcServer.Stop)
	incomeConnectionsCloser.Wait()
	outcomeConnectionsCloser.CloseAll()
}
