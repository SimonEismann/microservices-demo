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
	"errors"
	"fmt"
	"github.com/google/uuid"
	"net"
	"net/http"
	"os"
	"regexp"
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

	pb "github.com/SimonEismann/microservices-demo/src/paymentservice/genproto"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

const (
	listenPort       = "50051"
	metricsPort      = "50052"
	healthListenPort = "50053"
)

var (
	log *logrus.Logger
	cardNumberValidator = regexp.MustCompile(`^((4\d{3})|(5[1-5]\d{2})|(6011))-?\d{4}-?\d{4}-?\d{4}|3[4,7]\d{13}$`)
)

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

// repeats multiplication of random 50x50 matrices t times
func passTime(t int64) {
	if t <= 0 { return }
	for i := int64(0); i < t; i++ {
		a := *createMatrix(50)
		b := *createMatrix(50)
		res := make([]float64, 50 * 50)		// res(spalte, zeile) = res[zeile * 50 + spalte], genauso bei a und b
		for z := 0; z < 50; z++ {			// Zeile
			for s := 0; s < 50; s++ {		// Spalte
				dot_product := float64(0)
				for k := 0; k < 50; k++ {	// spaltenindex von a, zeilenindex von b
					dot_product += a[z * 50 + k] * b[k * 50 + s]
				}
				res[z * 50 + s] = dot_product
			}
		}
	}
}

// helper function for square matrix generation of passTime(t)
func createMatrix(size int) *[]float64 {
	data := make([]float64, size * size)
	for i := range data {
		data[i] = float64(i)
	}
	return &data
}

type paymentService struct {
	chargeDelay	int64
}

// original paymentService only accepts visa and mastercard. we accept every valid 15/16-digit card number (visa, mastercard, amex, discover) that is not expired.
func (ps *paymentService) Charge(c context.Context, request *pb.ChargeRequest) (*pb.ChargeResponse, error) {
	passTime(ps.chargeDelay)
	isValid := cardNumberValidator.MatchString(request.CreditCard.CreditCardNumber)
	isExpired := (request.CreditCard.CreditCardExpirationYear < int32(time.Now().Year())) || (request.CreditCard.CreditCardExpirationYear == int32(time.Now().Year()) && request.CreditCard.CreditCardExpirationMonth < int32(time.Now().Month()))

	if !isValid && isExpired {
		fmt.Printf("card number %s is invalid and expired!", request.CreditCard.CreditCardNumber)
		return nil, errors.New("the card is invalid and expired")
	} else if !isValid {
		fmt.Printf("card number %s is invalid!", request.CreditCard.CreditCardNumber)
		return nil, errors.New("the card is invalid")
	} else if isExpired {
		fmt.Printf("card number %s is expired!", request.CreditCard.CreditCardNumber)
		return nil, errors.New("the card is expired")
	}

	uuidString := uuid.New().String()
	resp := pb.ChargeResponse{
		TransactionId:        uuidString,		// return a random uuid
	}
	fmt.Printf("Transaction %s processed. Cardnumber: %s, Amount: %d.%d %s", uuidString, request.CreditCard.CreditCardNumber, request.Amount.Units, request.Amount.Nanos, request.Amount.CurrencyCode)
	return &resp, nil
}

func main() {
	go initTracing()
	port := listenPort
	if os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}

	svc := new(paymentService)
	mustMapEnvInt64(&svc.chargeDelay, "DELAY_CHARGE")

	log.Infof("service config: %+v", svc)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatal(err)
	}
	go initPrometheusStats()
	go initHealthServer()
	srv := grpc.NewServer(grpc.StatsHandler(&ocgrpc.ServerHandler{}))
	pb.RegisterPaymentServiceServer(srv, svc)
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
	healthpb.RegisterHealthServer(srv, new(paymentService))
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
			ServiceName: "paymentservice",
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
	endpoint, err := openzipkin.NewEndpoint("paymentservice", "")
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

func mustMapEnvInt64(target *int64, envKey string) {
	v := os.Getenv(envKey)
	if v == "" {
		panic(fmt.Sprintf("environment variable %q not set", envKey))
	}
	if n, err := strconv.ParseInt(v, 10, 64); err == nil {
		*target = n
	} else {
		panic(fmt.Sprintf("environment variable %q not an int64", envKey))
	}
}

func (ps *paymentService) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
}

func (ps *paymentService) Watch(req *healthpb.HealthCheckRequest, ws healthpb.Health_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "health check via Watch not implemented")
}