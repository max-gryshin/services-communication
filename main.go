package main

import (
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
	srv := myServer.NewServer(settings.App.ServiceName, settings.Nodes, settings.ServerConfig.FrequencyCommunication)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		srv.Serve(settings.ServerConfig.Port)
	}()
	// another servers need a time to start
	time.Sleep(time.Second * time.Duration(settings.ServerConfig.NodeCountByDefault/2))
	neighbors := service.NewNode(settings.App.ServiceName, settings.Nodes, settings.ServerConfig.FrequencyCommunication, srv)
	go func() {
		ticker := time.NewTicker(settings.ServerConfig.FrequencyCommunication)
		count := 0
		for range ticker.C {
			if count > 1e3 { // stop condition
				ticker.Stop()
				break
			}
			neighbors.LookUp()
			count++
		}
	}()
	wg.Wait()
}
