RANDOMSRC = Godeps/_workspace/src/github.com/jbenet/go-random/random

test: clean setup
	fig build --no-cache
	fig up

setup: docker_ipfs_image data/file

docker_ipfs_image:
	docker images | grep zaqwsx_ipfs-test-img

data/file: bin/random
	cp Makefile ./data/file

bin/random: 
	go build -o ./bin/random ../$(RANDOMSRC)

clean:
	docker rm $(docker ps -q -a -f status=exited) || true
	rm -f data/file
