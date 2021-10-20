#!/usr/bin/env bash

test_description="Test the glibc version"

. lib/test-lib.sh

test_expect_success "expect glibc version is not too high" '
  echo "glibc versions:" &&
  glibc-check list-versions $(which ipfs) &&
  glibc-check assert-all "major == 2 && minor < 32" $(which ipfs)
'

test_done
 
