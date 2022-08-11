#!/usr/bin/env bash

test_description="Test HTTP Gateway trace context propagation"

. lib/test-lib.sh

test_init_ipfs

export OTEL_TRACES_EXPORTER=otlp
export OTEL_EXPORTER_OTLP_PROTOCOL=grpc
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
export OTEL_TRACES_SAMPLER=always_on

cat <<EOF > collector-config.yaml
receivers:
  otlp:
    protocols:
      grpc:

processors:
  batch:

exporters:
  file:
    path: /traces/traces.json

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [file]
EOF

# touch traces.json and give it 777 perms, in case docker runs as a different user
rm -rf traces.json && touch traces.json && chmod 777 traces.json

test_expect_success "run opentelemetry collector" '
  docker run --rm -d -v "$PWD/collector-config.yaml":/config.yaml -v "$PWD":/traces --net=host --name=ipfs-test-otel-collector otel/opentelemetry-collector-contrib:0.52.0 --config /config.yaml
'

test_launch_ipfs_daemon

test_expect_success "Make a file to test with" '
  echo "Hello Worlds!" >expected &&
  HASH=$(ipfs add -q expected) ||
  test_fsh cat daemon_err
'

# HTTP GET Request
test_expect_success "GET to Gateway succeeds" '
  curl -svX GET "http://127.0.0.1:$GWAY_PORT/ipfs/$HASH" >/dev/null 2>curl_output &&
  cat curl_output
'

# GET Response from Gateway should contain no trace context headers
test_expect_success "GET response for request without traceparent contains no trace context headers" '
  grep "< traceparent: \*" curl_output && false || true
'

test_expect_success "Trace collector is writing traces" '
  until cat traces.json | grep "\"traceId\"" >/dev/null; do sleep 0.1; done
'

version="00"
trace_id="4bf92f3577b34da6a3ce929d0e0e4736"
parent_id="00f067aa0ba902b7"
flags="00"


# HTTP GET Request with traceparent
test_expect_success "GET to Gateway with traceparent header succeeds" '
  curl -H "traceparent: $version-$trace_id-$parent_id-$flags" -svX GET "http://127.0.0.1:$GWAY_PORT/ipfs/$HASH" >/dev/null 2>curl_output &&
  cat curl_output
'

test_expect_success "GET response for request with traceparent preserves trace id" '
  grep "< Traceparent: $version-$trace_id-" curl_output
'

test_expect_success "Trace id is used in traces" '
  until cat traces.json | grep "\"traceId\":\"$trace_id\"" >/dev/null; do sleep 0.1; done
'

test_kill_ipfs_daemon

test_expect_success "kill docker container" '
  docker kill ipfs-test-otel-collector
'

test_done
