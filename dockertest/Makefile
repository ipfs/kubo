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

data/filetiny: Makefile
	cp Makefile ./data/filetiny # simple

data/filerand: bin/random
	./bin/random 50000000 > ./data/filerand

bin/random:
	go build -o ./bin/random ../$(RANDOMSRC)

docker_ipfs_image: build/.built_img
	docker images | grep $(IPFS_DOCKER_IMAGE)

# this is here so that "make clean; make" rebuilds the docker img.
# but otherwise will reuse whatever you last built.
# TODO: make this detect whether code has been changed
build/.built_img:
	cd .. && docker build -t $(IPFS_DOCKER_IMAGE) .
	touch build/.built_img

clean:
	sh bin/clean.sh
	fig stop
	fig rm -v --force
	rm -f bin/random
	rm -f data/filetiny
	rm -f data/filerand
	rm -rf build/*
	rm -rf build/.built_img
