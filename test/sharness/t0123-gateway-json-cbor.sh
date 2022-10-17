#!/usr/bin/env bash

test_description="Test HTTP Gateway DAG-JSON (application/vnd.ipld.dag-json) and DAG-CBOR (application/vnd.ipld.dag-cbor) Support"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon_without_network

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

test_codec_unixfs () {
  name=$1
  format=$2

  test_expect_success "GET UnixFS $name with format=dag-$format has expected Content-Type" '
    curl -sD - "http://127.0.0.1:$GWAY_PORT/ipfs/$FILE_CID?format=dag-$format" > curl_output 2>&1 &&
    test_should_contain "Content-Type: application/vnd.ipld.dag-$format" curl_output &&
    test_should_not_contain "Content-Type: application/$format" curl_output
  '

  test_expect_success "GET UnixFS $name with 'Accept: application/vnd.ipld.dag-$format' has expected Content-Type" '
    curl -sD - -H "Accept: application/vnd.ipld.dag-$format" "http://127.0.0.1:$GWAY_PORT/ipfs/$FILE_CID" > curl_output 2>&1 &&
    test_should_contain "Content-Type: application/vnd.ipld.dag-$format" curl_output &&
    test_should_not_contain "Content-Type: application/$format" curl_output
  '

  test_expect_success "GET UnixFS $name with format=$format has expected Content-Type" '
    curl -sD - "http://127.0.0.1:$GWAY_PORT/ipfs/$FILE_CID?format=$format" > curl_output 2>&1 &&
    test_should_contain "Content-Type: application/$format" curl_output &&
    test_should_not_contain "Content-Type: application/vnd.ipld.dag-$format" curl_output
  '

  test_expect_success "GET UnixFS $name with 'Accept: application/$format' has expected Content-Type" '
    curl -sD - -H "Accept: application/$format" "http://127.0.0.1:$GWAY_PORT/ipfs/$FILE_CID" > curl_output 2>&1 &&
    test_should_contain "Content-Type: application/$format" curl_output &&
    test_should_not_contain "Content-Type: application/vnd.ipld.dag-$format" curl_output
  '

  test_expect_success "GET UnixFS $name has expected output for file" '
    curl -s "http://127.0.0.1:$GWAY_PORT/ipfs/$FILE_CID?format=dag-$format" > curl_output 2>&1 &&
    ipfs dag get --output-codec dag-$format $FILE_CID > ipfs_dag_get_output 2>&1 &&
    test_cmp ipfs_dag_get_output curl_output
  '

  test_expect_success "GET UnixFS $name has expected output for directory" '
    curl -s "http://127.0.0.1:$GWAY_PORT/ipfs/$DIR_CID?format=dag-$format" > curl_output 2>&1 &&
    ipfs dag get --output-codec dag-$format $DIR_CID > ipfs_dag_get_output 2>&1 &&
    test_cmp ipfs_dag_get_output curl_output
  '

  test_expect_success "GET UnixFS $name with format=dag-$format and format=$format produce same output" '
    curl -s "http://127.0.0.1:$GWAY_PORT/ipfs/$DIR_CID?format=dag-$format" > curl_output_1 2>&1 &&
    curl -s "http://127.0.0.1:$GWAY_PORT/ipfs/$DIR_CID?format=$format" > curl_output_2 2>&1 &&
    test_cmp curl_output_1 curl_output_2
  '
}

test_codec_unixfs "DAG-JSON" "json"
test_codec_unixfs "DAG-CBOR" "cbor"

test_codec () {
  name=$1
  format=$2

  test_expect_success "GET $name with format=$format produces correct output" '
    CID=$(echo "{ \"test\": \"json\" }" | ipfs dag put --input-codec json --store-codec $format) &&
    curl -s "http://127.0.0.1:$GWAY_PORT/ipfs/$CID?format=$format" > curl_output 2>&1 &&
    ipfs dag get --output-codec $format $CID > ipfs_dag_get_output 2>&1 &&
    test_cmp ipfs_dag_get_output curl_output
  '

  test_expect_success "GET $name with format=dag-$format produces correct output" '
    CID=$(echo "{ \"test\": \"json\" }" | ipfs dag put --input-codec json --store-codec $format) &&
    curl -s "http://127.0.0.1:$GWAY_PORT/ipfs/$CID?format=dag-$format" > curl_output 2>&1 &&
    ipfs dag get --output-codec dag-$format $CID > ipfs_dag_get_output 2>&1 &&
    test_cmp ipfs_dag_get_output curl_output
  '
}

test_codec "JSON" "json"
test_codec "CBOR" "cbor"

test_kill_ipfs_daemon

test_done