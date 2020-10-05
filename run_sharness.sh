#!/bin/sh

set -xe

SRC_DIR=/go-ipfs
cd $SRC_DIR

make -O -j 10 coverage/sharness_tests.coverprofile test/sharness/test-results/sharness.xml TEST_GENERATE_JUNIT=1 CONTINUE_ON_S_FAILURE=1

bash <(curl -s https://codecov.io/bash) -cF sharness -X search -f coverage/sharness_tests.coverprofile

mv test/sharness/test-results/sharness.xml /tmp/circleci-test-results/sharness

find test/sharness/test-results -name 't*-*.sh.*.counts' | test/sharness/lib/sharness/aggregate-results.sh | grep 'failed\s*0'
