package main

import (
	"context"
	"log"
	"servicesCommunication/internal/grpclog"
	myServer "servicesCommunication/internal/server"
	"servicesCommunication/internal/service"
	"servicesCommunication/internal/setting"
	"sync"
	"time"
)

func main() {
	settings := setting.NewSetting()
	grpclog.Setup(&settings.App)
	defer func() {
		err := grpclog.Close()
		if err != nil {
			log.Print(err)
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv := myServer.NewServer(settings.App.ServiceName, settings.Nodes, settings.ServerConfig.FrequencyCommunication)
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
