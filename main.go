package main

import (
	"log"
	"servicesCommunication/internal/grpclog"
	myServer "servicesCommunication/internal/server"
	"servicesCommunication/internal/service"
	"servicesCommunication/internal/setting"
	"servicesCommunication/internal/utils"
	"sync"
	"time"
)

func main() {
	settings := setting.LoadSetting()
	utils.SetUpUtils(settings.App.ServiceName, settings.ServerConfig.PortMin)
	grpclog.Setup(&settings.App)
	defer func() {
		err := grpclog.Close()
		if err != nil {
			log.Print(err)
		}
	}()

	client, err := grpclog.NewInfluxLogger(
		settings.App.UnfluxDBToken,
		settings.App.UnfluxDBURL,
		settings.App.UnfluxDBOrgName,
		settings.App.UnfluxDBBucketName,
	)
	if err != nil {
		grpclog.Fatal(err)
	}
	srv := myServer.NewServer(settings.ServerConfig.Port, settings.ServerConfig.FrequencyCommunication, *client)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		srv.Serve(settings.ServerConfig.Port)
	}()
	neighbors := service.NewNode(settings.ServerConfig.Port, settings.Nodes, settings.ServerConfig.FrequencyCommunication, srv)
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
