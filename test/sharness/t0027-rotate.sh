#!/usr/bin/env bash

test_description="Test rotate command"

. lib/test-lib.sh

test_rotate() {
        FROM_ALG=$1
        TO_ALG=$2

        test_expect_success "ipfs init (from $FROM_ALG, to $TO_ALG)" '
        export IPFS_PATH="$(pwd)/.ipfs" &&
        case $FROM_ALG in
        rsa)
                ipfs init --profile=test -a=rsa > /dev/null
                ;;
        ed25519)
                ipfs init --profile=test -a=ed25519 > /dev/null
                ;;
        *)
                ipfs init --profile=test > /dev/null
                ;;
        esac
        '

        test_expect_success "Save first ID and key" '
        ipfs id -f="<id>" > first_id &&
        ipfs id -f="<pubkey>" > first_key
        '

        test_launch_ipfs_daemon

        test_kill_ipfs_daemon

        test_expect_success "rotating keys" '
        case $TO_ALG in
        rsa)
                ipfs key rotate -t=rsa -s=2048 --oldkey=oldkey
                ;;
        ed25519)
                ipfs key rotate -t=ed25519 --oldkey=oldkey
                ;;
        *)
                ipfs key rotate --oldkey=oldkey
                ;;
        esac
        '

        test_expect_success "'ipfs key rotate -o self' should fail" '
        echo "Error: keystore name for back up cannot be named '\''self'\''" >expected-self
        test_must_fail ipfs key rotate -o self 2>actual-self &&
        test_cmp expected-self actual-self
        '

        test_expect_success "Compare second ID and key to first" '
        ipfs id -f="<id>" > second_id &&
        ipfs id -f="<pubkey>" > second_key &&
        ! test_cmp first_id second_id &&
        ! test_cmp first_key second_key
        '

        test_expect_success "checking ID" '
        ipfs config Identity.PeerID > expected-id &&
        ipfs id -f "<id>\n" > actual-id &&
        ipfs key list -l --ipns-base=b58mh | grep self | cut -d " " -f1 > keystore-id &&
        ipfs key list -l --ipns-base=b58mh | grep oldkey | cut -d " " -f1 | tr -d "\n" > old-keystore-id &&
        test_cmp expected-id actual-id &&
        test_cmp expected-id keystore-id &&
        test_cmp old-keystore-id first_id
        '

        test_launch_ipfs_daemon

        test_expect_success "publish name with new and old keys" '
        echo "hello world" > msg &&
        ipfs add msg | cut -d " " -f2 | tr -d "\n" > msg_hash &&
        ipfs name publish --offline --allow-offline --key=self $(cat msg_hash) &&
        ipfs name publish --offline --allow-offline --key=oldkey $(cat msg_hash)
        '

        test_kill_ipfs_daemon

        test_expect_success "clean up ipfs dir" '
        rm -rf "$IPFS_PATH"
        '

}
test_rotate 'rsa' ''
test_rotate 'ed25519' ''
test_rotate '' ''
test_rotate 'rsa' 'rsa'
test_rotate 'ed25519' 'rsa'
test_rotate '' 'rsa'
test_rotate 'rsa' 'ed25519'
test_rotate 'ed25519' 'ed25519'
test_rotate '' 'ed25519'

test_done
