package main

import (
	"context"
	grpcLog "github.com/max-gryshin/services-communication/internal/log"
	"github.com/max-gryshin/services-communication/internal/server"
	"github.com/max-gryshin/services-communication/internal/service"
	"github.com/max-gryshin/services-communication/internal/setting"
	"log"
	"sync"
	"time"
)

func main() {
	settings := setting.NewSetting()
	grpcLog.Setup(&settings.App)
	defer func() {
		err := grpcLog.Close()
		if err != nil {
			log.Print(err)
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := server.New(settings.App.ServiceName, settings.Nodes, settings.ServerConfig.FrequencyCommunication)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		srv.Serve(settings.ServerConfig.Port)
	}()
	// another servers need a time to start
	time.Sleep(time.Second * time.Duration(settings.ServerConfig.NodeCountByDefault))
	neighbors := service.NewNode(ctx, settings.App.ServiceName, settings.Nodes, settings.ServerConfig.FrequencyCommunication, srv)
	go func(ctx context.Context) {
		ticker := time.NewTicker(settings.ServerConfig.FrequencyCommunication)
	loop:
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				break loop
			case <-ticker.C:
				neighbors.LookUp(ctx)
			}
		}
	}(ctx)
	wg.Wait()
}
