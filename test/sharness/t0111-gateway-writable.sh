#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test HTTP Gateway (Writable)"

. lib/test-lib.sh

test_init_ipfs
test_config_ipfs_gateway_writable "/ip4/0.0.0.0/tcp/5002"
test_launch_ipfs_daemon

test_expect_success "ipfs daemon listening to TCP port 5002" '
  test_wait_open_tcp_port_10_sec 5002
'

test_expect_success "HTTP gateway gives access to sample file" '
  curl -s -o welcome "http://localhost:5002/ipfs/$HASH_WELCOME_DOCS/readme" &&
  grep "Hello and Welcome to IPFS!" welcome
'

test_expect_success "HTTP POST file gives Hash" '
  echo "$RANDOM" >infile &&
  curl -svX POST --data-binary @infile http://localhost:5002/ipfs/ 2>curl.out &&
  grep "HTTP/1.1 201 Created" curl.out
'

test_expect_success "We can HTTP GET file just created" '
  FILEPATH=$(grep Location curl.out | cut -d" " -f3- | tr -d "\r")
  curl -so outfile http://localhost:5002$FILEPATH &&
  diff -u infile outfile
'

test_expect_success "HTTP PUT empty directory" '
  echo "PUT http://localhost:5002/ipfs/$HASH_EMPTY_DIR/" &&
  curl -svX PUT "http://localhost:5002/ipfs/$HASH_EMPTY_DIR/" 2>curl.out &&
  cat curl.out &&
  grep "Ipfs-Hash: $HASH_EMPTY_DIR" curl.out &&
  grep "Location: /ipfs/$HASH_EMPTY_DIR/" curl.out &&
  grep "HTTP/1.1 201 Created" curl.out
'

test_expect_success "HTTP GET empty directory" '
  echo "GET http://localhost:5002/ipfs/$HASH_EMPTY_DIR/" &&
  curl -so outfile "http://localhost:5002/ipfs/$HASH_EMPTY_DIR/" 2>curl.out &&
  grep "Index of /ipfs/$HASH_EMPTY_DIR/" outfile
'

test_expect_success "HTTP PUT file to construct a hierarchy" '
  echo "$RANDOM" >infile
  echo "PUT http://localhost:5002/ipfs/$HASH_EMPTY_DIR/test.txt" &&
  curl -svX PUT --data-binary @infile "http://localhost:5002/ipfs/$HASH_EMPTY_DIR/test.txt" 2>curl.out &&
  grep "HTTP/1.1 201 Created" curl.out &&
  grep Location curl.out
'

test_expect_success "We can HTTP GET file just created" '
  FILEPATH=$(grep Location curl.out | cut -d" " -f3- | tr -d "\r") &&
  echo "$FILEPATH" = "${FILEPATH%/test.txt}/test.txt" &&
  [ "$FILEPATH" = "${FILEPATH%/test.txt}/test.txt" ] &&
  echo "GET http://localhost:5002$FILEPATH" &&
  curl -so outfile http://localhost:5002$FILEPATH &&
  diff -u infile outfile
'

test_expect_success "HTTP PUT file to append to existing hierarchy" '
  echo "$RANDOM" >infile2 &&
  echo "PUT http://localhost:5002${FILEPATH%/test.txt}/test/test.txt" &&
  curl -svX PUT --data-binary @infile2 "http://localhost:5002${FILEPATH%/test.txt}/test/test.txt 2>curl.out" &&
  grep "HTTP/1.1 201 Created" curl.out &&
  grep Location curl.out
'

# TODO: this seems not to be working.
# $FILEPATH is set to: /ipfs/QmcpPkdv1K5Rk1bT9Y4rx4FamT7ujry2C61HMzZEAuAnms/test.txt
# $FILEPATH should be set to /ipfs/<some hash>/test/test.txt
test_expect_failure "We can HTTP GET file just created" '
  FILEPATH=$(grep Location curl.out | cut -d" " -f3- | tr -d "\r");
  [ "$FILEPATH" = "${FILEPATH%/test/test.txt}/test/test.txt" ] &&
  echo "GET http://localhost:5002$FILEPATH" &&
  curl -so outfile2 "http://localhost:5002$FILEPATH" &&
  diff -u infile2 outfile2 &&
  echo "GET http://localhost:5002${FILEPATH%/test/test.txt}/test.txt" &&
  curl -so outfile "http://localhost:5002${FILEPATH%/test/test.txt}/test.txt" &&
  diff -u infile outfile
'

test_kill_ipfs_daemon

test_done
