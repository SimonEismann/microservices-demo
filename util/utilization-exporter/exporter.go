package main

import (
	"context"
	"fmt"
	"google.golang.org/api/iterator"
	"log"
	"os"
	"strconv"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3"
	googlepb "github.com/golang/protobuf/ptypes/timestamp"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
)

func main() {
	ctx := context.Background()
	argsWithoutProg := os.Args[1:]	// first: gcloud project ID, second: time interval (in seconds)

	client, err := monitoring.NewMetricClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	projectID := argsWithoutProg[0]
	interval, err := strconv.ParseInt(argsWithoutProg[1],10, 64)
	if err != nil {
		log.Fatalf("Failed to parse interval: %v", err)
	}
	endTime := time.Now().Unix()
	startTime := endTime - interval

	ts := client.ListTimeSeries(ctx, &monitoringpb.ListTimeSeriesRequest{
		Name:        "projects/" + projectID,
		Filter:      "metric.type = \"compute.googleapis.com/instance/cpu/utilization\"",
		Interval:    &monitoringpb.TimeInterval{
			EndTime:   &googlepb.Timestamp{
				Seconds: endTime,
			},
			StartTime: &googlepb.Timestamp{
				Seconds: startTime,
			},
		},
	})

	for true {
		timeseries, isDone := ts.Next()
		if isDone == iterator.Done {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Timeseries: %s\n", timeseries.String())
		points := timeseries.Points
		for i := 0; i < len(points); i++  {
			fmt.Printf("Point: %s --> %s\n", points[i].Interval.String(), points[i].Value.String())
		}
	}
}