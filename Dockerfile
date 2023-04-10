FROM --platform=${BUILDPLATFORM:-linux/amd64} golang:1.19.1-alpine

ARG TARGETPLATFORM
ARG BUILDPLATFORM
ARG TARGETOS
ARG TARGETARCH

RUN apk add make bash gcc musl-dev

ENV SRC_DIR /kubo

# Download packages first so they can be cached.
COPY go.mod go.sum $SRC_DIR/
RUN cd $SRC_DIR \
  && go mod download

COPY . $SRC_DIR

# Preload an in-tree but disabled-by-default plugin by adding it to the IPFS_PLUGINS variable
# e.g. docker build --build-arg IPFS_PLUGINS="foo bar baz"
ARG IPFS_PLUGINS

# Build the thing.
# Also: fix getting HEAD commit hash via git rev-parse.
RUN cd $SRC_DIR && \
	mkdir -p .git/objects && \
	GOOS=$TARGETOS \
	GOARCH=$TARGETARCH \
	GOFLAGS=-buildvcs=false \
	make build IPFS_PLUGINS=$IPFS_PLUGINS

# Now comes the actual target image, which aims to be as small as possible.
FROM --platform=${BUILDPLATFORM:-linux/amd64} alpine

RUN apk add --no-cache fuse ca-certificates su-exec tini

# Get the ipfs binary, entrypoint script, and TLS CAs from the build container.
ENV SRC_DIR /kubo
COPY --from=0 $SRC_DIR/cmd/ipfs/ipfs /usr/local/bin/ipfs
COPY --from=0 $SRC_DIR/bin/container_daemon /usr/local/bin/start_ipfs
COPY --from=0 $SRC_DIR/bin/container_init_run /usr/local/bin/container_init_run

# Fix permissions on start_ipfs (ignore the build machine's permissions)
RUN chmod 0755 /usr/local/bin/start_ipfs

# Swarm TCP; should be exposed to the public
EXPOSE 4001
# Swarm UDP; should be exposed to the public
EXPOSE 4001/udp
# Daemon API; must not be exposed publicly but to client services under you control
EXPOSE 5001
# Web Gateway; can be exposed publicly with a proxy, e.g. as https://ipfs.example.org
EXPOSE 8080
# Swarm Websockets; must be exposed publicly when the node is listening using the websocket transport (/ipX/.../tcp/8081/ws).
EXPOSE 8081

# Create the fs-repo directory and switch to a non-privileged user.
ENV IPFS_PATH /data/ipfs
RUN mkdir -p $IPFS_PATH \
  && adduser -D -h $IPFS_PATH -u 1000 -G users ipfs \
  && chown ipfs:users $IPFS_PATH

# Create mount points for `ipfs mount` command
RUN mkdir /ipfs /ipns \
  && chown ipfs:users /ipfs /ipns

# Create the init scripts directory
RUN mkdir /container-init.d \
  && chown ipfs:users /container-init.d

# Expose the fs-repo as a volume.
# start_ipfs initializes an fs-repo if none is mounted.
# Important this happens after the USER directive so permissions are correct.
VOLUME $IPFS_PATH

# The default logging level
ENV IPFS_LOGGING ""

# This just makes sure that:
# 1. There's an fs-repo, and initializes one if there isn't.
# 2. The API and Gateway are accessible from outside the container.
ENTRYPOINT ["/sbin/tini", "--", "/usr/local/bin/start_ipfs"]

# Healthcheck for the container
# QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn is the CID of empty folder
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ipfs dag stat /ipfs/QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn || exit 1

# Execute the daemon subcommand by default
CMD ["daemon", "--migrate=true", "--agent-version-suffix=docker"]
