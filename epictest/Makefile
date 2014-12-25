# This Makefile provides a way to really simple way to run benchmarks in a
# docker environment.

IMAGE = jbenet/go-ipfs-bench
CONTAINER = go-ipfs-bench
PACKAGE = epictest
PACKAGE_DIR = epictest
BUILD_DIR = ./build/bench
CONTAINER_WORKING_DIR = /go
CPU_PROF_NAME = cpu.out

all: collect

collect: clean build_image run_profiler cp_pprof_from_container

cp_pprof_from_container:
	docker cp $(CONTAINER):$(CONTAINER_WORKING_DIR)/$(CPU_PROF_NAME) $(BUILD_DIR)
	docker cp $(CONTAINER):$(CONTAINER_WORKING_DIR)/$(PACKAGE).test $(BUILD_DIR)

build_image:
	cd .. && docker build -t $(IMAGE) .

run_profiler:
	docker run --name $(CONTAINER) -it --entrypoint go $(IMAGE) test ./src/github.com/jbenet/go-ipfs/$(PACKAGE_DIR) --cpuprofile=$(CPU_PROF_NAME)


clean:
	docker rm $(CONTAINER) || true
	rm -rf $(BUILD_DIR)

analyze:
	go tool pprof $(BUILD_DIR)/$(PACKAGE).test $(BUILD_DIR)/$(CPU_PROF_NAME)
