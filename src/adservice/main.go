package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
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

	pb "github.com/SimonEismann/microservices-demo/tree/master/src/adservice/genproto"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

const (
	listenPort       = "9555"
	metricsPort      = "9556"
	healthListenPort = "9557"
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

type adService struct {
	ads         []*pb.Ad
	getAdsDelay	int64
}

func main() {
	go initTracing()

	port := listenPort
	if os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}

	svc := new(adService)
	mustMapEnvInt64(&svc.getAdsDelay, "DELAY_GETADS")
	log.Infof("service config: %+v", svc)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatal(err)
	}

	// start metrics endpoint
	go initPrometheusStats()

	go initHealthServer()

	err = svc.loadAdsFile()
	if err != nil {
		log.Fatalf("error parsing Ads json file %s", err)
	}

	srv := grpc.NewServer(grpc.StatsHandler(&ocgrpc.ServerHandler{}))
	pb.RegisterAdServiceServer(srv, svc)
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
	healthpb.RegisterHealthServer(srv, new(adService))
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

	// Register the Jaeger exporter to be able to retrieve
	// the collected spans.
	exporter, err := jaeger.NewExporter(jaeger.Options{
		Endpoint: fmt.Sprintf("http://%s", svcAddr),
		Process: jaeger.Process{
			ServiceName: "adservice",
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

	endpoint, err := openzipkin.NewEndpoint("adservice", "")
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

func (a *adService) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
}

func (a *adService) Watch(req *healthpb.HealthCheckRequest, ws healthpb.Health_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "health check via Watch not implemented")
}

func (a *adService) loadAdsFile() error {
	data, err := ioutil.ReadFile("ads.json")
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, &a.ads)
	if err != nil {
		return err
	}
	fmt.Printf("found %d ads\n", len(a.ads))

	return nil
}

func (a *adService) GetAds(ctx context.Context, req *pb.AdRequest) (*pb.AdResponse, error) {
	passTime(a.getAdsDelay)
	log.Infof("[GetAds] contextKeys=%q", req.ContextKeys)

	resp := &pb.AdResponse{Ads: a.ads}
	return resp, nil
}
