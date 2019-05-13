#!/usr/bin/env bash
#
# Copyright (c) 2016 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test object diff command"

. lib/test-lib.sh

test_init_ipfs

test_expect_success "create some objects for testing diffs" '
  mkdir foo &&
  echo "stuff" > foo/bar &&
  mkdir foo/baz &&
  A=$(ipfs add -r -q foo | tail -n1) &&
  AR=$(ipfs add --raw-leaves -r -q foo | tail -n1) &&
  echo "more things" > foo/cat &&
  B=$(ipfs add -r -q foo | tail -n1) &&
  BR=$(ipfs add --raw-leaves -r -q foo | tail -n1) &&
  echo "nested" > foo/baz/dog &&
  C=$(ipfs add -r -q foo | tail -n1)
  CR=$(ipfs add --raw-leaves -r -q foo | tail -n1)
  echo "changed" > foo/bar &&
  D=$(ipfs add -r -q foo | tail -n1) &&
  DR=$(ipfs add --raw-leaves -r -q foo | tail -n1) &&
  echo "" > single_file &&
  SINGLE_FILE=$(ipfs add -r -q single_file | tail -n1) &&
  SINGLE_FILE_RAW=$(ipfs add --raw-leaves -r -q single_file | tail -n1) &&
  mkdir empty_dir
  EMPTY_DIR=$(ipfs add -r -q empty_dir | tail -n1)
  EMPTY_DIR_RAW=$(ipfs add --raw-leaves -r -q empty_dir | tail -n1)
'

test_expect_success "diff against self is empty" '
  ipfs object diff $A $A > diff_out
'

test_expect_success "identity diff output looks good" '
  printf "" > diff_exp &&
  test_cmp diff_exp diff_out
'

test_expect_success "diff (raw-leaves) against self is empty" '
  ipfs object diff $AR $AR > diff_raw_out
'

test_expect_success "identity diff (raw-leaves) output looks good" '
  printf "" > diff_raw_exp &&
  test_cmp diff_raw_exp diff_raw_out
'

test_expect_success "diff against self (single file) is empty" '
  ipfs object diff $SINGLE_FILE $SINGLE_FILE > diff_out &&
  printf "" > diff_exp &&
  test_cmp diff_exp diff_out
'

test_expect_success "diff (raw-leaves) against self (single file) is empty" '
  ipfs object diff $SINGLE_FILE_RAW $SINGLE_FILE_RAW > diff_raw_out &&
  printf "" > diff_raw_exp &&
  test_cmp diff_raw_exp diff_raw_out
'

test_expect_success "diff against self (empty dir) is empty" '
  ipfs object diff $EMPTY_DIR $EMPTY_DIR > diff_out &&
  printf "" > diff_exp &&
  test_cmp diff_exp diff_out
'

test_expect_success "diff (raw-leaves) against self (empty dir) is empty" '
  ipfs object diff $EMPTY_DIR_RAW $EMPTY_DIR_RAW > diff_raw_out &&
  printf "" > diff_raw_exp &&
  test_cmp diff_raw_exp diff_raw_out
'

test_expect_success "diff added link works" '
  ipfs object diff $A $B > diff_out
'

test_expect_success "diff added link looks right" '
  echo + QmUSvcqzhdfYM1KLDbM76eLPdS9ANFtkJvFuPYeZt73d7A \"cat\" > diff_exp &&
  test_cmp diff_exp diff_out
'

test_expect_success "diff (raw-leaves) added link works" '
  ipfs object diff $AR $BR > diff_raw_out
'

test_expect_success "diff (raw-leaves) added link looks right" '
  echo + bafkreig43bpnc6sjo6izaiqzzq5esapazosa3f3wt6jsflwiu3x7ydhq2u \"cat\" > diff_raw_exp &&
  test_cmp diff_raw_exp diff_raw_out
'

test_expect_success "verbose diff added link works" '
  ipfs object diff -v $A $B > diff_out
'

test_expect_success "verbose diff added link looks right" '
  echo Added new link \"cat\" pointing to QmUSvcqzhdfYM1KLDbM76eLPdS9ANFtkJvFuPYeZt73d7A. > diff_exp &&
  test_cmp diff_exp diff_out
'

test_expect_success "verbose diff (raw-leaves) added link works" '
  ipfs object diff -v $AR $BR > diff_raw_out
'

test_expect_success "verbose diff (raw-leaves) added link looks right" '
  echo Added new link \"cat\" pointing to bafkreig43bpnc6sjo6izaiqzzq5esapazosa3f3wt6jsflwiu3x7ydhq2u. > diff_raw_exp &&
  test_cmp diff_raw_exp diff_raw_out
'

test_expect_success "diff removed link works" '
  ipfs object diff -v $B $A > diff_out
'

test_expect_success "diff removed link looks right" '
  echo Removed link \"cat\" \(was QmUSvcqzhdfYM1KLDbM76eLPdS9ANFtkJvFuPYeZt73d7A\). > diff_exp &&
  test_cmp diff_exp diff_out
'

test_expect_success "diff (raw-leaves) removed link works" '
  ipfs object diff -v $BR $AR > diff_raw_out
'

test_expect_success "diff (raw-leaves) removed link looks right" '
  echo Removed link \"cat\" \(was bafkreig43bpnc6sjo6izaiqzzq5esapazosa3f3wt6jsflwiu3x7ydhq2u\). > diff_raw_exp &&
  test_cmp diff_raw_exp diff_raw_out
'

test_expect_success "diff nested add works" '
  ipfs object diff -v $B $C > diff_out
'

test_expect_success "diff looks right" '
  echo Added new link \"baz/dog\" pointing to QmdNJQUTZuDpsUcec7YDuCfRfvw1w4J13DCm7YcU4VMZdS. > diff_exp &&
  test_cmp diff_exp diff_out
'

test_expect_success "diff (raw-leaves) nested add works" '
  ipfs object diff -v $BR $CR > diff_raw_out
'

test_expect_success "diff (raw-leaves) looks right" '
  echo Added new link \"baz/dog\" pointing to bafkreibxbkgajofglo2esqtv53bcp4nwstnqjr3nu2ylrlui5unldf4qum. > diff_raw_exp &&
  test_cmp diff_raw_exp diff_raw_out
'

test_expect_success "diff changed link works" '
  ipfs object diff -v $C $D > diff_out
'

test_expect_success "diff looks right" '
  echo Changed \"bar\" from QmNgd5cz2jNftnAHBhcRUGdtiaMzb5Rhjqd4etondHHST8 to QmRfFVsjSXkhFxrfWnLpMae2M4GBVsry6VAuYYcji5MiZb. > diff_exp &&
  test_cmp diff_exp diff_out
'

test_expect_success "diff (raw-leaves) changed link works" '
  ipfs object diff -v $CR $DR > diff_raw_out
'

test_expect_success "diff（raw-leaves) looks right" '
  echo Changed \"bar\" from bafkreidfn2oemjv5ns2fnc4ukgbjwt6bq5gdd4ciz4mpnehqi2dvwxfbde to bafkreid7rmo7yrtlmje7a3f6kxerotpsk6hhovg2pe755use55olukry6e. > diff_raw_exp &&
  test_cmp diff_raw_exp diff_raw_out
'

test_done
