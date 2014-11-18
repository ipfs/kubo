FROM golang:1.3
MAINTAINER Brian Tiger Chow <btc@perfmode.com>

COPY . /go/src/github.com/jbenet/go-ipfs
RUN cd /go/src/github.com/jbenet/go-ipfs/cmd/ipfs && go install

EXPOSE 4001 5001

ENTRYPOINT ["ipfs"]

CMD ["daemon", "--init"]

# build:    docker build -t go-ipfs .
# run:      docker run -p 4001:4001 -p 5001:5001 go-ipfs:latest daemon --init
