FROM alpine:3.3
MAINTAINER Brian Tiger Chow <btc@perfmode.com>

ENV IPFS_PATH /data/ipfs
ENV GOPATH /go:/go/src/github.com/ipfs/go-ipfs/Godeps/_workspace

EXPOSE 4001 5001 8080
# 4001 = Swarm, 5001 = API, 8080 = HTTP transport

ADD bin/container_daemon /usr/local/bin/start_ipfs
ADD bin/container_shacheck /usr/local/bin/shacheck

ADD . /go/src/github.com/ipfs/go-ipfs
WORKDIR /go/src/github.com/ipfs/go-ipfs/cmd/ipfs

RUN adduser -D -h /data -u 1000 ipfs \
 && mkdir -p /data/ipfs && chown ipfs:ipfs /data/ipfs \
 && apk add --update bash ca-certificates git go \
 && go install -ldflags "-X github.com/ipfs/go-ipfs/repo/config.CurrentCommit=$(git rev-parse --short HEAD 2> /dev/null || echo unknown)" \
 && mv /go/bin/ipfs /usr/local/bin/ipfs \
 && chmod 755 /usr/local/bin/start_ipfs \
 && apk del --purge go git

WORKDIR /
RUN rm -rf /go/src/github.com/ipfs/go-ipfs

USER ipfs
VOLUME /data/ipfs

ENTRYPOINT ["/usr/local/bin/start_ipfs"]

# build:    docker build -t go-ipfs .
# run:      docker run -p 4001:4001 -p 5001:5001 go-ipfs:latest
# run:      docker run -p 8080:8080 -p 4001:4001 -p 5001:5001 go-ipfs:latest
