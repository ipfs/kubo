FROM alpine:3.3
MAINTAINER Lars Gierth <lgierth@ipfs.io>

# This is a copy of the root Dockerfile,
# except that we optimize for build time, instead of image size.
#
# Please keep these two Dockerfiles in sync.
#
# Only sections different from the root Dockerfile are commented.


EXPOSE 4001
EXPOSE 4002/udp
EXPOSE 5001
EXPOSE 8080

ENV GX_IPFS   ""
ENV IPFS_PATH /data/ipfs
ENV IPFS_LOGGING ""
ENV GO_VERSION 1.5.4-r0
ENV GOPATH     /go
ENV PATH       /go/bin:$PATH
ENV SRC_PATH   /go/src/github.com/ipfs/go-ipfs

# This is an optimization which avoids rebuilding
# of the gx dependencies every time anything changes.
# gx will only be invoked if the dependencies have changed.
#
# Put differently: if package.json has changed,
# the image-id after this COPY command will change,
# and trigger a re-run of all following commands.
COPY ./package.json $SRC_PATH/package.json

RUN apk add --update musl go=$GO_VERSION git bash wget ca-certificates \
	&& mkdir -p $IPFS_PATH \
	&& adduser -D -h $IPFS_PATH -u 1000 ipfs \
	&& chown ipfs:ipfs $IPFS_PATH && chmod 755 $IPFS_PATH \
	&& go get -u github.com/whyrusleeping/gx \
	&& go get -u github.com/whyrusleeping/gx-go \
	&& ([ -z "$GX_IPFS" ] || echo $GX_IPFS > $IPFS_PATH/api) \
	&& cd $SRC_PATH \
	&& gx --verbose install --global

COPY . $SRC_PATH

RUN cd $SRC_PATH \
	&& ref="$(cat .git/HEAD | cut -d' ' -f2)" \
	&& commit="$(cat .git/$ref | head -c 7)" \
	&& echo "ldflags=-X github.com/ipfs/go-ipfs/repo/config.CurrentCommit=$commit" \
	&& cd $SRC_PATH/cmd/ipfs \
	&& go build -ldflags "-X github.com/ipfs/go-ipfs/repo/config.CurrentCommit=$commit" \
	&& cp ipfs /usr/local/bin/ipfs \
	&& cp $SRC_PATH/bin/container_daemon /usr/local/bin/start_ipfs \
	&& chmod 755 /usr/local/bin/start_ipfs \
	&& apk del --purge musl go git && rm -rf $GOPATH && rm -vf $IPFS_PATH/api

USER ipfs
VOLUME $IPFS_PATH
ENTRYPOINT ["/usr/local/bin/start_ipfs"]
