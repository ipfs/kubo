FROM alpine:3.3
MAINTAINER Brian Tiger Chow <btc@perfmode.com>

ENV IPFS_PATH /data/ipfs
ENV VERSION master

EXPOSE 4001 5001 8080
# 4001 = Swarm, 5001 = API, 8080 = HTTP transport

ADD bin/container_daemon /usr/local/bin/start_ipfs
ADD bin/container_shacheck /usr/local/bin/shacheck

ADD https://gobuilder.me/get/github.com/ipfs/go-ipfs/cmd/ipfs/ipfs_${VERSION}_linux-amd64 /usr/local/bin/ipfs_${VERSION}_linux-amd64

RUN adduser -D -h /data -u 1000 ipfs \
 && mkdir -p /data/ipfs && chown ipfs:ipfs /data/ipfs \
 && apk add --update bash curl ca-certificates \
 && cd /usr/local/bin \
 && /bin/bash /usr/local/bin/shacheck ${VERSION} ipfs_${VERSION}_linux-amd64 \
 && ln -sf ipfs_${VERSION}_linux-amd64 ipfs \
 && chmod 755 /usr/local/bin/start_ipfs \
 && chmod 755 /usr/local/bin/ipfs_${VERSION}_linux-amd64 \
 && apk del --purge curl

USER ipfs

VOLUME /data/ipfs
ENTRYPOINT ["/usr/local/bin/start_ipfs"]

# build:    docker build -t go-ipfs .
# run:      docker run -p 4001:4001 -p 5001:5001 go-ipfs:latest
# run:      docker run -p 8080:8080 -p 4001:4001 -p 5001:5001 go-ipfs:latest
