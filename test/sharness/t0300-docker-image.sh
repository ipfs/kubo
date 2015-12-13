#!/bin/sh
#
# Copyright (c) 2015 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test docker image"

. lib/test-lib.sh

test_expect_success "docker is installed" '
	type docker
'

test_expect_success "'docker --version' works" '
	docker --version >actual
'

test_expect_success "'docker --version' output looks good" '
	egrep "^Docker version" actual
'

test_expect_success "current user is in the 'docker' group" '
	groups | egrep "\bdocker\b"
'

TEST_TRASH_DIR=$(pwd)
TEST_SCRIPTS_DIR=$(dirname "$TEST_TRASH_DIR")
TEST_TESTS_DIR=$(dirname "$TEST_SCRIPTS_DIR")
APP_ROOT_DIR=$(dirname "$TEST_TESTS_DIR")

test_expect_success "docker image build succeeds" '
	docker_build "$APP_ROOT_DIR" >actual
'

test_expect_success "docker image build output looks good" '
	SUCCESS_LINE=$(egrep "^Successfully built" actual) &&
	IMAGE_ID=$(expr "$SUCCESS_LINE" : "^Successfully built \(.*\)") ||
	test_fsh cat actual
'

test_expect_success "docker image runs" '
	DOC_ID=$(docker_run "$IMAGE_ID")
'

test_expect_success "simple command can be run in docker container" '
	docker_exec "$DOC_ID" "echo Hello Worlds" >actual
'

test_expect_success "simple command output looks good" '
	echo "Hello Worlds" >expected &&
	test_cmp expected actual
'

test_expect_success "stop docker container" '
	docker_stop "$DOC_ID"
'

test_done

