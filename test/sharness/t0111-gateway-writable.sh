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

test_expect_success "ipfs daemon listening to TCP port $port" '
  test_wait_open_tcp_port_10_sec "$PORT_GWAY"
'

test_expect_success "HTTP gateway gives access to sample file" '
  curl -s -o welcome "http://localhost:$PORT_GWAY/ipfs/$HASH_WELCOME_DOCS/readme" &&
  grep "Hello and Welcome to IPFS!" welcome
'

test_expect_success "HTTP POST file gives Hash" '
  echo "$RANDOM" >infile &&
  URL="http://localhost:$port/ipfs/" &&
  curl -svX POST --data-binary @infile "$URL" 2>curl.out &&
  grep "HTTP/1.1 201 Created" curl.out &&
  LOCATION=$(grep Location curl.out) &&
  HASH=$(expr "$LOCATION" : "< Location: /ipfs/\(.*\)$")
'

# this is failing on osx
# claims "multihash too short. must be > 3 bytes" but the multihash is there.
test_expect_failure "We can HTTP GET file just created" '
  URL="http://localhost:$port/ipfs/$HASH" &&
  curl -so outfile "$URL" &&
  test_cmp infile outfile ||
  echo $URL &&
  test_fsh cat outfile
'

test_expect_success "HTTP PUT empty directory" '
  URL="http://localhost:$port/ipfs/$HASH_EMPTY_DIR/" &&
  echo "PUT $URL" &&
  curl -svX PUT "$URL" 2>curl.out &&
  cat curl.out &&
  grep "Ipfs-Hash: $HASH_EMPTY_DIR" curl.out &&
  grep "Location: /ipfs/$HASH_EMPTY_DIR/" curl.out &&
  grep "HTTP/1.1 201 Created" curl.out
'

test_expect_success "HTTP GET empty directory" '
  echo "GET $URL" &&
  curl -so outfile "$URL" 2>curl.out &&
  grep "Index of /ipfs/$HASH_EMPTY_DIR/" outfile
'

test_expect_success "HTTP PUT file to construct a hierarchy" '
  echo "$RANDOM" >infile &&
  URL="http://localhost:$port/ipfs/$HASH_EMPTY_DIR/test.txt" &&
  echo "PUT $URL" &&
  curl -svX PUT --data-binary @infile "$URL" 2>curl.out &&
  grep "HTTP/1.1 201 Created" curl.out &&
  LOCATION=$(grep Location curl.out) &&
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
  curl -svX PUT --data-binary @infile2 "$URL" 2>curl.out &&
  grep "HTTP/1.1 201 Created" curl.out &&
  LOCATION=$(grep Location curl.out) &&
  HASH=$(expr "$LOCATION" : "< Location: /ipfs/\(.*\)/test/test.txt")
'


test_expect_success "We can HTTP GET file just created" '
  URL="http://localhost:$port/ipfs/$HASH/test/test.txt" &&
  echo "GET $URL" &&
  curl -so outfile2 "$URL" &&
  test_cmp infile2 outfile2 &&
  URL="http://localhost:$port/ipfs/$HASH/test.txt" &&
  echo "GET $URL" &&
  curl -so outfile "$URL" &&
  test_cmp infile outfile
'

test_kill_ipfs_daemon

test_done
