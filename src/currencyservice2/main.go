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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	"contrib.go.opencensus.io/exporter/jaeger"
	"contrib.go.opencensus.io/exporter/prometheus"
	"contrib.go.opencensus.io/exporter/zipkin"
	openzipkin "github.com/openzipkin/zipkin-go"
	zipkinhttp "github.com/openzipkin/zipkin-go/reporter/http"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/SimonEismann/microservices-demo/src/currencyservice2/genproto"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

const (
	listenPort       = "7000"
	metricsPort      = "7001"
	healthListenPort = "7002"
	dataAddress		 = "currency_conversion.json"
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

type currencyService struct {
	dataAddr	string
	dataMap 	*map[string]float64
}

// helper function to load the currency_conversion.json
func (cs *currencyService) loadCurrenciesFile() {
	if cs.dataMap == nil {
		data, err := ioutil.ReadFile(cs.dataAddr)
		if err != nil {
			log.Fatal(err)
		}
		var tempResult map[string]string	// convert json to map
		err = json.Unmarshal(data, &tempResult)
		if err != nil {
			log.Fatal(err)
		}
		result := make(map[string]float64)	// convert string values to float64
		for k, v := range tempResult {
			convertedValue, err := strconv.ParseFloat(v, 64)
			if err != nil {
				log.Fatal(err)
			}
			result[k] = convertedValue
			fmt.Printf("%s: %s -> %f\n", k, v, convertedValue)
		}
		fmt.Printf("found %d currencies\n", len(result))
		cs.dataMap = &result
	}
}

func (cs *currencyService) GetSupportedCurrencies(c context.Context, empty *pb.Empty) (*pb.GetSupportedCurrenciesResponse, error) {
	cs.loadCurrenciesFile()
	keys := make([]string, 0, len(*cs.dataMap))
	for k := range *cs.dataMap {
		keys = append(keys, k)
	}
	resp := pb.GetSupportedCurrenciesResponse{
		CurrencyCodes: keys,
	}
	return &resp, nil
}

func (cs *currencyService) Convert(c context.Context, request *pb.CurrencyConversionRequest) (*pb.Money, error) {
	cs.loadCurrenciesFile()
	fromCurrency := request.From.CurrencyCode
	baseFactor, wasFound := (*cs.dataMap)[fromCurrency]	// convert to euro base
	if !wasFound {
		log.Fatalf("did not find from-currency " + fromCurrency)
	}
	baseNanos := math.Round(float64(request.From.Nanos) / baseFactor)
	baseUnits := float64(request.From.Units) / baseFactor
	toFactor, wasFound := (*cs.dataMap)[request.ToCode]	// convert to to-currency
	if !wasFound {
		log.Fatalf("did not find to-currency " + request.ToCode)
	}
	resUnits := int64(baseUnits * toFactor)
	resNanos := int32(baseNanos * toFactor)
	fmt.Printf("converted U:%d N:%d %s to U:%d N:%d %s", request.From.Units, request.From.Nanos, fromCurrency, resUnits, resNanos, request.ToCode)
	resp := pb.Money{
		CurrencyCode:         request.ToCode,
		Units:                resUnits,
		Nanos:                resNanos,
	}
	return &resp, nil
}

func main() {
	go initTracing()
	port := listenPort
	if os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}
	svc := new(currencyService)
	svc.dataAddr = dataAddress
	if os.Getenv("DATA_ADDR") != "" {
		port = os.Getenv("DATA_ADDR")
	}
	// mustMapEnv(&svc.shippingSvcAddr, "SHIPPING_SERVICE_ADDR")

	log.Infof("service config: %+v", svc)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatal(err)
	}
	go initPrometheusStats()
	go initHealthServer()
	srv := grpc.NewServer(grpc.StatsHandler(&ocgrpc.ServerHandler{}))
	pb.RegisterCurrencyServiceServer(srv, svc)
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
	healthpb.RegisterHealthServer(srv, new(currencyService))
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
			ServiceName: "currencyservice",
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	trace.RegisterExporter(exporter)
	log.Info("jaeger initialization completed.")
}

func initZipkinTracing() {
	// start zipkin exporter
	// URL to zipkin is like http://zipkin.tcc:9411/api/v2/spans
	svcAddr := os.Getenv("ZIPKIN_SERVICE_ADDR")
	if svcAddr == "" {
		log.Info("zipkin initialization disabled.")
		return
	}

	endpoint, err := openzipkin.NewEndpoint("currencyservice", "")
	if err != nil {
		log.Fatalf("unable to create local endpoint: %+v\n", err)
	}
	reporter := zipkinhttp.NewReporter(fmt.Sprintf("http://%s/api/v2/spans", svcAddr))
	exporter := zipkin.NewExporter(reporter, endpoint)
	trace.RegisterExporter(exporter)

	log.Info("zipkin initialization completed.")
}

func initPrometheusStats() {
	// init the prometheus /metrics endpoint
	exporter, err := prometheus.NewExporter(prometheus.Options{})
	if err != nil {
		log.Fatal(err)
	}

	// register basic views
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

func (cs *currencyService) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
}

func (cs *currencyService) Watch(req *healthpb.HealthCheckRequest, ws healthpb.Health_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "health check via Watch not implemented")
}