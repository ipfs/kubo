#!/bin/sh
#
# Copyright (c) 2015 Cayman Nava
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test Content-Type for channel-streaming commands"

. lib/test-lib.sh

test_init_ipfs

test_ls_cmd() {

	test_expect_success "Text encoded channel-streaming command succeeds" '
		mkdir -p testdir &&
		echo "hello test" >testdir/test.txt &&
		ipfs add -r testdir &&
        curl -i "http://localhost:$PORT_API/api/v0/refs?arg=QmTcJAn3JP8ZMAKS6WS75q8sbTyojWKbxcUHgLYGWur4Ym&stream-channels=true&encoding=text" >actual_output
	'

	test_expect_success "Text encoded channel-streaming command output looks good" '
		printf "HTTP/1.1 200 OK\r\n" >expected_output &&
		printf "Content-Type: text/plain\r\n" >>expected_output &&
		printf "Transfer-Encoding: chunked\r\n" >>expected_output &&
		printf "X-Chunked-Output: 1\r\n" >>expected_output &&
		printf "\r\n" >>expected_output &&
		echo QmRmPLc1FsPAn8F8F9DQDEYADNX5ER2sgqiokEvqnYknVW >>expected_output &&
		test_cmp expected_output actual_output
	'

	test_expect_success "JSON encoded channel-streaming command succeeds" '
		mkdir -p testdir &&
		echo "hello test" >testdir/test.txt &&
		ipfs add -r testdir &&
        curl -i "http://localhost:$PORT_API/api/v0/refs?arg=QmTcJAn3JP8ZMAKS6WS75q8sbTyojWKbxcUHgLYGWur4Ym&stream-channels=true&encoding=json" >actual_output
	'

	test_expect_success "JSON encoded channel-streaming command output looks good" '
		printf "HTTP/1.1 200 OK\r\n" >expected_output &&
		printf "Content-Type: application/json\r\n" >>expected_output &&
		printf "Transfer-Encoding: chunked\r\n" >>expected_output &&
		printf "X-Chunked-Output: 1\r\n" >>expected_output &&
		printf "\r\n" >>expected_output &&
		cat <<-\EOF >>expected_output &&
			{
			  "Ref": "QmRmPLc1FsPAn8F8F9DQDEYADNX5ER2sgqiokEvqnYknVW",
			  "Err": ""
			}
		EOF
		perl -pi -e '"'"'chomp if eof'"'"' expected_output &&
		test_cmp expected_output actual_output
	'
}

# should work online (only)
test_launch_ipfs_daemon
test_ls_cmd
test_kill_ipfs_daemon

test_done
