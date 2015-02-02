#!/bin/sh

test_description="Test config command"

. lib/test-lib.sh

# we use a function so that we can run it both offline + online
test_config_cmd_set() {

  # flags (like -bool in "ipfs config -bool")
  cfg_flags="" # unset in case.
  test "$#" = 3 && { cfg_flags=$1; shift; }

  cfg_key=$1
  cfg_val=$2
  test_expect_success "ipfs config succeeds" '
    ipfs config $cfg_flags "$cfg_key" "$cfg_val"
  '

  test_expect_success "ipfs config output looks good" '
    echo "$cfg_val" >expected &&
    ipfs config "$cfg_key" >actual &&
    test_cmp expected actual
  '

  # also test our lib function. it should work too.
  cfg_key="Lib.$cfg_key"
  test_expect_success "test_config_set succeeds" '
    test_config_set $cfg_flags "$cfg_key" "$cfg_val"
  '

  test_expect_success "test_config_set value looks good" '
    echo "$cfg_val" >expected &&
    ipfs config "$cfg_key" >actual &&
    test_cmp expected actual
  '
}


test_config_cmd() {
  test_config_cmd_set "beep" "boop"
  test_config_cmd_set "beep1" "boop2"
  test_config_cmd_set "beep1" "boop2"
  test_config_cmd_set "-bool" "beep2" "true"
  test_config_cmd_set "-bool" "beep2" "false"

}

test_init_ipfs

# should work offline
test_config_cmd

# should work online
test_launch_ipfs_daemon
test_config_cmd
test_kill_ipfs_daemon


test_done
