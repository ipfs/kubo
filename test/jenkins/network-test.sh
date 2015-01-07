#!/bin/sh

export GOPATH=$WORKSPACE

PWD=`pwd`
cd ../3nodetest
make clean
make test
make save_logs

docker cp 3nodetest_server_1:/root/.go-ipfs/logs/events.log    $(PWD)/build/server-events.log
docker cp 3nodetest_bootstrap_1:/root/.go-ipfs/logs/events.log $(PWD)/build/bootstrap-events.log
docker cp 3nodetest_client_1:/root/.go-ipfs/logs/events.log    $(PWD)/build/client-events.log
