package grpclog

import (
	"context"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"time"
)

type InfluxLogger struct {
	client     influxdb2.Client
	orgName    string
	bucketName string
}

func NewInfluxLogger(token string, url string, orgName string, bucketName string) (*InfluxLogger, error) {
	client := influxdb2.NewClient(url, token)
	<-time.After(time.Second * 1) // give influx db initialize
	_, err := client.Health(context.Background())
	if err != nil {
		return nil, err
	}

	return &InfluxLogger{
		client,
		orgName,
		bucketName,
	}, err
}

func (il *InfluxLogger) WriteEvent(side string, tags map[string]string) {
	writeAPI := il.client.WriteAPI(il.orgName, il.bucketName)
	fields := make(map[string]interface{})
	for key, val := range tags {
		fields[key] = val
	}
	p := influxdb2.NewPoint(side, tags, fields, time.Now())
	writeAPI.WritePoint(p)
	writeAPI.Flush()
}
