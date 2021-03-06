package main

import (
	"context"
	"fmt"
	"google.golang.org/api/iterator"
	"log"
	"os"
	"sort"
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
		Name:   "projects/" + projectID,
		Filter: "metric.type = \"compute.googleapis.com/instance/cpu/utilization\"",
		Interval: &monitoringpb.TimeInterval{
			EndTime: &googlepb.Timestamp{
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
		avg_util := 0.0
		max_util := 0.0
		for i := 0; i < len(points); i++  {
			fmt.Printf("Point: %s --> %s\n", points[i].Interval.String(), points[i].Value.String())
			avg_util += points[i].Value.GetDoubleValue()
			if points[i].Value.GetDoubleValue() > max_util {
				max_util = points[i].Value.GetDoubleValue()
			}
		}
		fmt.Printf("Average Utilization: %f\n", avg_util / float64(len(points)))
		fmt.Printf("Max Utilization: %f\n", max_util)
		// get the top 3 utilizations
		if len(points) >=3 {
			sort.SliceStable(points, func(i, j int) bool {
				return points[i].Value.GetDoubleValue() < points[j].Value.GetDoubleValue()
			})
			fmt.Printf("Top3 Utilization: %f\n", (points[len(points)-1].Value.GetDoubleValue() + points[len(points)-2].Value.GetDoubleValue() + points[len(points)-3].Value.GetDoubleValue()) / 3)
		}
	}
}