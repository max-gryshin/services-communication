package main

import (
	"github.com/joho/godotenv"
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
	if err := godotenv.Load(); err != nil {
		log.Print(err.Error())
		log.Print("No .env file found")
	}
	settings := setting.LoadSetting()
	utils.SetUpUtils(settings.App.ServiceName, settings.ServerConfig.PortMin)
	grpclog.Setup(&settings.App)
	defer func() {
		err := grpclog.Close()
		if err != nil {
			log.Print(err)
		}
	}()
	srv := myServer.NewServer(settings.ServerConfig.Port, settings.ServerConfig.FrequencyCommunication)
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
