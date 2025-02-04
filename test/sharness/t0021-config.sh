#!/usr/bin/env bash

test_description="Test config command"

. lib/test-lib.sh

# we use a function so that we can run it both offline + online
test_config_cmd_set() {

  # flags (like --bool in "ipfs config --bool")
  cfg_flags="" # unset in case.
  test "$#" = 3 && { cfg_flags=$1; shift; }

  cfg_key=$1
  cfg_val=$2
  test_expect_success "ipfs config succeeds" "
    ipfs config $cfg_flags \"$cfg_key\" \"$cfg_val\"
  "

  test_expect_success "ipfs config output looks good" "
    echo \"$cfg_val\" >expected &&
    if [$cfg_flags != \"--json\"]; then
      ipfs config \"$cfg_key\" >actual &&
      test_cmp expected actual
    else 
      ipfs config \"$cfg_key\" | tr -d \"\\n\\t \" >actual &&
      echo >>actual &&
      test_cmp expected actual
    fi
  "
}

test_profile_apply_revert() {
  profile=$1
  inverse_profile=$2

  test_expect_success "save expected config" '
    ipfs config show >expected
  '

  test_expect_success "'ipfs config profile apply ${profile}' works" '
    ipfs config profile apply '${profile}'
  '

  test_expect_success "profile ${profile} changed something" '
    ipfs config show >actual &&
    test_must_fail test_cmp expected actual
  '

  test_expect_success "'ipfs config profile apply ${inverse_profile}' works" '
    ipfs config profile apply '${inverse_profile}'
  '

  test_expect_success "config is back to previous state after ${inverse_profile} was applied" '
    ipfs config show >actual &&
    test_cmp expected actual
  '
}

test_profile_apply_dry_run_not_alter() {
  profile=$1

  test_expect_success "'ipfs config profile apply ${profile} --dry-run' doesn't alter config" '
    cat "$IPFS_PATH/config" >expected &&
    ipfs config profile apply '${profile}' --dry-run &&
    cat "$IPFS_PATH/config" >actual &&
    test_cmp expected actual
  '
}

test_config_cmd() {
  test_config_cmd_set "Addresses.API" "foo"
  test_config_cmd_set "Addresses.Gateway" "bar"
  test_config_cmd_set "Datastore.GCPeriod" "baz"
  test_config_cmd_set "AutoNAT.ServiceMode" "enabled"
  test_config_cmd_set "--bool" "Discovery.MDNS.Enabled" "true"
  test_config_cmd_set "--bool" "Discovery.MDNS.Enabled" "false"
  test_config_cmd_set "--json" "Datastore.HashOnRead" "true"
  test_config_cmd_set "--json" "Datastore.HashOnRead" "false"
  test_config_cmd_set "--json" "Experimental.FilestoreEnabled" "true"
  test_config_cmd_set "--json" "Import.BatchMaxSize" "null"
  test_config_cmd_set "--json" "Import.UnixFSRawLeaves" "true"
  test_config_cmd_set "--json" "Routing.Routers.Test" "{\\\"Parameters\\\":\\\"Test\\\",\\\"Type\\\":\\\"Test\\\"}"
  test_config_cmd_set "--json" "Experimental.OptimisticProvideJobsPoolSize" "1337"
  test_config_cmd_set "--json" "Addresses.Swarm" "[\\\"test\\\",\\\"test\\\",\\\"test\\\"]"
  test_config_cmd_set "--json" "Gateway.PublicGateways.Foo" "{\\\"DeserializedResponses\\\":true,\\\"InlineDNSLink\\\":false,\\\"NoDNSLink\\\":false,\\\"Paths\\\":[\\\"Bar\\\",\\\"Baz\\\"],\\\"UseSubdomains\\\":true}"
  test_config_cmd_set "--bool" "Gateway.PublicGateways.Foo.UseSubdomains" "false"

  test_expect_success "'ipfs config show' works" '
    ipfs config show >actual
  '

  test_expect_success "'ipfs config show' output looks good" '
    grep "\"API\": \"foo\"," actual &&
    grep "\"Gateway\": \"bar\"" actual &&
    grep "\"Enabled\": false" actual &&
    grep "\"HashOnRead\": false" actual
  '

  test_expect_success "'ipfs config show --config-file' works" '
    mv "$IPFS_PATH/config" "$IPFS_PATH/config-moved" &&
    ipfs config --config-file "$IPFS_PATH/config-moved" show >moved &&
    test_cmp moved actual &&
    mv "$IPFS_PATH/config-moved" "$IPFS_PATH/config"
  '

  test_expect_success "setup for config replace test" '
    cp "$IPFS_PATH/config" newconfig.json &&
    sed -i"~" -e /PrivKey/d -e s/10GB/11GB/ newconfig.json &&
    sed -i"~" -e '"'"'/PeerID/ {'"'"' -e '"'"' s/,$// '"'"' -e '"'"' } '"'"' newconfig.json
  '

  test_expect_success "run 'ipfs config replace'" '
  ipfs config replace - < newconfig.json
  '

  test_expect_success "check resulting config after 'ipfs config replace'" '
    sed -e /PrivKey/d "$IPFS_PATH/config" > replconfig.json &&
    sed -i"~" -e '"'"'/PeerID/ {'"'"' -e '"'"' s/,$// '"'"' -e '"'"' } '"'"' replconfig.json &&
    test_cmp replconfig.json newconfig.json
  '

  # SECURITY
  # Those tests are here to prevent exposing the PrivKey on the network

  test_expect_success "'ipfs config Identity' fails" '
    test_expect_code 1 ipfs config Identity 2> ident_out
  '

  test_expect_success "output looks good" '
    echo "Error: cannot show or change private key through API" > ident_exp &&
    test_cmp ident_exp ident_out
  '

  test_expect_success "'ipfs config Identity.PrivKey' fails" '
    test_expect_code 1 ipfs config Identity.PrivKey 2> ident_out
  '

  test_expect_success "output looks good" '
    test_cmp ident_exp ident_out
  '

  test_expect_success "lower cased PrivKey" '
    sed -i"~" -e '\''s/PrivKey/privkey/'\'' "$IPFS_PATH/config" &&
    test_expect_code 1 ipfs config Identity.privkey 2> ident_out
  '

  test_expect_success "output looks good" '
    test_cmp ident_exp ident_out
  '

  test_expect_success "fix it back" '
    sed -i"~" -e '\''s/privkey/PrivKey/'\'' "$IPFS_PATH/config"
  '

  test_expect_success "'ipfs config show' doesn't include privkey" '
    ipfs config show > show_config &&
    test_expect_code 1 grep PrivKey show_config
  '

  test_expect_success "'ipfs config replace' injects privkey back" '
    ipfs config replace show_config &&
    grep "\"PrivKey\":" "$IPFS_PATH/config" | grep -e ": \".\+\"" >/dev/null
  '

  test_expect_success "'ipfs config replace' with privkey errors out" '
    cp "$IPFS_PATH/config" real_config &&
    test_expect_code 1 ipfs config replace - < real_config 2> replace_out
  '

  test_expect_success "output looks good" '
    echo "Error: setting private key with API is not supported" > replace_expected
    test_cmp replace_out replace_expected
  '

  test_expect_success "'ipfs config replace' with lower case privkey errors out" '
    cp "$IPFS_PATH/config" real_config &&
    sed -i -e '\''s/PrivKey/privkey/'\'' real_config &&
    test_expect_code 1 ipfs config replace - < real_config 2> replace_out
  '

  test_expect_success "output looks good" '
    echo "Error: setting private key with API is not supported" > replace_expected
    test_cmp replace_out replace_expected
  '

  test_expect_success "'ipfs config Swarm.AddrFilters' looks good" '
    ipfs config Swarm.AddrFilters > actual_config &&
    test $(cat actual_config | wc -l) = 1
  '

  test_expect_success "copy ipfs config" '
    cp "$IPFS_PATH/config" before_patch
  '

  test_expect_success "'ipfs config profile apply server' works" '
    ipfs config profile apply server
  '

  test_expect_success "backup was created and looks good" '
    test_cmp "$(find "$IPFS_PATH" -name "config-*")" before_patch
  '

  test_expect_success "'ipfs config Swarm.AddrFilters' looks good with server profile" '
    ipfs config Swarm.AddrFilters > actual_config &&
    test $(cat actual_config | wc -l) = 18
  '

  test_expect_success "'ipfs config profile apply local-discovery' works" '
    ipfs config profile apply local-discovery
  '

  test_expect_success "'ipfs config Swarm.AddrFilters' looks good with applied local-discovery profile" '
    ipfs config Swarm.AddrFilters > actual_config &&
    test $(cat actual_config | wc -l) = 1
  '

  test_profile_apply_revert server local-discovery

  # tests above mess with values this profile changes, need to do that before testing test profile
  test_expect_success "ensure test profile is applied fully" '
    ipfs config profile apply test
  '

  # need to do this in reverse as the test profile is already applied in sharness
  test_profile_apply_revert default-networking test

  test_profile_apply_dry_run_not_alter server

  test_profile_apply_dry_run_not_alter local-discovery

  test_profile_apply_dry_run_not_alter test

  test_expect_success "'ipfs config profile apply local-discovery --dry-run' looks good with different profile info" '
    ipfs config profile apply local-discovery --dry-run > diff_info &&
    test `grep "DisableNatPortMap" diff_info | wc -l` = 2
  '

  test_expect_success "'ipfs config profile apply server --dry-run' looks good with same profile info" '
    ipfs config profile apply server --dry-run > diff_info &&
    test `grep "DisableNatPortMap" diff_info | wc -l` = 1
  '

  test_expect_success "'ipfs config profile apply server' looks good with same profile info" '
    ipfs config profile apply server > diff_info &&
    test `grep "DisableNatPortMap" diff_info | wc -l` = 1
  '

  test_expect_success "'ipfs config profile apply local-discovery' looks good with different profile info" '
    ipfs config profile apply local-discovery > diff_info &&
    test `grep "DisableNatPortMap" diff_info | wc -l` = 2
  '

  test_expect_success "'ipfs config profile apply test' looks good with different profile info" '
    ipfs config profile apply test > diff_info &&
    test `grep "DisableNatPortMap" diff_info | wc -l` = 2
  '

  test_expect_success "'ipfs config profile apply test --dry-run' doesn't include privkey" '
    ipfs config profile apply test --dry-run > show_config &&
    test_expect_code 1 grep PrivKey show_config
  '

  test_expect_success "'ipfs config profile apply test' doesn't include privkey" '
    ipfs config profile apply test > show_config &&
    test_expect_code 1 grep PrivKey show_config
  '

  # won't work as it changes datastore definition, which makes ipfs not launch
  # without converting first
  # test_profile_apply_revert pebbleds

  test_expect_success "cleanup config backups" '
    find "$IPFS_PATH" -name "config-*" -exec rm {} \;
  '
}

test_init_ipfs

# should work offline
test_config_cmd

# should work online
test_launch_ipfs_daemon
test_config_cmd
test_kill_ipfs_daemon


test_done
