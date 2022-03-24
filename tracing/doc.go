// Package tracing contains the tracing logic for go-ipfs, including configuring the tracer and
// helping keep consistent naming conventions across the stack.
//
// NOTE: Tracing is currently experimental. Span names may change unexpectedly, spans may be removed,
// and backwards-incompatible changes may be made to tracing configuration, options, and defaults.
//
// go-ipfs uses OpenTelemetry as its tracing API, and when possible, standard OpenTelemetry environment
// variables can be used to configure it. Multiple exporters can also be installed simultaneously,
// including one that writes traces to a JSON file on disk.
//
// In general, tracing is configured through environment variables. The IPFS-specific environment variables are:
//
//  - IPFS_TRACING: enable tracing in go-ipfs
//  - IPFS_TRACING_JAEGER: enable the Jaeger exporter
//  - IPFS_TRACING_RATIO: the ratio of traces to export, defaults to 1 (export everything)
//  - IPFS_TRACING_FILE: write traces to the given filename
//  - IPFS_TRACING_OTLP_HTTP: enable the OTLP HTTP exporter
//  - IPFS_TRACING_OTLP_GRPC: enable the OTLP gRPC exporter
//
// Different exporters have their own set of environment variables, depending on the exporter. These are typically
// standard environment variables. Some common ones:
//
// Jaeger:
//
//  - OTEL_EXPORTER_JAEGER_AGENT_HOST
//  - OTEL_EXPORTER_JAEGER_AGENT_PORT
//  - OTEL_EXPORTER_JAEGER_ENDPOINT
//  - OTEL_EXPORTER_JAEGER_USER
//  - OTEL_EXPORTER_JAEGER_PASSWORD
//
// OTLP HTTP/gRPC:
//
//  - OTEL_EXPORTER_OTLP_ENDPOINT
//  - OTEL_EXPORTER_OTLP_CERTIFICATE
//  - OTEL_EXPORTER_OTLP_HEADERS
//  - OTEL_EXPORTER_OTLP_COMPRESSION
//  - OTEL_EXPORTER_OTLP_TIMEOUT
//
// For example, if you run a local IPFS daemon, you can use the jaegertracing/all-in-one Docker image to run
// a full Jaeger stack and configure go-ipfs to publish traces to it:
//
//  docker run -d --name jaeger \
//    -e COLLECTOR_ZIPKIN_HOST_PORT=:9411 \
//    -p 5775:5775/udp \
//    -p 6831:6831/udp \
//    -p 6832:6832/udp \
//    -p 5778:5778 \
//    -p 16686:16686 \
//    -p 14268:14268 \
//    -p 14250:14250 \
//    -p 9411:9411 \
//    jaegertracing/all-in-one
//  IPFS_TRACING=1 IPFS_TRACING_JAEGER=1 ipfs daemon
//
//  In this example the Jaeger UI is available at http://localhost:16686.
//
//
// Implementer Notes
//
// Span names follow a convention of <Component>.<Span>, some examples:
//
//  - component=Gateway + span=Request -> Gateway.Request
//  - component=CoreAPI.PinAPI + span=Verify.CheckPin -> CoreAPI.PinAPI.Verify.CheckPin
//
// We follow the OpenTelemetry convention of using whatever TracerProvider is registered globally.
package tracing
