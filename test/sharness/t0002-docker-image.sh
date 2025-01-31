#!/usr/bin/env bash
#
# Copyright (c) 2015 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test docker image"

. lib/test-lib.sh

# if in travis CI on OSX, docker is not available
if ! test_have_prereq DOCKER; then
  skip_all='skipping docker tests, docker not available'

  test_done
fi

test_expect_success "'docker --version' works" '
  docker --version >actual
'

test_expect_success "'docker --version' output looks good" '
  egrep "^Docker version" actual
'

TEST_TRASH_DIR=$(pwd)
TEST_SCRIPTS_DIR=$(dirname "$TEST_TRASH_DIR")
TEST_TESTS_DIR=$(dirname "$TEST_SCRIPTS_DIR")
APP_ROOT_DIR=$(dirname "$TEST_TESTS_DIR")
IMAGE_TAG=kubo_test

test_expect_success "docker image build succeeds" '
  docker_build "$IMAGE_TAG" "$TEST_TESTS_DIR/../Dockerfile" "$APP_ROOT_DIR" ||
  test_fsh echo "TEST_TESTS_DIR: $TEST_TESTS_DIR" ||
  test_fsh echo "APP_ROOT_DIR : $APP_ROOT_DIR"
'

test_expect_success "write init scripts" '
  echo "ipfs config Provider.Strategy Bar" > 001.sh &&
  echo "ipfs config Pubsub.Router Qux" > 002.sh &&
  chmod +x 002.sh
'

test_expect_success "docker image runs" '
  DOC_ID=$(docker run -d \
                  -p 127.0.0.1:5001:5001 -p 127.0.0.1:8080:8080 \
                  -v "$PWD/001.sh":/container-init.d/001.sh \
                  -v "$PWD/002.sh":/container-init.d/002.sh \
                  "$IMAGE_TAG")
'

test_expect_success "docker container gateway is up" '
  pollEndpoint -host=/ip4/127.0.0.1/tcp/8080 -http-url http://localhost:8080/ipfs/bafkqaddimvwgy3zao5xxe3debi -v -tries 30 -tout 1s
'

test_expect_success "docker container API is up" '
  pollEndpoint -host=/ip4/127.0.0.1/tcp/5001 -http-url http://localhost:5001/version -v -tries 30 -tout 1s
'

test_expect_success "check that init scripts were run correctly and in the correct order" "
  echo -e \"Sourcing '/container-init.d/001.sh'...\nExecuting '/container-init.d/002.sh'...\" > expected &&
  docker logs $DOC_ID 2>/dev/null | grep -e 001.sh -e 002.sh > actual &&
  test_cmp actual expected
"

test_expect_success "check that init script configs were applied" '
  echo Bar > expected &&
  docker exec "$DOC_ID" ipfs config Provider.Strategy > actual &&
  test_cmp actual expected &&
  echo Qux > expected &&
  docker exec "$DOC_ID" ipfs config Pubsub.Router > actual &&
  test_cmp actual expected
'

test_expect_success "simple ipfs add/cat can be run in docker container" '
  echo "Hello Worlds" | tr -d "[:cntrl:]" > expected &&
  HASH=$(docker_exec "$DOC_ID" "echo $(cat expected) | ipfs add -q" | tr -d "[:cntrl:]") &&
  docker_exec "$DOC_ID" "ipfs cat $HASH" | tr -d "[:cntrl:]" > actual &&
  test_cmp expected actual
'

read testcode <<EOF
  pollEndpoint -host=/ip4/127.0.0.1/tcp/5001 -http-url http://localhost:5001/version -http-out | grep Commit | cut -d" " -f2 >actual ; \
  test -s actual ; \
  docker exec -i "$DOC_ID" ipfs version --enc json \
    | sed 's/^.*"Commit":"\\\([^"]*\\\)".*$/\\\1/g' >expected ; \
  test -s expected ; \
  test_cmp expected actual
EOF
test_expect_success "version CurrentCommit is set" "$testcode"

test_expect_success "stop docker container" '
  docker_stop "$DOC_ID"
'

docker_rm "$DOC_ID"
docker_rmi "$IMAGE_TAG"
test_done
