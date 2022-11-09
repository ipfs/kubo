#!/usr/bin/env bash

test_description="Test HTTP Gateway TAR (application/x-tar) Support"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon_without_network

OUTSIDE_ROOT_CID="bafybeicaj7kvxpcv4neaqzwhrqqmdstu4dhrwfpknrgebq6nzcecfucvyu"
INSIDE_ROOT_CID="bafybeibfevfxlvxp5vxobr5oapczpf7resxnleb7tkqmdorc4gl5cdva3y"

test_expect_success "Add the test directory" '
  mkdir -p rootDir/ipfs &&
  mkdir -p rootDir/ipns &&
  mkdir -p rootDir/api &&
  mkdir -p rootDir/ą/ę &&
  echo "I am a txt file on path with utf8" > rootDir/ą/ę/file-źł.txt &&
  echo "I am a txt file in confusing /api dir" > rootDir/api/file.txt &&
  echo "I am a txt file in confusing /ipfs dir" > rootDir/ipfs/file.txt &&
  echo "I am a txt file in confusing /ipns dir" > rootDir/ipns/file.txt &&
  DIR_CID=$(ipfs add -Qr --cid-version 1 rootDir) &&
  FILE_CID=$(ipfs files stat --enc=json /ipfs/$DIR_CID/ą/ę/file-źł.txt | jq -r .Hash) &&
  FILE_SIZE=$(ipfs files stat --enc=json /ipfs/$DIR_CID/ą/ę/file-źł.txt | jq -r .Size)
  echo "$FILE_CID / $FILE_SIZE"
'

test_expect_success "GET TAR with format=tar and extract" '
  curl "http://127.0.0.1:$GWAY_PORT/ipfs/$FILE_CID?format=tar" | tar -x
'

test_expect_success "GET TAR with 'Accept: application/x-tar' and extract" '
  curl -H "Accept: application/x-tar" "http://127.0.0.1:$GWAY_PORT/ipfs/$FILE_CID" | tar -x
'

test_expect_success "GET TAR with format=tar has expected Content-Type" '
  curl -sD - "http://127.0.0.1:$GWAY_PORT/ipfs/$FILE_CID?format=tar" > curl_output_filename 2>&1 &&
  test_should_contain "Content-Disposition: attachment;" curl_output_filename &&
  test_should_contain "Etag: W/\"$FILE_CID.x-tar" curl_output_filename &&
  test_should_contain "Content-Type: application/x-tar" curl_output_filename
'

test_expect_success "GET TAR with 'Accept: application/x-tar' has expected Content-Type" '
  curl -sD - -H "Accept: application/x-tar" "http://127.0.0.1:$GWAY_PORT/ipfs/$FILE_CID" > curl_output_filename 2>&1 &&
  test_should_contain "Content-Disposition: attachment;" curl_output_filename &&
  test_should_contain "Etag: W/\"$FILE_CID.x-tar" curl_output_filename &&
  test_should_contain "Content-Type: application/x-tar" curl_output_filename
'

test_expect_success "GET TAR has expected root file" '
  rm -rf outputDir && mkdir outputDir &&
  curl "http://127.0.0.1:$GWAY_PORT/ipfs/$FILE_CID?format=tar" | tar -x -C outputDir &&
  test -f "outputDir/$FILE_CID" &&
  echo "I am a txt file on path with utf8" > expected &&
  test_cmp expected outputDir/$FILE_CID
'

test_expect_success "GET TAR has expected root directory" '
  rm -rf outputDir && mkdir outputDir &&
  curl "http://127.0.0.1:$GWAY_PORT/ipfs/$DIR_CID?format=tar" | tar -x -C outputDir &&
  test -d "outputDir/$DIR_CID" &&
  echo "I am a txt file on path with utf8" > expected &&
  test_cmp expected outputDir/$DIR_CID/ą/ę/file-źł.txt
'

test_expect_success "GET TAR with explicit ?filename= succeeds with modified Content-Disposition header" "
  curl -fo actual -D actual_headers 'http://127.0.0.1:$GWAY_PORT/ipfs/$DIR_CID?filename=testтест.tar&format=tar' &&
  grep -F 'Content-Disposition: attachment; filename=\"test____.tar\"; filename*=UTF-8'\'\''test%D1%82%D0%B5%D1%81%D1%82.tar' actual_headers
"

test_expect_success "Add CARs with relative paths to test with" '
  ipfs dag import ../t0122-gateway-tar-data/outside-root.car > import_output &&
  test_should_contain $OUTSIDE_ROOT_CID import_output &&
  ipfs dag import ../t0122-gateway-tar-data/inside-root.car > import_output &&
  test_should_contain $INSIDE_ROOT_CID import_output
'

test_expect_success "GET TAR with relative paths outside root fails" '
  curl -o - "http://127.0.0.1:$GWAY_PORT/ipfs/$OUTSIDE_ROOT_CID?format=tar" > curl_output_filename &&
  test_should_contain "relative UnixFS paths outside the root are now allowed" curl_output_filename
'

test_expect_success "GET TAR with relative paths inside root works" '
  rm -rf outputDir && mkdir outputDir &&
  curl "http://127.0.0.1:$GWAY_PORT/ipfs/$INSIDE_ROOT_CID?format=tar" | tar -x -C outputDir &&
  test -f outputDir/$INSIDE_ROOT_CID/foobar/file
'

test_kill_ipfs_daemon

test_done
