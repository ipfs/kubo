#!/usr/bin/env bash
#
# Copyright (c) 2022 Protocol Labs
# MIT/Apache-2.0 Licensed; see the LICENSE file in this repository.
#

test_description="Test tracing"

. lib/test-lib.sh

test_init_ipfs

export OTEL_TRACES_EXPORTER=otlp
export OTEL_EXPORTER_OTLP_PROTOCOL=grpc
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317

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
  docker run --rm -d -v "$PWD/collector-config.yaml":/config.yaml -v "$PWD":/traces --net=host --name=ipfs-test-otel-collector otel/opentelemetry-collector-contrib:0.48.0 --config /config.yaml
'

test_launch_ipfs_daemon

test_expect_success "check that a swarm span eventually appears in exported traces" '
  until cat traces.json | grep CoreAPI.SwarmAPI >/dev/null; do sleep 0.1; done
'

test_expect_success "kill docker container" '
  docker kill ipfs-test-otel-collector
'

test_kill_ipfs_daemon

test_done
