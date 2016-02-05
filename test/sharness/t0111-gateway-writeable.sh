#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test HTTP Gateway (Writable)"

. lib/test-lib.sh

test_init_ipfs
test_config_ipfs_gateway_writable $ADDR_GWAY
test_launch_ipfs_daemon

port=$PORT_GWAY

test_expect_success "ipfs daemon up" '
  pollEndpoint -host $ADDR_GWAY -ep=/version -v -tout=1s -tries=60 2>poll_apierr > poll_apiout ||
  test_fsh cat poll_apierr || test_fsh cat poll_apiout
'

test_expect_success "HTTP gateway gives access to sample file" '
  curl -s -o welcome "http://localhost:$PORT_GWAY/ipfs/$HASH_WELCOME_DOCS/readme" &&
  grep "Hello and Welcome to IPFS!" welcome
'

test_expect_success "HTTP POST file gives Hash" '
  echo "$RANDOM" >infile &&
  URL="http://localhost:$port/ipfs/" &&
  curl -svX POST --data-binary @infile "$URL" 2>curl_post.out &&
  grep "HTTP/1.1 201 Created" curl_post.out &&
  LOCATION=$(grep Location curl_post.out) &&
  HASH=$(echo $LOCATION | cut -d":" -f2- |tr -d " \n\r")
'

test_expect_success "We can HTTP GET file just created" '
  URL="http://localhost:${port}${HASH}" &&
  curl -so outfile "$URL" &&
  test_cmp infile outfile
'

test_expect_success "File is pinned (unpin file should work)" '
  ipfs pin rm -r $HASH
'

test_expect_success "HTTP POST file with param pin=false gives Hash" '
  echo "$RANDOM" >infile &&
  URL="http://localhost:$port/ipfs/?pin=false" &&
  curl -svX POST --data-binary @infile "$URL" 2>curl_post.out &&
  grep "HTTP/1.1 201 Created" curl_post.out &&
  LOCATION=$(grep Location curl_post.out) &&
  HASH=$(echo $LOCATION | cut -d":" -f2- |tr -d " \n\r")
'

test_expect_success "We can HTTP GET file just created" '
  URL="http://localhost:${port}${HASH}" &&
  curl -so outfile "$URL" &&
  test_cmp infile outfile
'

test_expect_success "File is not pinned (unpin file should fail)" '
  test_must_fail ipfs pin rm -r $HASH
'

test_expect_success "HTTP POST multiple files gives Hash" '
  echo hapax >infile1 &&
  echo dis >infile2 &&
  URL="http://localhost:$port/ipfs/" &&
  curl -v --form "infile1=@infile1" --form "infile2=@infile2" "$URL" 2>curl_post.out &&
  grep "HTTP/1.1 201 Created" curl_post.out &&
  LOCATION=$(grep Location curl_post.out) &&
  HASH=$(echo $LOCATION | cut -d":" -f2- |tr -d " \n\r")
'

test_expect_success "We can ipfs get the files just created" '
  #ipfs get is used here instead since curl does not download multiple files
  ipfs get -o actual "$HASH" &&
  test_cmp infile1 actual/infile1 &&
  test_cmp infile2 actual/infile2
'

test_expect_success "HTTP PUT empty directory" '
  URL="http://localhost:$port/ipfs/$HASH_EMPTY_DIR/" &&
  echo "PUT $URL" &&
  curl -svX PUT "$URL" 2>curl_putEmpty.out &&
  cat curl_putEmpty.out &&
  grep "Ipfs-Hash: $HASH_EMPTY_DIR" curl_putEmpty.out &&
  grep "Location: /ipfs/$HASH_EMPTY_DIR" curl_putEmpty.out &&
  grep "HTTP/1.1 201 Created" curl_putEmpty.out
'

test_expect_success "HTTP GET empty directory" '
  echo "GET $URL" &&
  curl -so outfile "$URL" 2>curl_getEmpty.out &&
  grep "Index of /ipfs/$HASH_EMPTY_DIR/" outfile
'

test_expect_success "HTTP PUT file to construct a hierarchy" '
  echo "$RANDOM" >infile &&
  URL="http://localhost:$port/ipfs/$HASH_EMPTY_DIR/test.txt" &&
  echo "PUT $URL" &&
  curl -svX PUT --data-binary @infile "$URL" 2>curl_put.out &&
  grep "HTTP/1.1 201 Created" curl_put.out &&
  LOCATION=$(grep Location curl_put.out) &&
  HASH=$(expr "$LOCATION" : "< Location: /ipfs/\(.*\)/test.txt")
'

test_expect_success "We can HTTP GET file just created" '
  URL="http://localhost:$port/ipfs/$HASH/test.txt" &&
  echo "GET $URL" &&
  curl -so outfile "$URL" &&
  test_cmp infile outfile
'

test_expect_success "HTTP PUT file to append to existing hierarchy" '
  echo "$RANDOM" >infile2 &&
  URL="http://localhost:$port/ipfs/$HASH/test/test.txt" &&
  echo "PUT $URL" &&
  curl -svX PUT --data-binary @infile2 "$URL" 2>curl_putAgain.out &&
  grep "HTTP/1.1 201 Created" curl_putAgain.out &&
  LOCATION=$(grep Location curl_putAgain.out) &&
  HASH=$(expr "$LOCATION" : "< Location: /ipfs/\(.*\)/test/test.txt")
'


test_expect_success "We can HTTP GET file just updated" '
  URL="http://localhost:$port/ipfs/$HASH/test/test.txt" &&
  echo "GET $URL" &&
  curl -svo outfile2 "$URL" 2>curl_getAgain.out &&
  test_cmp infile2 outfile2
'

test_kill_ipfs_daemon

test_done
