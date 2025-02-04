#!/usr/bin/env bash
#
# Copyright (c) 2015 Brian Holder-Chow Lin On
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test user-provided config values"

. lib/test-lib.sh

test_init_ipfs

test_expect_success "bootstrap doesn't overwrite user-provided config keys (top-level)" '
  ipfs config Provider.Strategy >previous &&
  ipfs config Provider.Strategy foo &&
  ipfs bootstrap rm --all &&
  echo "foo" >expected &&
  ipfs config Provider.Strategy >actual &&
  ipfs config Provider.Strategy $(cat previous) &&
  test_cmp expected actual
'

test_done
