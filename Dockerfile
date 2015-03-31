FROM golang:1.4
MAINTAINER Brian Tiger Chow <btc@perfmode.com>

ADD . /go/src/github.com/ipfs/go-ipfs
RUN cd /go/src/github.com/ipfs/go-ipfs/cmd/ipfs && go install

RUN cp /go/src/github.com/ipfs/go-ipfs/bin/container_daemon /usr/local/bin/start_ipfs && \
    chmod 755 /usr/local/bin/start_ipfs

EXPOSE 4001 5001 8080
# 4001 = Swarm, 5001 = API, 8080 = HTTP transport

ENTRYPOINT ["/usr/local/bin/start_ipfs"]

# build:    docker build -t go-ipfs .
# run:      docker run -p 4001:4001 -p 5001:5001 go-ipfs:latest
# run:      docker run -p 8080:8080 -p 4001:4001 -p 5001:5001 go-ipfs:latest
