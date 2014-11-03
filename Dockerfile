FROM golang:1.3
MAINTAINER Brian Tiger Chow <btc@perfmode.com>

COPY . /go/src/github.com/jbenet/go-ipfs
RUN cd /go/src/github.com/jbenet/go-ipfs/cmd/ipfs && go install

EXPOSE 4001

CMD ["ipfs", "run"]

# build:    docker build -t go-ipfs .
# run:      docker run -p 4001:4001 -e "IPFS_LOGGING=debug" go-ipfs:latest
