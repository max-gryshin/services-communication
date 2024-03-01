package main

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/max-gryshin/services-communication/internal/config"
	grpcLog "github.com/max-gryshin/services-communication/internal/log"
	"github.com/max-gryshin/services-communication/internal/server/grpc"
	"github.com/max-gryshin/services-communication/internal/service/node"
)

func main() {
	settings := config.New()
	grpcLog.Setup(&settings.App)
	defer func() {
		err := grpcLog.Close()
		if err != nil {
			log.Print(err)
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := grpc.New(
		settings.App.Name,
		settings.ServerConfig.FrequencyCommunication,
	)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		srv.Serve(settings.ServerConfig.Port)
	}()
	// another nodes need a time to start
	time.Sleep(time.Second * time.Duration(settings.ServerConfig.NodeCountByDefault))
	n := node.New(
		ctx,
		settings.App.Name,
		settings.Nodes,
		settings.ServerConfig.FrequencyCommunication,
	)
	go func(ctx context.Context) {
		ticker := time.NewTicker(settings.ServerConfig.FrequencyCommunication)
	loop:
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				break loop
			case <-ticker.C:
				n.LookUp(ctx)
			}
		}
	}(ctx)
	wg.Wait()
}
