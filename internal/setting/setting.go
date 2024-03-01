package setting

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

// App is a structure for storage app configuration
type App struct {
	LogOut          io.Writer
	RuntimeRootPath string
	ServiceName     string
}

// ServerSetting is a structure for storage user_protobuf configuration
type ServerSetting struct {
	RunMode                string
	Port                   string
	FrequencyCommunication time.Duration
	NodeCountByDefault     int
	// ReadTimeout  time.Duration
	Timeout time.Duration
	// Path string
}

type NodeSetting struct {
	Host string
	Port string
}

// Setting is a structure for storage all settings
type Setting struct {
	ServerConfig ServerSetting
	Nodes        []string
	App          App
}

// NewSetting loads configuration from env variables
func NewSetting() *Setting {
	nodeCount, _ := strconv.Atoi(getEnv("NODE_COUNT"))
	nodeNames := make([]string, 0, nodeCount-1)
	port := getEnv("APP_PORT")
	var serviceName string
	// need a time to set up another nodes
	time.Sleep(time.Second * time.Duration(nodeCount))
	for i := 1; i <= nodeCount; i++ {
		nodeName := fmt.Sprintf("%s%d:%s", getEnv("APP_SERVICE"), i, port)
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

	s := Setting{
		ServerConfig: ServerSetting{
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
	}

	return &s
}

// Simple helper function to read an environment or return a default value
func getEnv(key string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}

	return ""
}

func getFileLog() io.Writer {
	filePath := filepath.Join(".", getEnv("LOG_PATH"))
	fileName := getEnv("LOG_NAME") + "." + getEnv("LOG_EXT")
	f, err := file.MustOpen(fileName, filePath)
	if err != nil {
		log.Fatalf("grpclog.Setup err: %v", err)
	}

	return f
}
