STRIP="perl -pe 's/\e\[?.*?[\@-~]//g'"

# TODO use a for loop like a grownup
docker logs dockertest_bootstrap_1 2>&1 | eval $STRIP > ./build/bootstrap.log 
docker logs dockertest_client_1 2>&1 | eval $STRIP > ./build/client.log 
docker logs dockertest_data_1 2>&1 | eval $STRIP > ./build/data.log 
docker logs dockertest_server_1 2>&1 | eval $STRIP > ./build/server.log
