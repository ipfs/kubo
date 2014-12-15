FROM zaqwsx_ipfs-test-img

RUN ipfs init -b=1024
ADD . /tmp/id
RUN mv -f /tmp/id/config /root/.go-ipfs/config
RUN ipfs id

EXPOSE 4031 4032/udp

ENTRYPOINT ["/bin/bash"]
CMD ["/tmp/id/run.sh"]
