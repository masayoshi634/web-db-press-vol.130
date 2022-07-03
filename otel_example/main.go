package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	"go.opentelemetry.io/otel/sdk/metric/export/aggregation"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	selector "go.opentelemetry.io/otel/sdk/metric/selector/simple"
)

var (
	Meter metric.Meter
)

func initProvider() error {
	config := prometheus.Config{}

	c := controller.New(
		processor.NewFactory(
			selector.NewWithInexpensiveDistribution(),
			aggregation.CumulativeTemporalitySelector(),
		),
	)
	exporter, err := prometheus.New(config, c)
	if err != nil {
		return fmt.Errorf("failed prmetheus exporter: %w", err)
	}

	global.SetMeterProvider(exporter.MeterProvider())

	http.HandleFunc("/", exporter.ServeHTTP)
	go func() {
		_ = http.ListenAndServe(":2222", nil)
	}()

	fmt.Println("Prometheus server running on :2222")
	return nil
}

func reqCounterMiddleware(next http.Handler) http.Handler {
	counter, _ := Meter.SyncInt64().Counter("http.req.counter")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		counter.Add(context.TODO(), 1, attribute.String("uri", r.RequestURI))
		next.ServeHTTP(w, r)
	})
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "hello")
}

func echoHandler(w http.ResponseWriter, r *http.Request) {
	io.Copy(w, r.Body)
}

func main() {
	err := initProvider()
	if err != nil {
		log.Fatal(err)
	}
	Meter = global.Meter("otel-demo")
	mux := http.NewServeMux()
	mux.Handle("/hello", reqCounterMiddleware(http.HandlerFunc(helloHandler)))
	mux.Handle("/echo", reqCounterMiddleware(http.HandlerFunc(echoHandler)))
	http.ListenAndServe(":8080", mux)
}
