FROM zaqwsx_ipfs-test-img

RUN ipfs init -b=2048
ADD . /tmp/id
RUN mv -f /tmp/id/config /root/.ipfs/config
RUN ipfs id

ENV IPFS_PROF true
ENV GOLOG_LOG_FMT nocolor

EXPOSE 4011 4012/udp
