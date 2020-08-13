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
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/mux"
	"github.com/namsral/flag"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/plugin/ochttp/propagation/b3"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
	"google.golang.org/grpc"

	"contrib.go.opencensus.io/exporter/jaeger"
	"contrib.go.opencensus.io/exporter/prometheus"
	"contrib.go.opencensus.io/exporter/zipkin"
	openzipkin "github.com/openzipkin/zipkin-go"
	zipkinhttp "github.com/openzipkin/zipkin-go/reporter/http"
)

const (
	defaultCurrency = "USD"
	cookieMaxAge    = 60 * 60 * 48

	cookiePrefix    = "shop_"
	cookieSessionID = cookiePrefix + "session-id"
	cookieCurrency  = cookiePrefix + "currency"
)

var (
	version               = "no version set"
	displayVersion        = flag.Bool("version", false, "Show version and quit")
	logLevel              = flag.String("logLevel", "debug", "log level from debug, info, warning, error. When debug, generate 100% Tracing")
	srvURL                = flag.String("srvURL", ":8080", "IP and port to bind, localhost:8080 or :8080")
	productCatalogSvcAddr = flag.String("PRODUCT_CATALOG_SERVICE_ADDR", "productcatalogservice:3550", "URL to productCatalog service")
	currencySvcAddr       = flag.String("CURRENCY_SERVICE_ADDR", "currencyservice:7000", "URL to Currency service")
	cartSvcAddr           = flag.String("CART_SERVICE_ADDR", "cartservice:7070", "URL to Cart service")
	recommendationSvcAddr = flag.String("RECOMMENDATION_SERVICE_ADDR", "recommendationservice:8080", "URL to Recommendation service")
	checkoutSvcAddr       = flag.String("CHECKOUT_SERVICE_ADDR", "checkoutservice:5050", "URL to Checkout service")
	shippingSvcAddr       = flag.String("SHIPPING_SERVICE_ADDR", "shippingservice:50051", "URL to Shipping service")
	adSvcAddr             = flag.String("AD_SERVICE_ADDR", "adservice:9555", "URL to Ad service")
	jaegerSvcAddr         = flag.String("JAEGER_SERVICE_ADDR", "", "URL to Jaeger Tracing agent")
	zipkinSvcAddr         = flag.String("ZIPKIN_SERVICE_ADDR", "", "URL to Zipkin Tracing agent (ex: zipkin:9411)")

	whitelistedCurrencies = map[string]bool{
		"USD": true,
		"EUR": true,
		"CAD": true,
		"JPY": true,
		"GBP": true,
		"TRY": true}
)

type ctxKeySessionID struct{}

type frontendServer struct {
	productCatalogSvcAddr string
	productCatalogSvcConn *grpc.ClientConn

	currencySvcAddr string
	currencySvcConn *grpc.ClientConn

	cartSvcAddr string
	cartSvcConn *grpc.ClientConn

	recommendationSvcAddr string
	recommendationSvcConn *grpc.ClientConn

	checkoutSvcAddr string
	checkoutSvcConn *grpc.ClientConn

	shippingSvcAddr string
	shippingSvcConn *grpc.ClientConn

	adSvcAddr string
	adSvcConn *grpc.ClientConn
}

func main() {
	// parse flags
	flag.Parse()
	if *displayVersion {
		fmt.Println(version)
		os.Exit(0)
	}

	// setup logs
	ctx := context.Background()
	log := logrus.New()
	log.Formatter = &logrus.JSONFormatter{
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "severity",
			logrus.FieldKeyMsg:   "message",
		},
		TimestampFormat: time.RFC3339Nano,
	}
	log.Out = os.Stdout
	currLogLevel, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		log.Fatalf("error parsing Log Level %s", err)
	}
	log.Level = currLogLevel

	// init Opencensus tracing
	go initTracing(log)

	svc := &frontendServer{
		adSvcAddr:             *adSvcAddr,
		cartSvcAddr:           *cartSvcAddr,
		checkoutSvcAddr:       *checkoutSvcAddr,
		currencySvcAddr:       *currencySvcAddr,
		productCatalogSvcAddr: *productCatalogSvcAddr,
		recommendationSvcAddr: *recommendationSvcAddr,
		shippingSvcAddr:       *shippingSvcAddr,
	}

	mustConnGRPC(ctx, &svc.currencySvcConn, svc.currencySvcAddr)
	mustConnGRPC(ctx, &svc.productCatalogSvcConn, svc.productCatalogSvcAddr)
	mustConnGRPC(ctx, &svc.cartSvcConn, svc.cartSvcAddr)
	mustConnGRPC(ctx, &svc.recommendationSvcConn, svc.recommendationSvcAddr)
	mustConnGRPC(ctx, &svc.shippingSvcConn, svc.shippingSvcAddr)
	mustConnGRPC(ctx, &svc.checkoutSvcConn, svc.checkoutSvcAddr)
	mustConnGRPC(ctx, &svc.adSvcConn, svc.adSvcAddr)

	r := mux.NewRouter()
	r.HandleFunc("/", svc.homeHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc("/product/{id}", svc.productHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc("/cart", svc.viewCartHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc("/cart", svc.addToCartHandler).Methods(http.MethodPost)
	r.HandleFunc("/cart/empty", svc.emptyCartHandler).Methods(http.MethodPost)
	r.HandleFunc("/setCurrency", svc.setCurrencyHandler).Methods(http.MethodPost)
	r.HandleFunc("/logout", svc.logoutHandler).Methods(http.MethodGet)
	r.HandleFunc("/cart/checkout", svc.placeOrderHandler).Methods(http.MethodPost)
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))
	r.HandleFunc("/robots.txt", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "User-agent: *\nDisallow: /") })
	// Opencensus-go ignore "/healthz" by default, refer to below link for more details
	// https://github.com/census-instrumentation/opencensus-go/blob/aad2c527c5defcf89b5afab7f37274304195a6b2/plugin/ochttp/trace.go#L231
	r.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, "ok") })

	// also init the prometheus handler
	initPrometheusStats(log, r)

	var handler http.Handler = r
	handler = &logHandler{log: log, next: handler} // add logging
	handler = ensureSessionID(handler)             // add session ID
	handler = &ochttp.Handler{                     // add opencensus instrumentation
		Handler:     handler,
		Propagation: &b3.HTTPFormat{}}

	srv := &http.Server{
		Handler: handler,
		Addr:    *srvURL,
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 360 * time.Second,
		ReadTimeout:  360 * time.Second,
	}

	go func() {
		log.Infof("starting server on %s", *srvURL)
		log.Fatal(srv.ListenAndServe())
	}()

	// trap SIGINT to trigger a shutdown.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	for {
		select {
		case <-signals:
			ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
			srv.Shutdown(ctx)
			return
		}
	}

}

func initTracing(log *logrus.Logger) {
	// This is a demo app with low QPS. trace.AlwaysSample() is used here
	// to make sure traces are available for observation and analysis.
	// In a production environment or high QPS setup please use
	// trace.ProbabilitySampler set at the desired probability.
	if log.Level == logrus.DebugLevel {
		trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})
	}

	initJaegerTracing(log)
	initZipkinTracing(log)
}

func initJaegerTracing(log *logrus.Logger) {

	if *jaegerSvcAddr == "" {
		log.Info("jaeger initialization disabled.")
		return
	}

	// Register the Jaeger exporter to be able to retrieve
	// the collected spans.
	exporter, err := jaeger.NewExporter(jaeger.Options{
		Endpoint: fmt.Sprintf("http://%s", *jaegerSvcAddr),
		Process: jaeger.Process{
			ServiceName: "frontend",
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	trace.RegisterExporter(exporter)
	log.Info("jaeger initialization completed.")
}

func initZipkinTracing(log *logrus.Logger) {
	// start zipkin exporter
	// URL to zipkin is like http://zipkin.tcc:9411/api/v2/spans
	if *zipkinSvcAddr == "" {
		log.Info("zipkin initialization disabled.")
		return
	}

	endpoint, err := openzipkin.NewEndpoint("frontend", "")
	if err != nil {
		log.Fatalf("unable to create local endpoint: %+v\n", err)
	}
	reporter := zipkinhttp.NewReporter(fmt.Sprintf("http://%s/api/v2/spans", *zipkinSvcAddr))
	exporter := zipkin.NewExporter(reporter, endpoint)
	trace.RegisterExporter(exporter)

	log.Info("zipkin initialization completed.")
}

func initPrometheusStats(log logrus.FieldLogger, r *mux.Router) {
	// init the prometheus /metrics endpoint
	exporter, err := prometheus.NewExporter(prometheus.Options{})
	if err != nil {
		log.Fatal(err)
	}

	// register basic views
	initStats(log, exporter)

	// init the prometheus /metrics endpoint
	r.Handle("/metrics", exporter).Methods(http.MethodGet, http.MethodHead)
	log.Info("prometheus metrics initialization completed.")
}

func initStats(log logrus.FieldLogger, exporter *prometheus.Exporter) {
	view.SetReportingPeriod(60 * time.Second)
	view.RegisterExporter(exporter)
	if err := view.Register(ochttp.DefaultServerViews...); err != nil {
		log.Warn("Error registering http default server views")
	} else {
		log.Info("Registered http default server views")
	}
	if err := view.Register(ocgrpc.DefaultClientViews...); err != nil {
		log.Warn("Error registering grpc default client views")
	} else {
		log.Info("Registered grpc default client views")
	}
}

func mustConnGRPC(ctx context.Context, conn **grpc.ClientConn, addr string) {
	var err error
	*conn, err = grpc.DialContext(ctx, addr,
		grpc.WithInsecure(),
		grpc.WithTimeout(time.Second*3),
		grpc.WithStatsHandler(&ocgrpc.ClientHandler{}))
	if err != nil {
		panic(errors.Wrapf(err, "grpc: failed to connect %s", addr))
	}
}
