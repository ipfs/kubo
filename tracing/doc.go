// Package tracing contains the tracing logic for go-ipfs, including configuring the tracer and
// helping keep consistent naming conventions across the stack.
//
// NOTE: Tracing is currently experimental. Span names may change unexpectedly, spans may be removed,
// and backwards-incompatible changes may be made to tracing configuration, options, and defaults.
//
// Tracing is configured through environment variables, as consistent with the OpenTelemetry spec as possible:
//
// https://github.com/open-telemetry/opentelemetry-specification/blob/main/specification/sdk-environment-variables.md
//
//  - OTEL_TRACES_EXPORTER: a comma-separated list of exporters
//    - otlp
//    - jaeger
//    - zipkin
//    - file
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
//  - OTEL_EXPORTER_OTLP_PROTOCOL
//    - one of [grpc, http/protobuf]
//    - default: grpc
//  - OTEL_EXPORTER_OTLP_ENDPOINT
//  - OTEL_EXPORTER_OTLP_CERTIFICATE
//  - OTEL_EXPORTER_OTLP_HEADERS
//  - OTEL_EXPORTER_OTLP_COMPRESSION
//  - OTEL_EXPORTER_OTLP_TIMEOUT
//
// Zipkin:
//
//  - OTEL_EXPORTER_ZIPKIN_ENDPOINT
//
// File:
//
//  - OTEL_EXPORTER_FILE_PATH
//    - file path to write JSON traces
//    - default: `$PWD/traces.json`
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
//    -p 14269:14269 \
//    -p 14250:14250 \
//    -p 9411:9411 \
//    jaegertracing/all-in-one
//  OTEL_TRACES_EXPORTER=jaeger ipfs daemon
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
