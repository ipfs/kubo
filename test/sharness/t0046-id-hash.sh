#!/usr/bin/env bash

test_description="Test basic operations with identity hash"

. lib/test-lib.sh

test_init_ipfs

ID_HASH0=bafkqaedknncdsodknncdsnzvnbvuioak
ID_HASH0_CONTENTS=jkD98jkD975hkD8

test_expect_success "can fetch random identity hash" '
  ipfs cat $ID_HASH0 > expected &&
  echo $ID_HASH0_CONTENTS > actual &&
  test_cmp expected actual
'

test_expect_success "can pin random identity hash" '
  ipfs pin add $ID_HASH0
'

test_expect_success "ipfs add succeeds with identity hash" '
  echo "djkd7jdkd7jkHHG" > junk.txt &&
  HASH=$(ipfs add -q --hash=identity junk.txt)
'

test_expect_success "content not actually added" '
  ipfs refs local | fgrep -q -v $HASH
'

test_expect_success "but can fetch it anyway" '
  ipfs cat $HASH > actual &&
  test_cmp junk.txt actual
'

test_expect_success "block rm does nothing" '
  ipfs pin rm $HASH &&
  ipfs block rm $HASH
'

test_expect_success "can still fetch it" '
  ipfs cat $HASH > actual
  test_cmp junk.txt actual
'

test_expect_success "ipfs add --inline works as expected" '
  echo $ID_HASH0_CONTENTS > afile &&
  HASH=$(ipfs add -q --inline afile)
'

test_expect_success "ipfs add --inline uses identity multihash" '
  MHTYPE=`cid-fmt %h $HASH`
  echo "mhtype is $MHTYPE"
  test "$MHTYPE" = identity
'

test_expect_success "ipfs add --inline --raw-leaves works as expected" '
  echo $ID_HASH0_CONTENTS > afile &&
  HASH=$(ipfs add -q --inline --raw-leaves afile)
'

test_expect_success "ipfs add --inline --raw-leaves outputs the correct hash" '
  echo "$ID_HASH0" = "$HASH" &&
  test "$ID_HASH0" = "$HASH"
'

test_expect_success "create 1000 bytes file and get its hash" '
  random 1000 2 > 1000bytes &&
  HASH0=$(ipfs add -q --raw-leaves --only-hash 1000bytes)
'

test_expect_success "ipfs add --inline --raw-leaves works as expected on large file" '
  HASH=$(ipfs add -q --inline --raw-leaves 1000bytes)
'

test_expect_success "ipfs add --inline --raw-leaves outputs the correct hash on large file" '
  echo "$HASH0" = "$HASH" &&
  test "$HASH0" = "$HASH"
'

test_expect_success "enable filestore" '
  ipfs config --json Experimental.FilestoreEnabled true
'

test_expect_success "can fetch random identity hash (filestore enabled)" '
  ipfs cat $ID_HASH0 > expected &&
  echo $ID_HASH0_CONTENTS > actual &&
  test_cmp expected actual
'

test_expect_success "can pin random identity hash (filestore enabled)" '
  ipfs pin add $ID_HASH0
'

test_expect_success "ipfs add succeeds with identity hash and --nocopy" '
  echo "djkd7jdkd7jkHHG" > junk.txt &&
  HASH=$(ipfs add -q --hash=identity --nocopy junk.txt)
'

test_expect_success "content not actually added (filestore enabled)" '
  ipfs refs local | fgrep -q -v $HASH
'

test_expect_success "but can fetch it anyway (filestore enabled)" '
  ipfs cat $HASH > actual &&
  test_cmp junk.txt actual
'

test_done
