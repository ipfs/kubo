package tracing

import (
	"context"
	"fmt"

	"github.com/ipfs/boxo/tracing"
	version "github.com/ipfs/kubo"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	traceapi "go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

// shutdownTracerProvider adds a shutdown method for tracer providers.
//
// Note that this doesn't directly use the provided TracerProvider interface
// to avoid build breaking go-ipfs if new methods are added to it.
type shutdownTracerProvider interface {
	traceapi.TracerProvider

	Tracer(instrumentationName string, opts ...traceapi.TracerOption) traceapi.Tracer
	Shutdown(ctx context.Context) error
}

// noopShutdownTracerProvider adds a no-op Shutdown method to a TracerProvider.
type noopShutdownTracerProvider struct{ traceapi.TracerProvider }

func (n *noopShutdownTracerProvider) Shutdown(ctx context.Context) error { return nil }

// NewTracerProvider creates and configures a TracerProvider.
func NewTracerProvider(ctx context.Context) (shutdownTracerProvider, error) {
	exporters, err := tracing.NewSpanExporters(ctx)
	if err != nil {
		return nil, err
	}
	if len(exporters) == 0 {
		return &noopShutdownTracerProvider{TracerProvider: noop.NewTracerProvider()}, nil
	}

	options := []trace.TracerProviderOption{}

	for _, exporter := range exporters {
		options = append(options, trace.WithBatcher(exporter))
	}

	r, err := resource.Merge(
		resource.Default(),
		resource.NewSchemaless(
			semconv.ServiceNameKey.String("Kubo"),
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
	return otel.Tracer("Kubo").Start(ctx, fmt.Sprintf("%s.%s", componentName, spanName), opts...)
}
