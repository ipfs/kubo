package tracing

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	version "github.com/ipfs/kubo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/jaeger"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	traceapi "go.opentelemetry.io/otel/trace"
)

// shutdownTracerProvider adds a shutdown method for tracer providers.
//
// Note that this doesn't directly use the provided TracerProvider interface
// to avoid build breaking go-ipfs if new methods are added to it.
type shutdownTracerProvider interface {
	Tracer(instrumentationName string, opts ...traceapi.TracerOption) traceapi.Tracer
	Shutdown(ctx context.Context) error
}

// noopShutdownTracerProvider adds a no-op Shutdown method to a TracerProvider.
type noopShutdownTracerProvider struct{ traceapi.TracerProvider }

func (n *noopShutdownTracerProvider) Shutdown(ctx context.Context) error { return nil }

func buildExporters(ctx context.Context) ([]trace.SpanExporter, error) {
	// These env vars are standardized but not yet supported by opentelemetry-go.
	// Once supported, we can remove most of this code.
	//
	// Specs:
	// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/sdk-environment-variables.md#exporter-selection
	// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/protocol/exporter.md
	var exporters []trace.SpanExporter
	for _, exporterStr := range strings.Split(os.Getenv("OTEL_TRACES_EXPORTER"), ",") {
		switch exporterStr {
		case "otlp":
			protocol := "http/protobuf"
			if v := os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL"); v != "" {
				protocol = v
			}
			if v := os.Getenv("OTEL_EXPORTER_OTLP_TRACES_PROTOCOL"); v != "" {
				protocol = v
			}

			switch protocol {
			case "http/protobuf":
				exporter, err := otlptracehttp.New(ctx)
				if err != nil {
					return nil, fmt.Errorf("building OTLP HTTP exporter: %w", err)
				}
				exporters = append(exporters, exporter)
			case "grpc":
				exporter, err := otlptracegrpc.New(ctx)
				if err != nil {
					return nil, fmt.Errorf("building OTLP gRPC exporter: %w", err)
				}
				exporters = append(exporters, exporter)
			default:
				return nil, fmt.Errorf("unknown or unsupported OTLP exporter '%s'", exporterStr)
			}
		case "jaeger":
			exporter, err := jaeger.New(jaeger.WithCollectorEndpoint())
			if err != nil {
				return nil, fmt.Errorf("building Jaeger exporter: %w", err)
			}
			exporters = append(exporters, exporter)
		case "zipkin":
			exporter, err := zipkin.New("")
			if err != nil {
				return nil, fmt.Errorf("building Zipkin exporter: %w", err)
			}
			exporters = append(exporters, exporter)
		case "file":
			// This is not part of the spec, but provided for convenience
			// so that you don't have to setup a collector,
			// and because we don't support the stdout exporter.
			filePath := os.Getenv("OTEL_EXPORTER_FILE_PATH")
			if filePath == "" {
				cwd, err := os.Getwd()
				if err != nil {
					return nil, fmt.Errorf("finding working directory for the OpenTelemetry file exporter: %w", err)
				}
				filePath = path.Join(cwd, "traces.json")
			}
			exporter, err := newFileExporter(filePath)
			if err != nil {
				return nil, err
			}
			exporters = append(exporters, exporter)
		case "none":
			continue
		case "":
			continue
		case "stdout":
			// stdout is already used for certain kinds of logging, so we don't support this
			fallthrough
		default:
			return nil, fmt.Errorf("unknown or unsupported exporter '%s'", exporterStr)
		}
	}
	return exporters, nil
}

// NewTracerProvider creates and configures a TracerProvider.
func NewTracerProvider(ctx context.Context) (shutdownTracerProvider, error) {
	exporters, err := buildExporters(ctx)
	if err != nil {
		return nil, err
	}
	if len(exporters) == 0 {
		return &noopShutdownTracerProvider{TracerProvider: traceapi.NewNoopTracerProvider()}, nil
	}

	options := []trace.TracerProviderOption{}

	for _, exporter := range exporters {
		options = append(options, trace.WithBatcher(exporter))
	}

	r, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			semconv.ServiceNameKey.String("go-ipfs"),
			semconv.ServiceVersionKey.String(version.CurrentVersionNumber),
		),
	)
	if err != nil {
		return nil, err
	}
	options = append(options, trace.WithResource(r))

	return trace.NewTracerProvider(options...), nil
}

// Span starts a new span using the standard IPFS tracing conventions.
func Span(ctx context.Context, componentName string, spanName string, opts ...traceapi.SpanStartOption) (context.Context, traceapi.Span) {
	return otel.Tracer("go-ipfs").Start(ctx, fmt.Sprintf("%s.%s", componentName, spanName), opts...)
}
