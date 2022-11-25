package setting

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// App is a structure for storage app configuration
type App struct {
	LogSavePath     string
	LogSaveName     string
	LogFileExt      string
	RuntimeRootPath string
	ServiceName     string
}

// ServerSetting is a structure for storage user_protobuf configuration
type ServerSetting struct {
	RunMode                string
	Host                   string
	Port                   string
	PortHTTP               string
	FrequencyCommunication time.Duration
	PortMin                int
	PortMax                int
	// ReadTimeout  time.Duration
	// WriteTimeout time.Duration
	// Path string
}

type NodeSetting struct {
	Host string
	Port string
}

// Setting is a structure for storage all settings
type Setting struct {
	ServerConfig ServerSetting
	Nodes        []int
	App          App
}

// LoadSetting loads configuration from env variables
func LoadSetting() *Setting {
	portFrom, err := strconv.Atoi(getEnv("APP_PORT_FROM"))
	if err != nil {
		fmt.Errorf("cannot get port of first node: %s", err.Error())
	}
	portTo, err := strconv.Atoi(getEnv("APP_PORT_TO"))
	if err != nil {
		fmt.Errorf("cannot get port of last node: %s", err.Error())
	}
	nodes := make([]int, 0, portTo-portFrom)
	for i := portFrom; i <= portTo; i++ {
		if strconv.Itoa(i) == getEnv("APP_PORT") {
			continue
		}
		nodes = append(nodes, i)
	}
	return &Setting{
		ServerConfig: ServerSetting{
			Host:                   getEnv("APP_HOST"),
			Port:                   getEnv("APP_PORT"),
			PortMin:                portFrom,
			PortMax:                portTo,
			PortHTTP:               getEnv("APP_PORT_HTTP"),
			FrequencyCommunication: time.Second * 2,
		},
		Nodes: nodes,
		App: App{
			getEnv("LOG_PATH"),
			getEnv("LOG_NAME"),
			getEnv("LOG_EXT"),
			".",
			getEnv("APP_SERVICE"),
		},
	}
}

// Simple helper function to read an environment or return a default value
func getEnv(key string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}

	return ""
}
