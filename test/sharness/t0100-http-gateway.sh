#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test HTTP Gateway"

exec 3>&1 4>&2
. lib/test-lib.sh

test_expect_success "Configure http gateway" '
  export IPFS_PATH="$PWD/.go-ipfs";
  if ! [ -e ../ipfs-path ]; then
    IPFS_PATH="$PWD/../ipfs-path" ipfs init;
  fi;
  cp -R ../ipfs-path "$IPFS_PATH" &&
  ipfs config Addresses.Gateway /ip4/0.0.0.0/tcp/5002 &&
  ipfs config -bool Gateway.Writable true
'

test_expect_success "ipfs daemon --init launches and listen to TCP port 5002" '
  export IPFS_PATH="$PWD/.go-ipfs" &&
  ipfs daemon 2>&1 >actual_init &
  IPFS_PID=$(ps | grep ipfs | awk "{print \$1}");
  test_wait_open_tcp_port_10_sec 5002
'

test_expect_success "HTTP gateway gives access to sample file" '
  curl -s -o welcome http://localhost:5002/ipfs/QmTTFXiXoixwT53tcGPu419udsHEHYu6AHrQC8HAKdJYaZ &&
  grep "Hello and Welcome to IPFS!" welcome
'

test_expect_success "HTTP POST file gives Hash" '
  echo "$RANDOM" >infile
  curl -svX POST --data-binary @infile http://localhost:5002/ipfs/ 2>curl.out && 
  grep "HTTP/1.1 201 Created" curl.out
'

test_expect_success "We can HTTP GET file just created" '
  path=$(grep Location curl.out | cut -d" " -f3- | tr -d "\r")
  curl -so outfile http://localhost:5002$path &&
  diff -u infile outfile
'

test_expect_success "HTTP PUT empty directory" '
  echo "PUT http://localhost:5002/ipfs/QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn/" &&
  curl -svX PUT http://localhost:5002/ipfs/QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn/ 2>curl.out &&
  cat curl.out &&
  grep "Ipfs-Hash: QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn" curl.out &&
  grep "Location: /ipfs/QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn/" curl.out &&
  grep "HTTP/1.1 201 Created" curl.out
'

test_expect_success "HTTP GET empty directory" '
  echo "GET http://localhost:5002/ipfs/QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn/" &&
  curl -so outfile http://localhost:5002/ipfs/QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn/ 2>curl.out && 
  grep "Index of /ipfs/QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn/" outfile
'

test_expect_success "HTTP PUT file to construct a hierarchy" '
  echo "$RANDOM" >infile
  echo "PUT http://localhost:5002/ipfs/QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn/test.txt" &&
  curl -svX PUT --data-binary @infile http://localhost:5002/ipfs/QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn/test.txt 2>curl.out && 
  grep "HTTP/1.1 201 Created" curl.out &&
  grep Location curl.out
'

test_expect_success "We can HTTP GET file just created" '
  path=$(grep Location curl.out | cut -d" " -f3- | tr -d "\r");
  echo "$path" = "${path%/test.txt}/test.txt";
  [ "$path" = "${path%/test.txt}/test.txt" ] &&
  echo "GET http://localhost:5002$path" &&
  curl -so outfile http://localhost:5002$path &&
  diff -u infile outfile
'

test_expect_success "HTTP PUT file to append to existing hierarchy" '
  echo "$RANDOM" >infile2;
  echo "PUT http://localhost:5002${path%/test.txt}/test/test.txt" &&
  curl -svX PUT --data-binary @infile2 http://localhost:5002${path%/test.txt}/test/test.txt 2>curl.out && 
  grep "HTTP/1.1 201 Created" curl.out &&
  grep Location curl.out
'

test_expect_success "We can HTTP GET file just created" '
  path=$(grep Location curl.out | cut -d" " -f3- | tr -d "\r");
  [ "$path" = "${path%/test/test.txt}/test/test.txt" ] &&
  echo "GET http://localhost:5002$path" &&
  curl -so outfile2 http://localhost:5002$path &&
  diff -u infile2 outfile2 &&
  echo "GET http://localhost:5002${path%/test/test.txt}/test.txt" &&
  curl -so outfile http://localhost:5002${path%/test/test.txt}/test.txt &&
  diff -u infile outfile
'

test_expect_success "daemon is still running" '
  echo IPFS_PID=$IPFS_PID;
  kill -15 $IPFS_PID
'

test_expect_success "'ipfs daemon' can be killed" '
  test_kill_repeat_10_sec $IPFS_PID
'

test_done
