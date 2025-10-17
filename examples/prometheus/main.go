// Prometheus Metrics Example
//
// This example demonstrates how to export Snowflake generator metrics to Prometheus
// for production monitoring and alerting.
//
// Features demonstrated:
// - Prometheus metrics export
// - HTTP metrics endpoint
// - Real-time metric updates
// - Grafana dashboard compatible
//
// Usage:
//   go run main.go
//   curl http://localhost:8080/metrics
//
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/sxyafiq/snowflake"
)

// MetricsExporter wraps a Snowflake generator and exports metrics to Prometheus
type MetricsExporter struct {
	gen      *snowflake.Generator
	workerID int64
}

// NewMetricsExporter creates a new metrics exporter
func NewMetricsExporter(workerID int64) (*MetricsExporter, error) {
	// Create generator with metrics enabled
	cfg := snowflake.DefaultConfig(workerID)
	cfg.EnableMetrics = true

	gen, err := snowflake.NewWithConfig(cfg)
	if err != nil {
		return nil, err
	}

	return &MetricsExporter{
		gen:      gen,
		workerID: workerID,
	}, nil
}

// GenerateID generates an ID and updates metrics
func (m *MetricsExporter) GenerateID(ctx context.Context) (snowflake.ID, error) {
	return m.gen.GenerateIDWithContext(ctx)
}

// PrometheusHandler returns an HTTP handler that exposes metrics in Prometheus format
func (m *MetricsExporter) PrometheusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get current metrics
		metrics := m.gen.GetMetrics()

		// Write metrics in Prometheus format
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")

		// Snowflake ID generation metrics
		fmt.Fprintf(w, "# HELP snowflake_ids_generated_total Total number of IDs generated\n")
		fmt.Fprintf(w, "# TYPE snowflake_ids_generated_total counter\n")
		fmt.Fprintf(w, "snowflake_ids_generated_total{worker=\"%d\"} %d\n", m.workerID, metrics.Generated)

		// Clock backward events
		fmt.Fprintf(w, "# HELP snowflake_clock_backward_total Number of times clock moved backward (recovered)\n")
		fmt.Fprintf(w, "# TYPE snowflake_clock_backward_total counter\n")
		fmt.Fprintf(w, "snowflake_clock_backward_total{worker=\"%d\"} %d\n", m.workerID, metrics.ClockBackward)

		// Clock backward errors (unrecoverable)
		fmt.Fprintf(w, "# HELP snowflake_clock_backward_errors_total Number of unrecoverable clock backward errors\n")
		fmt.Fprintf(w, "# TYPE snowflake_clock_backward_errors_total counter\n")
		fmt.Fprintf(w, "snowflake_clock_backward_errors_total{worker=\"%d\"} %d\n", m.workerID, metrics.ClockBackwardErr)

		// Sequence overflows
		fmt.Fprintf(w, "# HELP snowflake_sequence_overflow_total Number of sequence overflows (had to wait for next ms)\n")
		fmt.Fprintf(w, "# TYPE snowflake_sequence_overflow_total counter\n")
		fmt.Fprintf(w, "snowflake_sequence_overflow_total{worker=\"%d\"} %d\n", m.workerID, metrics.SequenceOverflow)

		// Wait time microseconds
		fmt.Fprintf(w, "# HELP snowflake_wait_time_microseconds_total Total time spent waiting in microseconds\n")
		fmt.Fprintf(w, "# TYPE snowflake_wait_time_microseconds_total counter\n")
		fmt.Fprintf(w, "snowflake_wait_time_microseconds_total{worker=\"%d\"} %d\n", m.workerID, metrics.WaitTimeUs)

		// Generator info (static labels)
		fmt.Fprintf(w, "# HELP snowflake_generator_info Generator information\n")
		fmt.Fprintf(w, "# TYPE snowflake_generator_info gauge\n")
		fmt.Fprintf(w, "snowflake_generator_info{worker=\"%d\"} 1\n", m.workerID)

		// Derived metrics (rates calculated by Prometheus)
		// These are just for documentation, Prometheus will calculate them
		if metrics.Generated > 0 {
			avgWaitUs := float64(metrics.WaitTimeUs) / float64(metrics.Generated)
			fmt.Fprintf(w, "# HELP snowflake_avg_wait_microseconds Average wait time per ID in microseconds\n")
			fmt.Fprintf(w, "# TYPE snowflake_avg_wait_microseconds gauge\n")
			fmt.Fprintf(w, "snowflake_avg_wait_microseconds{worker=\"%d\"} %.2f\n", m.workerID, avgWaitUs)
		}
	}
}

// HealthHandler returns a health check handler
func (m *MetricsExporter) HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		metrics := m.gen.GetMetrics()

		// Check for critical issues
		if metrics.ClockBackwardErr > 10 {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "UNHEALTHY: Too many clock backward errors: %d\n", metrics.ClockBackwardErr)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "OK: Generated %d IDs\n", metrics.Generated)
	}
}

func main() {
	// Get worker ID from environment or use default
	workerID := int64(0)
	if envWorkerID := os.Getenv("WORKER_ID"); envWorkerID != "" {
		if id, err := strconv.ParseInt(envWorkerID, 10, 64); err == nil {
			workerID = id
		}
	}

	// Create metrics exporter
	exporter, err := NewMetricsExporter(workerID)
	if err != nil {
		log.Fatalf("Failed to create exporter: %v", err)
	}

	// Setup HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", exporter.PrometheusHandler())
	mux.HandleFunc("/health", exporter.HealthHandler())
	mux.HandleFunc("/generate", func(w http.ResponseWriter, r *http.Request) {
		// Example endpoint that generates IDs
		ctx := r.Context()
		id, err := exporter.GenerateID(ctx)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error: %v", err), http.StatusInternalServerError)
			return
		}

		// Return ID in multiple formats
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"id": "%d", "base62": "%s", "timestamp": "%s"}`,
			id.Int64(), id.Base62(), id.Time().Format(time.RFC3339))
	})

	// Root handler with instructions
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `Snowflake Prometheus Metrics Exporter

Endpoints:
  /metrics    - Prometheus metrics endpoint
  /health     - Health check endpoint
  /generate   - Generate a new ID (example usage)

Example queries:
  # View all metrics
  curl http://localhost:8080/metrics

  # Generate an ID
  curl http://localhost:8080/generate

  # Check health
  curl http://localhost:8080/health

Prometheus scrape config:
  scrape_configs:
    - job_name: 'snowflake'
      static_configs:
        - targets: ['localhost:8080']

PromQL Queries:
  # Generation rate (IDs per second)
  rate(snowflake_ids_generated_total[1m])

  # Total IDs generated across all workers
  sum(snowflake_ids_generated_total)

  # Clock backward error rate
  rate(snowflake_clock_backward_errors_total[5m])

  # Alert on clock issues
  snowflake_clock_backward_errors_total > 10

Worker ID: %d
`, workerID)
	})

	// Start background ID generation to demonstrate metrics
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for range ticker.C {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			_, err := exporter.GenerateID(ctx)
			cancel()
			if err != nil {
				log.Printf("Error generating ID: %v", err)
			}
		}
	}()

	// Start HTTP server
	port := "8080"
	if envPort := os.Getenv("PORT"); envPort != "" {
		port = envPort
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: mux,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("Shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
	}()

	log.Printf("Starting Snowflake metrics exporter on :%s", port)
	log.Printf("Worker ID: %d", workerID)
	log.Printf("Metrics endpoint: http://localhost:%s/metrics", port)
	log.Printf("Health endpoint: http://localhost:%s/health", port)

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}
