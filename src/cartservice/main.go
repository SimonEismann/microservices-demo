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
	"os"
	"strconv"
	"strings"
	"time"

	"contrib.go.opencensus.io/exporter/jaeger"
	"contrib.go.opencensus.io/exporter/zipkin"
	"github.com/go-redis/redis/v8"
	openzipkin "github.com/openzipkin/zipkin-go"
	zipkinhttp "github.com/openzipkin/zipkin-go/reporter/http"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/SimonEismann/microservices-demo/src/cartservice/genproto"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

const (
	listenPort       = "7070"
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

// convert cart content to csv
func cartItemsToString(items *[]*pb.CartItem) *string {
	res := ""
	if items != nil {
		for i := 0; i < len(*items); i++ {
			item := (*items)[i]
			res += item.ProductId + ";" + strconv.FormatInt(int64(item.Quantity), 10) + "\n"
		}
	}
	return &res
}

// parses cart item csv to CartItem array/slice
func cartItemsFromString(data *string) *[]*pb.CartItem {
	lines := strings.Split(*data, "\n")
	items := []*pb.CartItem{}
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		temp := strings.Split(line, ";")
		if len(temp) == 2 {
			quantity, err := strconv.ParseInt(temp[1], 10, 32)
			if err != nil {
				log.Error("Could not convert quantity string " + temp[1] + " to int32!")
			} else {
				item := &pb.CartItem{
					ProductId: temp[0],
					Quantity:  int32(quantity),
				}
				items = append(items, item)
			}
		}
	}
	return &items
}

type cartService struct {
	redisSvcAddr string
	addItemDelay int64
	getCartDelay int64
	emptyCartDelay int64
	isDummy bool
}

func (cs *cartService) ConnectToRedis(c context.Context) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cs.redisSvcAddr,
		Password: "", // no password set
		DB:       0,  // use default DB
		MaxRetries: 5, // like in the c# implementation
	})
	return rdb
}

func (cs *cartService) AddItem(c context.Context, request *pb.AddItemRequest) (*pb.Empty, error) {
	passTime(cs.addItemDelay)
	if !cs.isDummy {
		rdb := cs.ConnectToRedis(c)
		val, err := rdb.Get(c, request.UserId).Result()		// redis maps keys (userId) to value (cart items as string)
		cart := pb.Cart{UserId: request.UserId}
		if err == redis.Nil {
			log.Info("cart for " + request.UserId + "does not exist")
			cart.Items = []*pb.CartItem{}
		} else if err != nil {
			log.Fatal(err)
		} else {
			log.Info("Cart retrieved: " + val)
			cart.Items = *cartItemsFromString(&val)
		}
		foundExisting := false
		for _, item := range cart.Items {
			if item.ProductId == request.Item.ProductId {
				foundExisting = true
				item.Quantity += request.Item.Quantity
				break
			}
		}
		if !foundExisting {
			cart.Items = append(cart.Items, request.Item)
		}
		err2 := rdb.Set(c, request.UserId, *cartItemsToString(&cart.Items), 0).Err()
		if err2 != nil {
			log.Fatal(err2)
		}
	}
	return &pb.Empty{}, nil
}

func (cs *cartService) GetCart(c context.Context, request *pb.GetCartRequest) (*pb.Cart, error) {
	passTime(cs.getCartDelay)
	cart := pb.Cart{UserId: request.UserId}
	if cs.isDummy {
		cart.Items = []*pb.CartItem{{
			ProductId: "0PUK6V6EV0",
			Quantity:  1,
				}}
	} else {
		rdb := cs.ConnectToRedis(c)
		val, err := rdb.Get(c, request.UserId).Result()		// redis maps keys (userId) to value (cart items as string)
		if err == redis.Nil {
			log.Info("cart for " + request.UserId + " does not exist")
			cart.Items = []*pb.CartItem{}
		} else if err != nil {
			log.Fatal(err)
		} else {
			log.Info("Cart retrieved: " + val)
			cart.Items = *cartItemsFromString(&val)
		}
	}
	return &cart, nil
}

// Deletes cart from redis for specific userId
func (cs *cartService) EmptyCart(c context.Context, request *pb.EmptyCartRequest) (*pb.Empty, error) {
	passTime(cs.emptyCartDelay)
	if !cs.isDummy {
		rdb := cs.ConnectToRedis(c)
		err := rdb.Del(c, request.UserId).Err()
		if err != nil {
			log.Fatal(err)
		}
	}
	return &pb.Empty{}, nil
}

func main() {
	go initTracing()
	port := listenPort
	if os.Getenv("PORT") != "" {
		port = os.Getenv("PORT")
	}
	svc := new(cartService)
	svc.isDummy = os.Getenv("DUMMY") != ""
	mustMapEnv(&svc.redisSvcAddr, "REDIS_ADDR")
	mustMapEnvInt64(&svc.addItemDelay, "DELAY_ADD_ITEM")
	mustMapEnvInt64(&svc.getCartDelay, "DELAY_GET_CART")
	mustMapEnvInt64(&svc.emptyCartDelay, "DELAY_EMPTY_CART")
	log.Infof("service config: %+v", svc)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		log.Fatal(err)
	}
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

func (cs *cartService) Check(ctx context.Context, req *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	return &healthpb.HealthCheckResponse{Status: healthpb.HealthCheckResponse_SERVING}, nil
}

func (cs *cartService) Watch(req *healthpb.HealthCheckRequest, ws healthpb.Health_WatchServer) error {
	return status.Errorf(codes.Unimplemented, "health check via Watch not implemented")
}