#!/bin/sh
#
# Copyright (c) 2014 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test ipfs repo operations"

. lib/test-lib.sh

test_init_ipfs

# test publishing a hash

test_expect_success "'ipfs name publish' succeeds" '
	PEERID=`ipfs id --format="<id>"` &&
	test_check_peerid "${PEERID}" &&
	ipfs name publish "/ipfs/$HASH_WELCOME_DOCS" >publish_out
'

test_expect_success "publish output looks good" '
	echo "Published to ${PEERID}: /ipfs/$HASH_WELCOME_DOCS" >expected1 &&
	test_cmp expected1 publish_out
'

test_expect_success "'ipfs name resolve' succeeds" '
	ipfs name resolve "$PEERID" >output
'

test_expect_success "resolve output looks good" '
	printf "/ipfs/%s" "$HASH_WELCOME_DOCS" >expected2 &&
	test_cmp expected2 output
'

# now test with a path

test_expect_success "'ipfs name publish' succeeds" '
	PEERID=`ipfs id --format="<id>"` &&
	test_check_peerid "${PEERID}" &&
	ipfs name publish "/ipfs/$HASH_WELCOME_DOCS/help" >publish_out
'

test_expect_success "publish a path looks good" '
	echo "Published to ${PEERID}: /ipfs/$HASH_WELCOME_DOCS/help" >expected3 &&
	test_cmp expected3 publish_out
'

test_expect_success "'ipfs name resolve' succeeds" '
	ipfs name resolve "$PEERID" >output
'

test_expect_success "resolve output looks good" '
	printf "/ipfs/%s/help" "$HASH_WELCOME_DOCS" >expected4 &&
	test_cmp expected4 output
'

test_expect_success "ipfs cat on published content succeeds" '
    ipfs cat "/ipfs/$HASH_WELCOME_DOCS/help" >expected &&
    ipfs cat "/ipns/$PEERID" >actual &&
    test_cmp expected actual
'

# publish with a specific private key (DEADKP_PK)

# Create some non-useless content
# (Someone should pin this.)
HASH_DEADKP_DOC=`echo 'Public Key: $HASH_DEADKP_ID
Private Key: $HASH_DEADKP_PK

This keypair is for documentation purposes.
If you have a need for a keypair in IPFS documentation,
feel free to use this keypair.' | ipfs add | grep -o Qm[a-zA-Z0-9]* | tail -n 1`

test_expect_success "'ipfs name publish <hash> <privkey>' succeeds" '
	echo ipfs name publish "/ipfs/$HASH_DEADKP_DOC" $HASH_DEADKP_PK &&
	ipfs name publish "/ipfs/$HASH_DEADKP_DOC" $HASH_DEADKP_PK >actual_privkey_publish
'

test_expect_success "publish with our explicit privkey looks good" '
	echo "Published to ${HASH_DEADKP_ID}: /ipfs/$HASH_DEADKP_DOC" >expected_privkey_publish &&
	test_cmp expected_privkey_publish actual_privkey_publish
'

# Note that we can't resolve this because the DEADKP is a public keypair (by nature of it being used in tests),
# so it would be perfectly possible for someone to perform a bit of mischief to make the tests fail.
# And since the content's going to likely remain the same, it's a useless check, really.

test_done
