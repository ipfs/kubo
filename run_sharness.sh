#!/bin/sh
set -xe

SRC_DIR=/go-ipfs
cd $SRC_DIR

make -O -j 2 coverage/sharness_tests.coverprofile test/sharness/test-results/sharness.xml TEST_GENERATE_JUNIT=1 CONTINUE_ON_S_FAILURE=1
