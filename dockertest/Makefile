RANDOMSRC = Godeps/_workspace/src/github.com/jbenet/go-random/random
IPFS_DOCKER_IMAGE = zaqwsx_ipfs-test-img

test: clean setup
	fig build --no-cache
	fig up --no-color | tee build/fig.log
	make save_logs # save the ipfs logs for inspection
	# fig up won't report the error using an error code, so we grep the
	# fig.log file to find out whether the call succeeded
	tail build/fig.log | grep "exited with code 0"

setup: docker_ipfs_image data/filetiny data/filerand

save_logs:
	sh bin/save_logs.sh

docker_ipfs_image:
	docker images | grep $(IPFS_DOCKER_IMAGE)

data/filetiny: Makefile
	cp Makefile ./data/filetiny # simple

data/filerand: bin/random
	./bin/random 1000000 > ./data/filerand

bin/random:
	go build -o ./bin/random ../$(RANDOMSRC)

clean:
	sh bin/clean.sh
	fig stop
	fig rm -v --force
	rm -f bin/random
	rm -f data/filetiny
	rm -f data/filerand
	rm -rf build/*
