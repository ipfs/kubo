FROM golang:1.4
MAINTAINER Brian Tiger Chow <btc@perfmode.com>

ADD . /go/src/github.com/jbenet/go-ipfs
RUN cd /go/src/github.com/jbenet/go-ipfs/cmd/ipfs && go install

RUN echo -n "#!/bin/bash\nipfs init\nipfs config Addresses.API /ip4/0.0.0.0/tcp/5001\nipfs config Addresses.Gateway /ip4/0.0.0.0/tcp/8080\nipfs daemon" > /usr/local/bin/start_ipfs && \
    chmod 755 /usr/local/bin/start_ipfs

EXPOSE 4001 5001 4002/udp 8080

ENTRYPOINT ["/usr/local/bin/start_ipfs"]

# build:    docker build -t go-ipfs .
# run:      docker run -p 4001:4001 -p 5001:5001 go-ipfs:latest daemon --init
# run:      docker run -p 4002:4002/udp -p 4001:4001 -p 5001:5001 go-ipfs:latest daemon --init
