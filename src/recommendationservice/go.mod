module github.com/SimonEismann/microservices-demo/tree/master/src/recommendationservice

go 1.13

require (
	contrib.go.opencensus.io/exporter/jaeger v0.2.0
	contrib.go.opencensus.io/exporter/prometheus v0.1.0 // indirect
	contrib.go.opencensus.io/exporter/zipkin v0.1.1
	github.com/GoogleCloudPlatform/microservices-demo v0.2.0 // indirect
	github.com/golang/protobuf v1.3.2
	github.com/openzipkin/zipkin-go v0.2.2
	github.com/sirupsen/logrus v1.4.2
	go.opencensus.io v0.22.2
	golang.org/x/net v0.0.0-20200114155413-6afb5195e5aa
	google.golang.org/grpc v1.27.0
)
