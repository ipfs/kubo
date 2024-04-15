#!/usr/bin/env bash
#
# Copyright (c) 2020 Protocol Labs
# MIT/Apache-2.0 Licensed; see the LICENSE file in this repository.
#

test_description="Test prometheus metrics are exposed correctly"

. lib/test-lib.sh

test_init_ipfs

test_expect_success "enable ResourceMgr in the config" '
  ipfs config --json Swarm.ResourceMgr.Enabled false
'

test_launch_ipfs_daemon

test_expect_success "collect metrics" '
  curl "$API_ADDR/debug/metrics/prometheus" > raw_metrics
'

test_kill_ipfs_daemon

test_expect_success "filter metrics" '
  sed -ne "s/^\([a-z0-9_]\+\).*/\1/p" raw_metrics | LC_ALL=C sort | uniq > filtered_metrics
'

test_expect_success "make sure metrics haven't changed" '
  diff -u ../t0119-prometheus-data/prometheus_metrics filtered_metrics
'

# Check what was added by enabling ResourceMgr.Enabled
#
# NOTE: we won't see all the dynamic ones, but that is  ok: the point of the
# test here is to detect regression when rcmgr metrics disappear due to
# refactor/human error.

test_expect_success "enable ResourceMgr in the config" '
  ipfs config --json Swarm.ResourceMgr.Enabled true
'

test_launch_ipfs_daemon

test_expect_success "collect metrics" '
  curl "$API_ADDR/debug/metrics/prometheus" > raw_metrics
'

test_kill_ipfs_daemon

test_expect_success "filter metrics and find ones added by enabling ResourceMgr" '
  sed -ne "s/^\([a-z0-9_]\+\).*/\1/p" raw_metrics | LC_ALL=C sort > filtered_metrics &&
  grep -v -x -f ../t0119-prometheus-data/prometheus_metrics filtered_metrics | LC_ALL=C sort | uniq > rcmgr_metrics
'

test_expect_success "make sure initial metrics added by setting ResourceMgr.Enabled haven't changed" '
  diff -u ../t0119-prometheus-data/prometheus_metrics_added_by_enabling_rcmgr rcmgr_metrics
'

test_done
