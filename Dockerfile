FROM golang:1.3
MAINTAINER Brian Tiger Chow <btc@perfmode.com>

RUN apt-get update
RUN apt-get install -y fuse

COPY . /go/src/github.com/jbenet/go-ipfs

RUN cd /go/src/github.com/jbenet/go-ipfs/cmd/ipfs && go install
RUN ipfs init
RUN ipfs config Identity.Address "/ip4/0.0.0.0/tcp/4001"
RUN mkdir /ipfs

EXPOSE 4001

CMD ["ipfs", "mount", "/ipfs"]

# build:    docker build -t go-ipfs .
# run:      docker run --privileged=true -i -t go-ipfs:latest
