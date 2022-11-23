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

test_headers () {
  name=$1
  format=$2

  test_expect_success "GET UnixFS as $name with format=dag-$format has expected Content-Type" '
    curl -sD - "http://127.0.0.1:$GWAY_PORT/ipfs/$FILE_CID?format=dag-$format" > curl_output 2>&1 &&
    test_should_contain "Content-Type: application/vnd.ipld.dag-$format" curl_output &&
    test_should_contain "Content-Disposition: attachment\; filename=\"${FILE_CID}.${format}\"" curl_output &&
    test_should_not_contain "Content-Type: application/$format" curl_output
  '

  test_expect_success "GET UnixFS as $name with 'Accept: application/vnd.ipld.dag-$format' has expected Content-Type" '
    curl -sD - -H "Accept: application/vnd.ipld.dag-$format" "http://127.0.0.1:$GWAY_PORT/ipfs/$FILE_CID" > curl_output 2>&1 &&
    test_should_contain "Content-Type: application/vnd.ipld.dag-$format" curl_output &&
    test_should_contain "Content-Disposition: attachment\; filename=\"${FILE_CID}.${format}\"" curl_output &&
    test_should_not_contain "Content-Type: application/$format" curl_output
  '

  test_expect_success "GET UnixFS as $name with format=$format has expected Content-Type" '
    curl -sD - "http://127.0.0.1:$GWAY_PORT/ipfs/$FILE_CID?format=$format" > curl_output 2>&1 &&
    test_should_contain "Content-Type: application/$format" curl_output &&
    test_should_contain "Content-Disposition: attachment\; filename=\"${FILE_CID}.${format}\"" curl_output &&
    test_should_not_contain "Content-Type: application/vnd.ipld.dag-$format" curl_output
  '

  test_expect_success "GET UnixFS as $name with 'Accept: application/$format' has expected Content-Type" '
    curl -sD - -H "Accept: application/$format" "http://127.0.0.1:$GWAY_PORT/ipfs/$FILE_CID" > curl_output 2>&1 &&
    test_should_contain "Content-Type: application/$format" curl_output &&
    test_should_contain "Content-Disposition: attachment\; filename=\"${FILE_CID}.${format}\"" curl_output &&
    test_should_not_contain "Content-Type: application/vnd.ipld.dag-$format" curl_output
  '
}

test_headers "DAG-JSON" "json"
test_headers "DAG-CBOR" "cbor"

test_dag_pb () {
  name=$1
  format=$2

  test_expect_success "GET UnixFS as $name has expected output for file" '
    curl -s "http://127.0.0.1:$GWAY_PORT/ipfs/$FILE_CID?format=dag-$format" > curl_output 2>&1 &&
    ipfs dag get --output-codec dag-$format $FILE_CID > ipfs_dag_get_output 2>&1 &&
    test_cmp ipfs_dag_get_output curl_output
  '

  test_expect_success "GET UnixFS as $name has expected output for directory" '
    curl -s "http://127.0.0.1:$GWAY_PORT/ipfs/$DIR_CID?format=dag-$format" > curl_output 2>&1 &&
    ipfs dag get --output-codec dag-$format $DIR_CID > ipfs_dag_get_output 2>&1 &&
    test_cmp ipfs_dag_get_output curl_output
  '

  test_expect_success "GET UnixFS as $name with format=dag-$format and format=$format produce same output" '
    curl -s "http://127.0.0.1:$GWAY_PORT/ipfs/$DIR_CID?format=dag-$format" > curl_output_1 2>&1 &&
    curl -s "http://127.0.0.1:$GWAY_PORT/ipfs/$DIR_CID?format=$format" > curl_output_2 2>&1 &&
    test_cmp curl_output_1 curl_output_2
  '
}

test_dag_pb "DAG-JSON" "json"
test_dag_pb "DAG-CBOR" "cbor"

test_cmp_dag_get () {
  name=$1
  format=$2

  test_expect_success "GET $name without Accept or format= has expected Content-Type" '
    CID=$(echo "{ \"test\": \"json\" }" | ipfs dag put --input-codec json --store-codec $format) &&
    curl -sD - "http://127.0.0.1:$GWAY_PORT/ipfs/$CID" > curl_output 2>&1 &&
    test_should_contain "Content-Disposition: attachment\; filename=\"${CID}.${format}\"" curl_output &&
    test_should_contain "Content-Type: application/$format" curl_output
  '

  test_expect_success "GET $name without Accept or format= produces correct output" '
    CID=$(echo "{ \"test\": \"json\" }" | ipfs dag put --input-codec json --store-codec $format) &&
    curl -s "http://127.0.0.1:$GWAY_PORT/ipfs/$CID" > curl_output 2>&1 &&
    ipfs dag get --output-codec $format $CID > ipfs_dag_get_output 2>&1 &&
    test_cmp ipfs_dag_get_output curl_output
  '

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

test_cmp_dag_get "JSON" "json"
test_cmp_dag_get "CBOR" "cbor"

test_expect_success "GET JSON as CBOR produces DAG-CBOR output" '
  CID=$(echo "{ \"test\": \"json\" }" | ipfs dag put --input-codec json --store-codec json) &&
  curl -s "http://127.0.0.1:$GWAY_PORT/ipfs/$CID?format=cbor" > curl_output 2>&1 &&
  ipfs dag get --output-codec dag-cbor $CID > ipfs_dag_get_output 2>&1 &&
  test_cmp ipfs_dag_get_output curl_output
'

test_expect_success "GET CBOR as JSON produces DAG-JSON output" '
  CID=$(echo "{ \"test\": \"json\" }" | ipfs dag put --input-codec json --store-codec cbor) &&
  curl -s "http://127.0.0.1:$GWAY_PORT/ipfs/$CID?format=json" > curl_output 2>&1 &&
  ipfs dag get --output-codec dag-json $CID > ipfs_dag_get_output 2>&1 &&
  test_cmp ipfs_dag_get_output curl_output
'

DAG_CBOR_TRAVERSAL_CID="bafyreibs4utpgbn7uqegmd2goqz4bkyflre2ek2iwv743fhvylwi4zeeim"
DAG_JSON_TRAVERSAL_CID="baguqeeram5ujjqrwheyaty3w5gdsmoz6vittchvhk723jjqxk7hakxkd47xq"
DAG_PB_CID="bafybeiegxwlgmoh2cny7qlolykdf7aq7g6dlommarldrbm7c4hbckhfcke"

test_expect_success "Add CARs for path traversal and DAG-PB representation tests" '
  ipfs dag import ../t0123-gateway-json-cbor/dag-cbor-traversal.car > import_output &&
  test_should_contain $DAG_CBOR_TRAVERSAL_CID import_output &&
  ipfs dag import ../t0123-gateway-json-cbor/dag-json-traversal.car > import_output &&
  test_should_contain $DAG_JSON_TRAVERSAL_CID import_output &&
  ipfs dag import ../t0123-gateway-json-cbor/dag-pb.car > import_output &&
  test_should_contain $DAG_PB_CID import_output
'

test_expect_success "GET DAG-JSON with Accept: text/html returns HTML" '
  curl -sD - -H "Accept: text/html" "http://127.0.0.1:$GWAY_PORT/ipfs/$DAG_JSON_TRAVERSAL_CID" > curl_output 2>&1 &&
  test_should_not_contain "Content-Disposition: attachment" curl_output &&
  test_should_contain "Content-Type: text/html" curl_output
'

test_expect_success "GET DAG-JSON traversal returns 400 if there is path remainder" '
  curl --head "http://127.0.0.1:$GWAY_PORT/ipfs/$DAG_JSON_TRAVERSAL_CID/foo?format=dag-json" > curl_output 2>&1 &&
  test_should_contain "400 Bad Request" curl_output
'

test_expect_success "GET DAG-JSON traverses multiple links" '
  curl -s "http://127.0.0.1:$GWAY_PORT/ipfs/$DAG_JSON_TRAVERSAL_CID/foo/link/bar?format=dag-json" > curl_output 2>&1 &&
  jq --sort-keys . curl_output > actual &&
  echo "{ \"hello\": \"this is not a link\" }" | jq --sort-keys . > expected &&
  test_cmp expected actual
'

test_expect_success "GET DAG-CBOR with Accept: text/html returns HTML" '
  curl -sD - -H "Accept: text/html" "http://127.0.0.1:$GWAY_PORT/ipfs/$DAG_CBOR_TRAVERSAL_CID" > curl_output 2>&1 &&
  test_should_not_contain "Content-Disposition: attachment" curl_output &&
  test_should_contain "Content-Type: text/html" curl_output
'

test_expect_success "GET DAG-CBOR traversal returns 400 if there is path remainder" '
  curl --head "http://127.0.0.1:$GWAY_PORT/ipfs/$DAG_CBOR_TRAVERSAL_CID/foo?format=dag-cbor" > curl_output 2>&1 &&
  test_should_contain "400 Bad Request" curl_output
'

test_expect_success "GET DAG-CBOR traverses multiple links" '
  curl -s "http://127.0.0.1:$GWAY_PORT/ipfs/$DAG_CBOR_TRAVERSAL_CID/foo/link/bar?format=dag-json" > curl_output 2>&1 &&
  jq --sort-keys . curl_output > actual &&
  echo "{ \"hello\": \"this is not a link\" }" | jq --sort-keys . > expected &&
  test_cmp expected actual
'

# test_expect_success "GET DAG-PB has expected output" '
#   curl -s "http://127.0.0.1:$GWAY_PORT/ipfs/$DAG_PB_CID?format=dag-json" > curl_output 2>&1 &&
#   jq --sort-keys . curl_output > actual &&
#   test_cmp ../t0123-gateway-json-cbor/dag-pb.json actual
# '

test_kill_ipfs_daemon

test_done

