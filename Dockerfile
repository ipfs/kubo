FROM alpine:3.2
MAINTAINER Brian Tiger Chow <btc@perfmode.com>

ENV IPFS_PATH /data/ipfs
ENV GOPATH /gobuild
ENV GOPKG github.com/ipfs/go-ipfs

EXPOSE 4001 5001 8080
# 4001 = Swarm, 5001 = API, 8080 = HTTP transport

VOLUME /data/ipfs

ADD . $GOPATH/src/$GOPKG

RUN adduser -D -h /data -u 1000 ipfs \
 && mkdir -p /data/ipfs && chown ipfs:ipfs /data/ipfs \
 && apk add --update go ca-certificates bash \
 && cd $GOPATH/src/$GOPKG/cmd/ipfs \
 && go get -v ./... \
 && go install -v \
 && mv $GOPATH/bin/ipfs /usr/local/bin/ipfs \
 && cp ../../bin/container_daemon /usr/local/bin/start_ipfs \
 && chmod 755 /usr/local/bin/start_ipfs \
 && apk del go && rm -rf /var/cache/apk/* $GOPATH

USER ipfs

ENTRYPOINT ["/usr/local/bin/start_ipfs"]

# build:    docker build -t go-ipfs .
# run:      docker run -p 4001:4001 -p 5001:5001 go-ipfs:latest
# run:      docker run -p 8080:8080 -p 4001:4001 -p 5001:5001 go-ipfs:latest
