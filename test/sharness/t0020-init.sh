#!/usr/bin/env bash
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test init command"

. lib/test-lib.sh

# test that ipfs fails to init with BAD_IPFS_DIR that isn't writeable
test_expect_success "create dir and change perms succeeds" '
  export BAD_IPFS_DIR="$(pwd)/.badipfs" &&
  mkdir "$BAD_IPFS_DIR" &&
  chmod 000 "$BAD_IPFS_DIR"
'

test_expect_success "ipfs init fails" '
  test_must_fail ipfs init --repo-dir "$BAD_IPFS_DIR" 2> init_fail_out
'

# Under Windows/Cygwin the error message is different,
# so we use the STD_ERR_MSG prereq.
if test_have_prereq STD_ERR_MSG; then
  init_err_msg="Error: error loading plugins: open $BAD_IPFS_DIR/config: permission denied"
else
  init_err_msg="Error: error loading plugins: open $BAD_IPFS_DIR/config: The system cannot find the path specified."
fi

test_expect_success "ipfs init output looks good" '
  echo "$init_err_msg" >init_fail_exp &&
  test_cmp init_fail_exp init_fail_out
'

test_expect_success "cleanup dir with bad perms" '
  chmod 775 "$BAD_IPFS_DIR" &&
  rmdir "$BAD_IPFS_DIR"
'

# test no repo error message
# this applies to `ipfs add sth`, `ipfs refs <hash>`
test_expect_success "ipfs cat fails" '
  export IPFS_DIR="$(pwd)/.ipfs" &&
  test_must_fail ipfs cat --repo-dir "$IPFS_DIR" Qmaa4Rw81a3a1VEx4LxB7HADUAXvZFhCoRdBzsMZyZmqHD 2> cat_fail_out
'

test_expect_success "ipfs cat no repo message looks good" '
  echo "Error: no IPFS repo found in $IPFS_DIR." > cat_fail_exp &&
  echo "please run: '"'"'ipfs init'"'"'" >> cat_fail_exp &&
  test_path_cmp cat_fail_exp cat_fail_out
'

# $1 must be one of 'rsa', 'ed25519' or '' (for default key algorithm).
test_ipfs_init_flags() {
        TEST_ALG=$1

        # test that init succeeds
        test_expect_success "ipfs init succeeds" '
        export IPFS_DIR="$(pwd)/.ipfs" &&
        echo "IPFS_DIR: \"$IPFS_DIR\"" &&
        RSA_BITS="2048" &&
        case $TEST_ALG in
                "rsa")
                        ipfs init --repo-dir "$IPFS_DIR" --algorithm=rsa --bits="$RSA_BITS" >actual_init || test_fsh cat actual_init
                        ;;
                "ed25519")
                        ipfs init --repo-dir "$IPFS_DIR" --algorithm=ed25519 >actual_init || test_fsh cat actual_init
                        ;;
                *)
                        ipfs init --repo-dir "$IPFS_DIR" --algorithm=rsa --bits="$RSA_BITS" >actual_init || test_fsh cat actual_init
                        ;;
        esac
        '

        test_expect_success ".ipfs/ has been created" '
        test -d "$IPFS_DIR" &&
        test -f "$IPFS_DIR/config" &&
        test -d "$IPFS_DIR/datastore" &&
        test -d "$IPFS_DIR/blocks" &&
        test ! -f ._check_writeable ||
        test_fsh ls -al $IPFS_DIR
        '

        test_expect_success "ipfs config succeeds" '
        echo /ipfs >expected_config &&
        ipfs config --repo-dir "$IPFS_DIR" Mounts.IPFS >actual_config &&
        test_cmp expected_config actual_config
        '

        test_expect_success "ipfs peer id looks good" '
        PEERID=$(ipfs config --repo-dir "$IPFS_DIR" Identity.PeerID) &&
        test_check_peerid "$PEERID"
        '

        test_expect_success "ipfs init output looks good" '
        STARTFILE="ipfs cat /ipfs/$HASH_WELCOME_DOCS/readme" &&

        echo "generating $RSA_BITS-bit RSA keypair...done" >rsa_expected &&
        echo "peer identity: $PEERID" >>rsa_expected &&
        echo "initializing IPFS node at $IPFS_DIR" >>rsa_expected &&
        echo "to get started, enter:" >>rsa_expected &&
        printf "\\n\\t$STARTFILE\\n\\n" >>rsa_expected &&

        echo "generating ED25519 keypair...done" >ed25519_expected &&
        echo "peer identity: $PEERID" >>ed25519_expected &&
        echo "initializing IPFS node at $IPFS_DIR" >>ed25519_expected &&
        echo "to get started, enter:" >>ed25519_expected &&
        printf "\\n\\t$STARTFILE\\n\\n" >>ed25519_expected &&

        case $TEST_ALG in
                rsa)
                        test_cmp rsa_expected actual_init
                        ;;
                ed25519)
                        test_cmp ed25519_expected actual_init
                        ;;
                *)
                        test_cmp rsa_expected actual_init
                        ;;
        esac
        '

        test_expect_success "Welcome readme exists" '
        ipfs cat /ipfs/$HASH_WELCOME_DOCS/readme
        '

        test_expect_success "clean up ipfs dir" '
        rm -rf "$IPFS_DIR"
        '

        test_expect_success "'ipfs init --empty-repo' succeeds" '
        RSA_BITS="2048" &&
        case $TEST_ALG in
                rsa)
                        ipfs init --repo-dir "$IPFS_DIR" --algorithm=rsa --bits="$RSA_BITS" --empty-repo >actual_init
                        ;;
                ed25519)
                        ipfs init --repo-dir "$IPFS_DIR" --algorithm=ed25519 --empty-repo >actual_init
                        ;;
                *)
                        ipfs init --repo-dir "$IPFS_DIR" --empty-repo >actual_init
                        ;;
        esac
        '

        test_expect_success "ipfs peer id looks good" '
        PEERID=$(ipfs config --repo-dir "$IPFS_DIR" Identity.PeerID) &&
        test_check_peerid "$PEERID"
        '

        test_expect_success "'ipfs init --empty-repo' output looks good" '

        echo "generating $RSA_BITS-bit RSA keypair...done" >rsa_expected &&
        echo "peer identity: $PEERID" >>rsa_expected &&
        echo "initializing IPFS node at $IPFS_DIR" >>rsa_expected &&

        echo "generating ED25519 keypair...done" >ed25519_expected &&
        echo "peer identity: $PEERID" >>ed25519_expected &&
        echo "initializing IPFS node at $IPFS_DIR" >>ed25519_expected &&

        case $TEST_ALG in
                rsa)
                        test_cmp rsa_expected actual_init
                        ;;
                ed25519)
                        test_cmp ed25519_expected actual_init
                        ;;
                *)
                        test_cmp ed25519_expected actual_init
                        ;;
        esac
        '

        test_expect_success "Welcome readme doesn't exist" '
        test_must_fail ipfs cat /ipfs/$HASH_WELCOME_DOCS/readme
        '

        test_expect_success "ipfs id agent string contains correct version" '
        ipfs id -f "<aver>" | grep $(ipfs version -n)
        '

        test_expect_success "clean up ipfs dir" '
        rm -rf "$IPFS_DIR"
        '
}
test_ipfs_init_flags 'ed25519'
test_ipfs_init_flags 'rsa'
test_ipfs_init_flags ''

# test init profiles
test_expect_success "'ipfs init --profile' with invalid profile fails" '
  RSA_BITS="2048" &&
  test_must_fail ipfs init --repo-dir "$IPFS_DIR" --profile=nonexistent_profile 2> invalid_profile_out
  EXPECT="Error: invalid configuration profile: nonexistent_profile" &&
  grep "$EXPECT" invalid_profile_out
'

test_expect_success "'ipfs init --profile' succeeds" '
  RSA_BITS="2048" &&
  ipfs init --repo-dir "$IPFS_DIR" --profile=server
'

test_expect_success "'ipfs config Swarm.AddrFilters' looks good" '
  ipfs config --repo-dir "$IPFS_DIR" Swarm.AddrFilters > actual_config &&
  test $(cat actual_config | wc -l) = 18
'

test_expect_success "clean up ipfs dir" '
  rm -rf "$IPFS_DIR"
'

test_expect_success "'ipfs init --profile=test' succeeds" '
  RSA_BITS="2048" &&
  ipfs init --repo-dir "$IPFS_DIR" --profile=test
'

test_expect_success "'ipfs config Bootstrap' looks good" '
  ipfs config --repo-dir "$IPFS_DIR" Bootstrap > actual_config &&
  test $(cat actual_config) = "[]"
'

test_expect_success "'ipfs config Addresses.API' looks good" '
  ipfs config --repo-dir "$IPFS_DIR" Addresses.API > actual_config &&
  test $(cat actual_config) = "/ip4/127.0.0.1/tcp/0"
'

test_expect_success "ipfs init from existing config succeeds" '
  export ORIG_PATH=$IPFS_DIR
  export IPFS_DIR=$(pwd)/.ipfs-clone

  ipfs init --repo-dir "$IPFS_DIR" "$ORIG_PATH/config" &&
  ipfs config --repo-dir "$IPFS_DIR" Addresses.API > actual_config &&
  test $(cat actual_config) = "/ip4/127.0.0.1/tcp/0"
'

test_expect_success "clean up ipfs clone dir and reset IPFS_DIR" '
  rm -rf "$IPFS_DIR" &&
  export IPFS_DIR=$ORIG_PATH
'

test_expect_success "clean up ipfs dir" '
  rm -rf "$IPFS_DIR"
'

test_expect_success "'ipfs init --profile=lowpower' succeeds" '
  RSA_BITS="2048" &&
  ipfs init --repo-dir "$IPFS_DIR" --profile=lowpower
'

test_expect_success "'ipfs config Discovery.Routing' looks good" '
  ipfs config --repo-dir "$IPFS_DIR" Routing.Type > actual_config &&
  test $(cat actual_config) = "dhtclient"
'

test_expect_success "clean up ipfs dir" '
  rm -rf "$IPFS_DIR"
'

test_init_ipfs

test_launch_ipfs_daemon

test_expect_success "ipfs init should not run while daemon is running" '
  test_must_fail ipfs init --repo-dir "$IPFS_DIR" 2> daemon_running_err &&
  EXPECT="Error: ipfs daemon is running. please stop it to run this command" &&
  grep "$EXPECT" daemon_running_err
'

test_kill_ipfs_daemon

test_done
