package tracing

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/sdk/trace"
)

// fileExporter wraps a file-writing exporter and closes the file when the exporter is shutdown.
type fileExporter struct {
	file           *os.File
	writerExporter *stdouttrace.Exporter
}

func newFileExporter(file string) (*fileExporter, error) {
	f, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, fmt.Errorf("opening '%s' for OpenTelemetry file exporter: %w", file, err)
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
