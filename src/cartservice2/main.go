// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"contrib.go.opencensus.io/exporter/jaeger"
	"contrib.go.opencensus.io/exporter/prometheus"
	"contrib.go.opencensus.io/exporter/zipkin"
	"github.com/go-redis/redis/v8"
	openzipkin "github.com/openzipkin/zipkin-go"
	zipkinhttp "github.com/openzipkin/zipkin-go/reporter/http"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/GoogleCloudPlatform/microservices-demo/src/checkoutservice/genproto"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

const (
	listenPort       = "7070"
	metricsPort      = "7071"
	healthListenPort = "7072"
)

var log *logrus.Logger

func init() {
	log = logrus.New()
	log.Level = logrus.DebugLevel
	log.Formatter = &logrus.JSONFormatter{
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "severity",
			logrus.FieldKeyMsg:   "message",
		},
		TimestampFormat: time.RFC3339Nano,
	}
	log.Out = os.Stdout
}

type cartService struct {
	redisSvcAddr string
}

func (cs *cartService) ConnectToRedis() *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cs.redisSvcAddr,
		Password: "", // no password set
		DB:       0,  // use default DB
		MaxRetries: 5,
	})
	pong, err := rdb.Ping(context.Background()).Result()
	if err != nil {
		log.Fatal(err)
	} else {
		log.Info(pong)
	}
	return rdb
}

func (cs *cartService) AddItem(c context.Context, request *pb.AddItemRequest) (*pb.Empty, error) {
	rdb := cs.ConnectToRedis()
	val, err := rdb.Get(c, "cart").Result()
	if err != nil {
		log.Fatal(err)
	} else {
		log.Info(val)
	}
	return &pb.Empty{}, nil		//DEBUG
}

func (cs *cartService) GetCart(c context.Context, request *pb.GetCartRequest) (*pb.Cart, error) {
	rdb := cs.ConnectToRedis()
	val, err := rdb.Get(c, "cart").Result()
	if err != nil {
		log.Fatal(err)
	} else {
		log.Info(val)
	}
	return &pb.Cart{}, nil		//DEBUG
}

func (cs *cartService) EmptyCart(c context.Context, request *pb.EmptyCartRequest) (*pb.Empty, error) {
	rdb := cs.ConnectToRedis()
	val, err := rdb.Get(c, "cart").Result()
	if err != nil {
		log.Fatal(err)
	} else {
		log.Info(val)
	}
	return &pb.Empty{}, nil		//DEBUG
}

func main() {
	go initTracing()
	port := listenPort
	if os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}
	svc := new(cartService)
	mustMapEnv(&svc.redisSvcAddr, "REDIS_ADDR")
	log.Infof("service config: %+v", svc)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatal(err)
	}
	go initPrometheusStats()
	go initHealthServer()
	srv := grpc.NewServer(grpc.StatsHandler(&ocgrpc.ServerHandler{}))
	pb.RegisterCartServiceServer(srv, svc)
	log.Infof("starting to listen on tcp: %q", lis.Addr().String())
	err = srv.Serve(lis)
	log.Fatal(err)
}

func initHealthServer() {
	port := healthListenPort
	if os.Getenv("HEALTH_PORT") != "" {
		port = os.Getenv("HEALTH_PORT")
	}
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatal(err)
	}
	srv := grpc.NewServer()
	healthpb.RegisterHealthServer(srv, new(cartService))
	log.Infof("starting to listen on tcp: %q", lis.Addr().String())
	err = srv.Serve(lis)
	log.Fatal(err)
}

func initJaegerTracing() {
	svcAddr := os.Getenv("JAEGER_SERVICE_ADDR")
	if svcAddr == "" {
		log.Info("jaeger initialization disabled.")
		return
	}
	exporter, err := jaeger.NewExporter(jaeger.Options{
		Endpoint: fmt.Sprintf("http://%s", svcAddr),
		Process: jaeger.Process{
			ServiceName: "cartservice",
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	trace.RegisterExporter(exporter)
	log.Info("jaeger initialization completed.")
}

func initZipkinTracing() {
	svcAddr := os.Getenv("ZIPKIN_SERVICE_ADDR")
	if svcAddr == "" {
		log.Info("zipkin initialization disabled.")
		return
	}
	endpoint, err := openzipkin.NewEndpoint("cartservice", "")
	if err != nil {
		log.Fatalf("unable to create local endpoint: %+v\n", err)
	}
	reporter := zipkinhttp.NewReporter(fmt.Sprintf("http://%s/api/v2/spans", svcAddr))
	exporter := zipkin.NewExporter(reporter, endpoint)
	trace.RegisterExporter(exporter)
	log.Info("zipkin initialization completed.")
}

func initPrometheusStats() {
	exporter, err := prometheus.NewExporter(prometheus.Options{})
	if err != nil {
		log.Fatal(err)
	}
	initStats(exporter)
	metricsURL := fmt.Sprintf(":%s", metricsPort)
	http.Handle("/metrics", exporter)
	log.Infof("starting HTTP server at %s", metricsURL)
	log.Fatal(http.ListenAndServe(metricsURL, nil))
}

func initStats(exporter *prometheus.Exporter) {
	view.SetReportingPeriod(60 * time.Second)
	view.RegisterExporter(exporter)
	if err := view.Register(ocgrpc.DefaultServerViews...); err != nil {
		log.Warn("Error registering default server views")
	} else {
		log.Info("Registered default server views")
	}
	if err := view.Register(ocgrpc.DefaultClientViews...); err != nil {
		log.Warn("Error registering default client views")
	} else {
		log.Info("Registered default client views")
	}
}

func initTracing() {
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})
	initJaegerTracing()
	initZipkinTracing()
}

func mustMapEnv(target *string, envKey string) {
	v := os.Getenv(envKey)
	if v == "" {
		panic(fmt.Sprintf("environment variable %q not set", envKey))
	}
	*target = v
}

func (cs *cartService) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
}

func (cs *cartService) Watch(req *healthpb.HealthCheckRequest, ws healthpb.Health_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "health check via Watch not implemented")
}