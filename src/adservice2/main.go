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
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"time"

	"github.com/gorilla/mux"
	"github.com/namsral/flag"
	"github.com/sirupsen/logrus"

	"go.opencensus.io/plugin/ocgrpc"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/plugin/ochttp/propagation/b3"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"

	"contrib.go.opencensus.io/exporter/jaeger"
	"contrib.go.opencensus.io/exporter/prometheus"
	"contrib.go.opencensus.io/exporter/zipkin"
	openzipkin "github.com/openzipkin/zipkin-go"
	zipkinhttp "github.com/openzipkin/zipkin-go/reporter/http"
)

var (
	version        = "no version set"
	displayVersion = flag.Bool("version", false, "Show version and quit")
	logLevel       = flag.String("logLevel", "debug", "log level from debug, info, warning, error. When debug, genetate 100% Tracing")
	srvURL         = flag.String("srvURL", ":9555", "IP and port to bind, localhost:9555 or :9555")
	adFile         = flag.String("adFile", "ads.json", "path to the Ads json file")

	jaegerSvcAddr    = flag.String("JAEGER_SERVICE_ADDR", "", "URL to Jaeger Tracing agent")
	zipkinSvcAddr    = flag.String("ZIPKIN_SERVICE_ADDR", "", "URL to Zipkin Tracing agent (ex: zipkin:9411)")
	extraLatency     = flag.Duration("EXTRA_LATENCY", 0*time.Second, "lattency to add to service response")
	startDelay       = flag.Duration("startDelay", 0*time.Second, "delay before service is available (return 503 failed probe)")
	bindDelay        = flag.Duration("bindDelay", 0*time.Second, "delay before binding the service port at startup")
	consecutiveError = flag.Int("consecutiveError", 0, "number of error 500 to return before answering the call")
)

func printVersion() {
	fmt.Printf("Go Version: %s", runtime.Version())
	fmt.Printf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("vuedashd Version: %v", version)
}

func main() {
	// parse flags
	flag.Parse()
	if *displayVersion {
		printVersion()
		os.Exit(0)
	}

	// setup logs
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

	// set injected latency
	if *extraLatency > 0 {
		log.Infof("extra latency enabled (duration: %v)", *extraLatency)
	}

	// set injected latency
	if *startDelay > 0 {
		log.Infof("start delay enabled (duration: %v)", *startDelay)
	}
	// ready time
	readyTime := time.Now().Add(*startDelay).Add(*bindDelay)

	a := &adserviceServer{
		adFile:      *adFile,
		adsIndex:    make(map[string][]int),
		failCounter: 0,
		failCount:   *consecutiveError,
	}

	err = a.loadAdsFile()
	if err != nil {
		log.Fatalf("error parsing Ads json file %s", err)
	}

	r := mux.NewRouter()
	r.HandleFunc("/ad", a.randomAdHandler).Methods(http.MethodGet, http.MethodHead)
	r.HandleFunc("/ads/{category}", a.categoryAdHandler).Methods(http.MethodGet, http.MethodHead)

	// healthz basic
	r.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		// return 503 if the start delay is not overdue
		if time.Until(readyTime) > 0 {
			http.Error(w, `{"status": "FAIL", "error": "service not ready"}`, 503)
			return
		}

		m := map[string]interface{}{"version": version, "status": "OK"}

		b, err := json.Marshal(m)
		if err != nil {
			http.Error(w, "Bad version set", 500)
			return
		}

		w.Write(b)
	})

	// also init the prometheus handler
	go initTracing(log)
	initPrometheusStats(log, r)

	var handler http.Handler = r
	handler = &ochttp.Handler{ // add opencensus instrumentation
		Handler:     handler,
		Propagation: &b3.HTTPFormat{},
	}

	srv := &http.Server{
		Handler: handler,
		Addr:    *srvURL,
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 360 * time.Second,
		ReadTimeout:  360 * time.Second,
	}

	go func() {
		if *bindDelay > 0 {
			log.Infof("bind delay enabled, waiting %v before opening %s )", *bindDelay, *srvURL)
			time.Sleep(*bindDelay)
		}

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
			ServiceName: "adservice",
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

	endpoint, err := openzipkin.NewEndpoint("adservice", "")
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
