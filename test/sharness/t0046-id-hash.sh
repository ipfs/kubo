#!/usr/bin/env bash

test_description="Test basic operations with identity hash"

. lib/test-lib.sh

test_init_ipfs

ID_HASH0=z25RnHTQ7DveGAsV6YDSDR8EkWvD
ID_HASH0_CONTENTS=jkD98jkD975hkD8

test_expect_success "can fetch random id hash" '
  ipfs cat $ID_HASH0 > expected &&
  echo $ID_HASH0_CONTENTS > actual &&
  test_cmp expected actual
'

test_expect_success "can pin random id hash" '
  ipfs pin add $ID_HASH0
'

test_expect_success "ipfs add succeeds with id hash" '
  echo "djkd7jdkd7jkHHG" > junk.txt &&
  HASH=$(ipfs add -q --hash=id junk.txt)
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

test_expect_success "enable filestore" '
  ipfs config --json Experimental.FilestoreEnabled true
'

test_expect_success "can fetch random id hash (filestore enabled)" '
  ipfs cat $ID_HASH0 > expected &&
  echo $ID_HASH0_CONTENTS > actual &&
  test_cmp expected actual
'

test_expect_success "can pin random id hash (filestore enabled)" '
  ipfs pin add $ID_HASH0
'

test_expect_success "ipfs add succeeds with id hash and --nocopy" '
  echo "djkd7jdkd7jkHHG" > junk.txt &&
  HASH=$(ipfs add -q --hash=id --nocopy junk.txt)
'

test_expect_success "content not actually added (filestore enabled)" '
  ipfs refs local | fgrep -q -v $HASH
'

test_expect_success "but can fetch it anyway (filestore enabled)" '
  ipfs cat $HASH > actual &&
  test_cmp junk.txt actual
'

test_done
