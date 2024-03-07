package config

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/max-gryshin/services-communication/internal/file"
	"github.com/max-gryshin/services-communication/internal/utils"
)

const (
	DevEnvironment  = "dev"
	ProdEnvironment = "prod"
)

// App is a structure for storage app configuration
type App struct {
	LogOut          io.Writer
	RuntimeRootPath string
	Name            string
}

// ServerConfig is a structure for storage user_protobuf configuration
type ServerConfig struct {
	RunMode                string
	Port                   string
	FrequencyCommunication time.Duration
	NodeCountByDefault     int
	// ReadTimeout  time.Duration
	Timeout time.Duration
	// Path string
}

// Config is a structure for storage all settings
type Config struct {
	ServerConfig                ServerConfig
	Nodes                       []string
	App                         App
	Environment                 string
	GracefullyShutdownTimeoutMs int
}

// New loads configuration from env variables
func New() *Config {
	nodeCount, _ := strconv.Atoi(getEnv("NODE_COUNT", "1"))
	nodeNames := make([]string, 0, nodeCount-1)
	port := getEnv("APP_PORT", "")
	var serviceName string
	// need a time to set up another nodes
	time.Sleep(time.Second * time.Duration(nodeCount))
	for i := 1; i <= nodeCount; i++ {
		nodeName := fmt.Sprintf("%s%d:%s", getEnv("APP_SERVICE", ""), i, port)
		tcpAddr, err := net.ResolveTCPAddr("tcp", nodeName)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		if tcpAddr.IP.String() != utils.GetLocalIP() {
			nodeNames = append(nodeNames, nodeName)
		} else {
			serviceName = nodeName
		}
	}

	s := Config{
		ServerConfig: ServerConfig{
			Port:                   port,
			FrequencyCommunication: time.Second * utils.GetRandDuration(1, 3),
			Timeout:                time.Second * 3,
			NodeCountByDefault:     nodeCount,
		},
		Nodes: nodeNames,
		App: App{
			os.Stdout, // getFileLog
			".",
			serviceName,
		},
		Environment:                 DevEnvironment,
		GracefullyShutdownTimeoutMs: 10000,
	}

	return &s
}

// Simple helper function to read an environment or return a default value
func getEnv(key string, def string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}

	return def
}

func getFileLog() io.Writer {
	filePath := filepath.Join(".", getEnv("LOG_PATH", ""))
	fileName := getEnv("LOG_NAME", "") + "." + getEnv("LOG_EXT", "")
	f, err := file.MustOpen(fileName, filePath)
	if err != nil {
		log.Fatalf("log.Setup err: %v", err)
	}

	return f
}
