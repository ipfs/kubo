#!/bin/sh
#
# Script to aggregate results using Sharness
#
# Copyright (c) 2014, 2022 Christian Couder, Piotr Galar
# MIT Licensed; see the LICENSE file in this repository.
#

SHARNESS_AGGREGATE_JUNIT="lib/sharness/aggregate-junit-reports.sh"

test -f "$SHARNESS_AGGREGATE_JUNIT" || {
  echo >&2 "Cannot find: $SHARNESS_AGGREGATE_JUNIT"
  echo >&2 "Please check Sharness installation."
  exit 1
}

ls test-results/t*-*.sh.*.xml.part | "$SHARNESS_AGGREGATE_JUNIT" > test-results/sharness.xml
