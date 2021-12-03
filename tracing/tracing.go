package tracing

import (
	"context"
	"fmt"
	"os"
	"strconv"

	version "github.com/ipfs/go-ipfs"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	traceapi "go.opentelemetry.io/otel/trace"
)

var exporterBuilders = map[string]func(context.Context, string) (trace.SpanExporter, error){
	"IPFS_TRACING_JAEGER": func(ctx context.Context, s string) (trace.SpanExporter, error) {
		return jaeger.New(jaeger.WithCollectorEndpoint())
	},
	"IPFS_TRACING_FILE": func(ctx context.Context, s string) (trace.SpanExporter, error) {
		return newFileExporter(s)
	},
	"IPFS_TRACING_OTLP_HTTP": func(ctx context.Context, s string) (trace.SpanExporter, error) {
		return otlptracehttp.New(ctx)
	},
	"IPFS_TRACING_OTLP_GRPC": func(ctx context.Context, s string) (trace.SpanExporter, error) {
		return otlptracegrpc.New(ctx)
	},
}

// fileExporter wraps a file-writing exporter and closes the file when the exporter is shutdown.
type fileExporter struct {
	file           *os.File
	writerExporter *stdouttrace.Exporter
}

var _ trace.SpanExporter = &fileExporter{}

func newFileExporter(file string) (*fileExporter, error) {
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", file, err)
	}
	stdoutExporter, err := stdouttrace.New(stdouttrace.WithWriter(f))
	if err != nil {
		return nil, err
	}
	return &fileExporter{
		writerExporter: stdoutExporter,
		file:           f,
	}, nil
}

func (e *fileExporter) ExportSpans(ctx context.Context, spans []trace.ReadOnlySpan) error {
	return e.writerExporter.ExportSpans(ctx, spans)
}

func (e *fileExporter) Shutdown(ctx context.Context) error {
	if err := e.writerExporter.Shutdown(ctx); err != nil {
		return err
	}
	if err := e.file.Close(); err != nil {
		return fmt.Errorf("closing trace file: %w", err)
	}
	return nil
}

// noopShutdownTracerProvider wraps a TracerProvider with a no-op Shutdown method.
type noopShutdownTracerProvider struct {
	tp traceapi.TracerProvider
}

func (n *noopShutdownTracerProvider) Shutdown(ctx context.Context) error {
	return nil
}
func (n *noopShutdownTracerProvider) Tracer(instrumentationName string, opts ...traceapi.TracerOption) traceapi.Tracer {
	return n.tp.Tracer(instrumentationName, opts...)
}

type ShutdownTracerProvider interface {
	traceapi.TracerProvider
	Shutdown(ctx context.Context) error
}

// NewTracerProvider creates and configures a TracerProvider.
func NewTracerProvider(ctx context.Context) (ShutdownTracerProvider, error) {
	if os.Getenv("IPFS_TRACING") == "" {
		return &noopShutdownTracerProvider{tp: traceapi.NewNoopTracerProvider()}, nil
	}

	options := []trace.TracerProviderOption{}

	traceRatio := 1.0
	if envRatio := os.Getenv("IPFS_TRACING_RATIO"); envRatio != "" {
		r, err := strconv.ParseFloat(envRatio, 64)
		if err == nil {
			traceRatio = r
		}
	}
	options = append(options, trace.WithSampler(trace.ParentBased(trace.TraceIDRatioBased(traceRatio))))

	r, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("go-ipfs"),
			semconv.ServiceVersionKey.String(version.CurrentVersionNumber),
		),
	)
	if err != nil {
		return nil, err
	}
	options = append(options, trace.WithResource(r))

	for envVar, builder := range exporterBuilders {
		if val := os.Getenv(envVar); val != "" {
			exporter, err := builder(ctx, val)
			if err != nil {
				return nil, err
			}
			options = append(options, trace.WithBatcher(exporter))
		}
	}

	return trace.NewTracerProvider(options...), nil
}

// Span starts a new span using the standard IPFS tracing conventions.
func Span(ctx context.Context, componentName string, spanName string, opts ...traceapi.SpanStartOption) (context.Context, traceapi.Span) {
	return otel.Tracer("go-ipfs").Start(ctx, fmt.Sprintf("%s.%s", componentName, spanName), opts...)
}
