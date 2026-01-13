# syntax=docker/dockerfile:1
# Enables BuildKit with cache mounts for faster builds
FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.25 AS builder

ARG TARGETOS TARGETARCH

ENV SRC_DIR=/kubo

# Cache go module downloads between builds for faster rebuilds
COPY go.mod go.sum $SRC_DIR/
WORKDIR $SRC_DIR
RUN --mount=type=cache,target=/go/pkg/mod \
  go mod download

COPY . $SRC_DIR

# Preload an in-tree but disabled-by-default plugin by adding it to the IPFS_PLUGINS variable
# e.g. docker build --build-arg IPFS_PLUGINS="foo bar baz"
ARG IPFS_PLUGINS

# Allow for other targets to be built, e.g.: docker build --build-arg MAKE_TARGET="nofuse"
ARG MAKE_TARGET=build

# Build ipfs binary with cached go modules and build cache.
# mkdir .git/objects allows git rev-parse to read commit hash for version info
RUN --mount=type=cache,target=/go/pkg/mod \
  --mount=type=cache,target=/root/.cache/go-build \
  mkdir -p .git/objects \
  && GOOS=$TARGETOS GOARCH=$TARGETARCH GOFLAGS=-buildvcs=false make ${MAKE_TARGET} IPFS_PLUGINS=$IPFS_PLUGINS

# Extract required runtime tools from Debian.
# We use Debian instead of Alpine because we need glibc compatibility
# for the busybox base image we're using.
FROM debian:bookworm-slim AS utilities
RUN set -eux; \
	apt-get update; \
	apt-get install -y --no-install-recommends \
		tini \
    # Using gosu (~2MB) instead of su-exec (~20KB) because it's easier to
    # install on Debian. Useful links:
    # - https://github.com/ncopa/su-exec#why-reinvent-gosu
    # - https://github.com/tianon/gosu/issues/52#issuecomment-441946745
		gosu \
    # fusermount enables IPFS mount commands
    fuse \
    ca-certificates \
	; \
	apt-get clean; \
	rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# Final minimal image with shell for debugging (busybox provides sh)
FROM busybox:stable-glibc

# Copy ipfs binary, startup scripts, and runtime dependencies
ENV SRC_DIR=/kubo
COPY --from=utilities /usr/sbin/gosu /sbin/gosu
COPY --from=utilities /usr/bin/tini /sbin/tini
COPY --from=utilities /bin/fusermount /usr/local/bin/fusermount
COPY --from=utilities /etc/ssl/certs /etc/ssl/certs
COPY --from=builder $SRC_DIR/cmd/ipfs/ipfs /usr/local/bin/ipfs
COPY --from=builder --chmod=755 $SRC_DIR/bin/container_daemon /usr/local/bin/start_ipfs
COPY --from=builder $SRC_DIR/bin/container_init_run /usr/local/bin/container_init_run

# Set SUID for fusermount to enable FUSE mounting by non-root user
RUN chmod 4755 /usr/local/bin/fusermount

# Swarm P2P port (TCP/UDP) - expose publicly for peer connections
EXPOSE 4001 4001/udp
# API port - keep private, only for trusted clients
EXPOSE 5001
# Gateway port - can be exposed publicly via reverse proxy
EXPOSE 8080
# Swarm WebSockets - expose publicly for browser-based peers
EXPOSE 8081

# Create ipfs user (uid 1000) and required directories with proper ownership
ENV IPFS_PATH=/data/ipfs
RUN mkdir -p $IPFS_PATH /ipfs /ipns /mfs /container-init.d \
  && adduser -D -h $IPFS_PATH -u 1000 -G users ipfs \
  && chown ipfs:users $IPFS_PATH /ipfs /ipns /mfs /container-init.d

# Volume for IPFS repository data persistence
VOLUME $IPFS_PATH

# The default logging level
ENV GOLOG_LOG_LEVEL=""

# Entrypoint initializes IPFS repo if needed and configures networking.
# tini ensures proper signal handling and zombie process cleanup
ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/start_ipfs"]

# Health check verifies IPFS daemon is responsive.
# Uses empty directory CID (QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn) as test
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ipfs --api=/ip4/127.0.0.1/tcp/5001 dag stat /ipfs/QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn || exit 1

# Default: run IPFS daemon with auto-migration enabled
CMD ["daemon", "--migrate=true", "--agent-version-suffix=docker"]
