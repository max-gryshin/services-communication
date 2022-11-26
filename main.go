package main

import (
	"fiveServices/internal/grpclog"
	myServer "fiveServices/internal/server"
	"fiveServices/internal/service"
	"fiveServices/internal/setting"
	"fiveServices/internal/utils"
	"github.com/joho/godotenv"
	"log"
	"strconv"
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
	port, err := strconv.Atoi(settings.ServerConfig.Port)
	if err != nil {
		grpclog.Error(err)
	}
	srv := myServer.NewServer(port, settings.ServerConfig.FrequencyCommunication)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		srv.Serve(port)
	}()
	neighbors := service.NewNode(port, settings.Nodes, settings.ServerConfig.FrequencyCommunication, srv)
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
