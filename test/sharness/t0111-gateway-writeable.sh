#!/usr/bin/env bash
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test HTTP Gateway (Writable)"

. lib/test-lib.sh

test_init_ipfs

test_launch_ipfs_daemon --writable
test_expect_success "ipfs daemon --writable overrides config" '
  curl -v -X POST http://$GWAY_ADDR/ipfs/ 2> outfile &&
  grep "HTTP/1.1 201 Created" outfile &&
  grep "Location: /ipfs/QmbFMke1KXqnYyBBWxB74N4c5SBnJMVAiMNRcGu6x1AwQH" outfile
'
test_kill_ipfs_daemon

test_config_ipfs_gateway_writable
test_launch_ipfs_daemon --writable=false
test_expect_success "ipfs daemon --writable=false overrides Writable=true config" '
  curl -v -X POST http://$GWAY_ADDR/ipfs/ 2> outfile &&
  grep "HTTP/1.1 405 Method Not Allowed" outfile
'
test_kill_ipfs_daemon
test_launch_ipfs_daemon

port=$GWAY_PORT

test_expect_success "ipfs daemon up" '
  pollEndpoint -host $GWAY_MADDR -ep=/version -v -tout=1s -tries=60 2>poll_apierr > poll_apiout ||
  test_fsh cat poll_apierr || test_fsh cat poll_apiout
'

test_expect_success "HTTP gateway gives access to sample file" '
  curl -s -o welcome "http://$GWAY_ADDR/ipfs/$HASH_WELCOME_DOCS/readme" &&
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

test_expect_success "HTTP PUT empty directory fails" '
  URL="http://localhost:$port/ipfs/$HASH_EMPTY_DIR/" &&
  echo "PUT $URL" &&
  curl -svX PUT "$URL" 2>curl_putEmpty.out &&
  cat curl_putEmpty.out &&
  grep "HTTP/1.1 400 Bad Request" curl_putEmpty.out
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

test_expect_success "We can replace a file with PUT" '
  echo "$RANDOM" >infile3 &&
  URL="http://localhost:$port/ipfs/$HASH/test/test.txt" &&
  echo "PUT $URL" &&
  curl -svX PUT --data-binary @infile3 "$URL" 2>curl_putExisting.out &&
  grep "HTTP/1.1 201 Created" curl_putExisting.out
'

test_expect_success "Put gives us the hash of the new tree" '
  LOCATION_HEADER=$(grep Location curl_putExisting.out) &&
  IPFS_HASH_HEADER=$(grep Ipfs-Hash curl_putExisting.out) &&
  HASH1=$(expr "$LOCATION_HEADER" : "< Location: /ipfs/\(.*\)/test/test.txt") &&
  HASH2=$(expr "$IPFS_HASH_HEADER" : "< Ipfs-Hash: \(\w*\)") &&
  [ "$HASH2" = "$HASH1" ]
'

test_expect_success "We can HTTP GET file just replaced" '
  URL="http://localhost:$port/ipfs/$HASH/test/test.txt" &&
  echo "GET $URL" &&
  curl -svo outfile3 "$URL" 2>curl_getExisting.out &&
  test_cmp infile3 outfile3
'

test_expect_success "We can't PUT to a path ending with /" '
  URL="http://localhost:$port/ipfs/$HASH/test/" &&
  echo "PUT $URL" &&
  curl -svX PUT "$URL" 2>curl_putDirectoryFail.out &&
  cat curl_putDirectoryFail.out &&
  grep "HTTP/1.1 400 Bad Request" curl_putDirectoryFail.out
'

test_expect_success "PUT to a directory replaces it with a file" '
  echo "$RANDOM" >infile4 &&
  URL="http://localhost:$port/ipfs/$HASH/test" &&
  echo "PUT $URL" &&
  curl -svX PUT --data-binary @infile4 "$URL" 2>curl_putDirectory.out &&
  grep "HTTP/1.1 201 Created" curl_putDirectory.out &&
  LOCATION=$(grep Location curl_putDirectory.out) &&
  HASH=$(expr "$LOCATION" : "< Location: /ipfs/\(.*\)/test")
'

test_expect_success "We can HTTP GET directory just replaced" '
  URL="http://localhost:$port/ipfs/$HASH/test" &&
  echo "GET $URL" &&
  curl -svo outfile4 "$URL" 2>curl_getDirectory.out &&
  test_cmp infile4 outfile4
'

test_kill_ipfs_daemon

test_done
