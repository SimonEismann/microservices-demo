package main

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"time"

	"contrib.go.opencensus.io/exporter/jaeger"
	"contrib.go.opencensus.io/exporter/zipkin"
	openzipkin "github.com/openzipkin/zipkin-go"
	zipkinhttp "github.com/openzipkin/zipkin-go/reporter/http"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/SimonEismann/microservices-demo/tree/master/src/recommendationservice/genproto"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

const MaxRecomms = 5

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

type recommendationService struct {
	listRecommsDelay	int64
	prodcatserviceAddr	string
}

func (a *recommendationService) GetProductList(ctx context.Context) (*[]string, error) {
	conn, err := grpc.DialContext(ctx, a.prodcatserviceAddr,
		grpc.WithInsecure(),
		grpc.WithStatsHandler(&ocgrpc.ClientHandler{}))
	if err != nil {
		return nil, fmt.Errorf("could not connect product service: %+v", err)
	}
	defer conn.Close()

	prodList, err := pb.NewProductCatalogServiceClient(conn).ListProducts(ctx, new(pb.Empty))
	if err != nil {
		return nil, fmt.Errorf("failed to get product list: %+v", err)
	}
	var products []string
	for i := 0; i < len(prodList.Products); i++ {
		products = append(products, prodList.Products[i].Id)
	}
	return &products, nil
}


func (a *recommendationService) ListRecommendations(c context.Context, request *pb.ListRecommendationsRequest) (*pb.ListRecommendationsResponse, error) {
	passTime(a.listRecommsDelay)
	prodList, err := a.GetProductList(c)
	if err != nil {
		return nil, err
	}
	var res []string
	for i := 0; i < len(*prodList); i++ {
		pid := (*prodList)[i]
		found := false
		for rpid := range request.ProductIds {
			if pid == request.ProductIds[rpid] {
				found = true
				break
			}
		}
		if !found {
			res = append(res, pid)
		}
	}
	rand.Shuffle(len(res), func(i, j int) { res[i], res[j] = res[j], res[i] })
	return &pb.ListRecommendationsResponse{ProductIds: res[:min(len(res), MaxRecomms)]}, nil
}

func min(n1 int, n2 int) int {
	if n1 < n2 {
		return n1
	} else {
		return n2
	}
}

func main() {
	go initTracing()

	port := "8080"
	if os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}

	svc := new(recommendationService)
	mustMapEnvInt64(&svc.listRecommsDelay, "DELAY_LIST_RECOMMS")
	mustMapEnv(&svc.prodcatserviceAddr, "PRODUCT_CATALOG_SERVICE_ADDR")
	log.Infof("service config: %+v", svc)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatal(err)
	}

	go initHealthServer()

	srv := grpc.NewServer(grpc.StatsHandler(&ocgrpc.ServerHandler{}))
	pb.RegisterRecommendationServiceServer(srv, svc)
	log.Infof("starting to listen on tcp: %q", lis.Addr().String())
	err = srv.Serve(lis)
	log.Fatal(err)
}

func initHealthServer() {
	port := "8081"
	if os.Getenv("HEALTH_PORT") != "" {
		port = os.Getenv("HEALTH_PORT")
	}
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatal(err)
	}
	srv := grpc.NewServer()
	healthpb.RegisterHealthServer(srv, new(recommendationService))
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
			ServiceName: "recommendationservice",
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

	endpoint, err := openzipkin.NewEndpoint("recommendationservice", "")
	if err != nil {
		log.Fatalf("unable to create local endpoint: %+v\n", err)
	}
	reporter := zipkinhttp.NewReporter(fmt.Sprintf("http://%s/api/v2/spans", svcAddr))
	exporter := zipkin.NewExporter(reporter, endpoint)
	trace.RegisterExporter(exporter)

	log.Info("zipkin initialization completed.")
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

func mustMapEnv(target *string, envKey string) {
	v := os.Getenv(envKey)
	if v == "" {
		panic(fmt.Sprintf("environment variable %q not set", envKey))
	}
	*target = v
}

func (a *recommendationService) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
}

func (a *recommendationService) Watch(req *healthpb.HealthCheckRequest, ws healthpb.Health_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "health check via Watch not implemented")
}
