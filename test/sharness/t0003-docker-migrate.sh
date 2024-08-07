#!/usr/bin/env bash
#
# Copyright (c) 2017 Whyrusleeping
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test docker image migration"

. lib/test-lib.sh

# if in travis CI on OSX, docker is not available
if ! test_have_prereq DOCKER; then
  skip_all='skipping '$test_description', docker not available'

  test_done
fi

if ! test_have_prereq SOCAT; then
  skip_all="skipping '$test_description': socat is not available"
  test_done
fi

TEST_TRASH_DIR=$(pwd)
TEST_SCRIPTS_DIR=$(dirname "$TEST_TRASH_DIR")
TEST_TESTS_DIR=$(dirname "$TEST_SCRIPTS_DIR")
APP_ROOT_DIR=$(dirname "$TEST_TESTS_DIR")
IMAGE_TAG=kubo_migrate

test_expect_success "docker image build succeeds" '
  docker_build "$IMAGE_TAG" "$TEST_TESTS_DIR/../Dockerfile" "$APP_ROOT_DIR"
'

test_init_ipfs

test_expect_success "configure migration sources" '
  ipfs config --json Migration.DownloadSources "[\"http://127.0.0.1:17233\"]"
'

test_expect_success "setup http response" '
  mkdir migration &&
  echo "v1.1.1" > migration/versions &&
  mkdir -p migration/fs-repo-6-to-7 &&
  echo "v1.1.1" > migration/fs-repo-6-to-7/versions &&
  CID=$(ipfs add -r -Q migration) &&
  echo "HTTP/1.1 200 OK" > vers_resp &&
  echo "Content-Type: application/vnd.ipld.car" >> vers_resp &&
  echo "" >> vers_resp &&
  ipfs dag export $CID >> vers_resp
'

test_expect_success "make repo be version 4" '
  echo 4 > "$IPFS_PATH/version"
'

test_expect_success "startup fake dists server" '
  ( socat tcp-listen:17233,fork,bind=127.0.0.1,reuseaddr "SYSTEM:cat vers_resp"!!STDERR 2> dist_serv_out ) &
  echo $! > netcat_pid
'

test_expect_success "docker image runs" '
  DOC_ID=$(docker run -d -v "$IPFS_PATH":/data/ipfs -e IPFS_DIST_PATH=/ipfs/$CID --net=host "$IMAGE_TAG")
'

test_expect_success "docker container tries to pull migrations from netcat" '
  sleep 4 &&
  cat dist_serv_out
'

test_expect_success "see logs" '
  docker logs $DOC_ID
'

test_expect_success "stop docker container" '
  docker_stop "$DOC_ID"
'

test_expect_success "kill the net cat" '
  kill $(cat netcat_pid) || true
'

test_expect_success "correct version was requested" '
  grep "/fs-repo-6-to-7/v1.1.1/fs-repo-6-to-7_v1.1.1_linux-amd64.tar.gz" dist_serv_out > /dev/null
'

docker_rm "$DOC_ID"
docker_rmi "$IMAGE_TAG"
test_done
