#!/usr/bin/env bash
#
# Copyright (c) 2020 Protocol Labs
# MIT/Apache-2.0 Licensed; see the LICENSE file in this repository.
#

test_description="Test prometheus metrics are exposed correctly"

. lib/test-lib.sh

test_init_ipfs

test_launch_ipfs_daemon

test_expect_success "collect metrics" '
  curl "$API_ADDR/debug/metrics/prometheus" > raw_metrics
'

test_kill_ipfs_daemon

test_expect_success "filter metrics" '
  sed -ne "s/^\([a-z0-9_]\+\).*/\1/p" raw_metrics | LC_ALL=C sort > filtered_metrics
'

test_expect_success "make sure metrics haven't changed" '
  diff -u ../t0116-prometheus-data/prometheus_metrics filtered_metrics
'

test_done
