#!/usr/bin/env bash

test_description="Test HTTP Gateway DAG-JSON (application/vnd.ipld.dag-json) and DAG-CBOR (application/vnd.ipld.dag-cbor) Support"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon_without_network

# Import test case
# See the static fixtures in ./t0123-gateway-json-cbor/
test_expect_success "Add the test directory" '
  ipfs dag import ../t0123-gateway-json-cbor/fixtures.car
'
DIR_CID=bafybeiafyvqlazbbbtjnn6how5d6h6l6rxbqc4qgpbmteaiskjrffmyy4a # ./rootDir
FILE_JSON_CID=bafkreibrppizs3g7axs2jdlnjua6vgpmltv7k72l7v7sa6mmht6mne3qqe # ./rootDir/ą/ę/t.json
FILE_CID=bafkreialihlqnf5uwo4byh4n3cmwlntwqzxxs2fg5vanqdi3d7tb2l5xkm # ./rootDir/ą/ę/file-źł.txt
FILE_SIZE=34

## Quick regression check for JSON stored on UnixFS:
## it has nothing to do with DAG-JSON and JSON codecs,
## but a lot of JSON data is stored on UnixFS and is requested with or without various hints
## and we want to avoid surprises like https://github.com/protocol/bifrost-infra/issues/2290
test_expect_success "GET UnixFS file with JSON bytes is returned with application/json Content-Type" '
  curl -sD headers "http://127.0.0.1:$GWAY_PORT/ipfs/$FILE_JSON_CID" > curl_output 2>&1 &&
  curl -sD headers_accept -H "Accept: application/json" "http://127.0.0.1:$GWAY_PORT/ipfs/$FILE_JSON_CID" > curl_output_accept 2>&1 &&
  ipfs cat $FILE_JSON_CID > ipfs_cat_output 2>&1 &&
  test_should_contain "Content-Type: application/json" headers &&
  test_should_contain "Content-Type: application/json" headers_accept &&
  test_cmp ipfs_cat_output curl_output &&
  test_cmp curl_output curl_output_accept
'


## Reading UnixFS (data encoded with dag-pb codec) as DAG-CBOR and DAG-JSON
## (returns representation defined in https://ipld.io/specs/codecs/dag-pb/spec/#logical-format)

test_dag_pb_conversion () {
  name=$1
  format=$2
  disposition=$3

  test_expect_success "GET UnixFS file as $name with format=dag-$format converts to the expected Content-Type" '
    curl -sD headers "http://127.0.0.1:$GWAY_PORT/ipfs/$FILE_CID?format=dag-$format" > curl_output 2>&1 &&
    ipfs dag get --output-codec dag-$format $FILE_CID > ipfs_dag_get_output 2>&1 &&
    test_cmp ipfs_dag_get_output curl_output &&
    test_should_contain "Content-Type: application/vnd.ipld.dag-$format" headers &&
    test_should_contain "Content-Disposition: ${disposition}\; filename=\"${FILE_CID}.${format}\"" headers &&
    test_should_not_contain "Content-Type: application/$format" headers
  '

  test_expect_success "GET UnixFS directory as $name with format=dag-$format converts to the expected Content-Type" '
    curl -sD headers "http://127.0.0.1:$GWAY_PORT/ipfs/$DIR_CID?format=dag-$format" > curl_output 2>&1 &&
    ipfs dag get --output-codec dag-$format $DIR_CID > ipfs_dag_get_output 2>&1 &&
    test_cmp ipfs_dag_get_output curl_output &&
    test_should_contain "Content-Type: application/vnd.ipld.dag-$format" headers &&
    test_should_contain "Content-Disposition: ${disposition}\; filename=\"${DIR_CID}.${format}\"" headers &&
    test_should_not_contain "Content-Type: application/$format" headers
  '

  test_expect_success "GET UnixFS as $name with 'Accept: application/vnd.ipld.dag-$format' converts to the expected Content-Type" '
    curl -sD - -H "Accept: application/vnd.ipld.dag-$format" "http://127.0.0.1:$GWAY_PORT/ipfs/$FILE_CID" > curl_output 2>&1 &&
    test_should_contain "Content-Disposition: ${disposition}\; filename=\"${FILE_CID}.${format}\"" curl_output &&
    test_should_contain "Content-Type: application/vnd.ipld.dag-$format" curl_output &&
    test_should_not_contain "Content-Type: application/$format" curl_output
  '

  test_expect_success "GET UnixFS as $name with 'Accept: foo, application/vnd.ipld.dag-$format,bar' converts to the expected Content-Type" '
    curl -sD - -H "Accept: foo, application/vnd.ipld.dag-$format,text/plain" "http://127.0.0.1:$GWAY_PORT/ipfs/$FILE_CID" > curl_output 2>&1 &&
    test_should_contain "Content-Type: application/vnd.ipld.dag-$format" curl_output
  '

  test_expect_success "GET UnixFS with format=$format (not dag-$format) is no-op (no conversion)" '
    curl -sD headers "http://127.0.0.1:$GWAY_PORT/ipfs/$FILE_CID?format=$format" > curl_output 2>&1 &&
    ipfs cat $FILE_CID > cat_output &&
    test_cmp cat_output curl_output &&
    test_should_contain "Content-Type: text/plain" headers &&
    test_should_not_contain "Content-Type: application/$format" headers &&
    test_should_not_contain "Content-Type: application/vnd.ipld.dag-$format" headers
  '

  test_expect_success "GET UnixFS with 'Accept: application/$format' (not dag-$format) is no-op (no conversion)" '
    curl -sD headers -H "Accept: application/$format" "http://127.0.0.1:$GWAY_PORT/ipfs/$FILE_CID" > curl_output 2>&1 &&
    ipfs cat $FILE_CID > cat_output &&
    test_cmp cat_output curl_output &&
    test_should_contain "Content-Type: text/plain" headers &&
    test_should_not_contain "Content-Type: application/$format" headers &&
    test_should_not_contain "Content-Type: application/vnd.ipld.dag-$format" headers
  '
}

test_dag_pb_conversion "DAG-JSON" "json" "inline"
test_dag_pb_conversion "DAG-CBOR" "cbor" "attachment"


# Requesting CID with plain json (0x0200) and cbor (0x51) codecs
# (note these are not UnixFS, not DAG-* variants, just raw block identified by a CID with a special codec)
test_plain_codec () {
  name=$1
  format=$2
  disposition=$3

  # no explicit format, just codec in CID
  test_expect_success "GET $name without Accept or format= has expected $format Content-Type and body as-is" '
    CID=$(echo "{ \"test\": \"plain json\" }" | ipfs dag put --input-codec json --store-codec $format) &&
    curl -sD headers "http://127.0.0.1:$GWAY_PORT/ipfs/$CID" > curl_output 2>&1 &&
    ipfs block get $CID > ipfs_block_output 2>&1 &&
    test_cmp ipfs_block_output curl_output &&
    test_should_contain "Content-Disposition: ${disposition}\; filename=\"${CID}.${format}\"" headers &&
    test_should_contain "Content-Type: application/$format" headers
  '

  # explicit format still gives correct output, just codec in CID
  test_expect_success "GET $name with ?format= has expected $format Content-Type and body as-is" '
    CID=$(echo "{ \"test\": \"plain json\" }" | ipfs dag put --input-codec json --store-codec $format) &&
    curl -sD headers "http://127.0.0.1:$GWAY_PORT/ipfs/$CID?format=$format" > curl_output 2>&1 &&
    ipfs block get $CID > ipfs_block_output 2>&1 &&
    test_cmp ipfs_block_output curl_output &&
    test_should_contain "Content-Disposition: ${disposition}\; filename=\"${CID}.${format}\"" headers &&
    test_should_contain "Content-Type: application/$format" headers
  '

  # explicit format still gives correct output, just codec in CID
  test_expect_success "GET $name with Accept has expected $format Content-Type and body as-is" '
    CID=$(echo "{ \"test\": \"plain json\" }" | ipfs dag put --input-codec json --store-codec $format) &&
    curl -sD headers -H "Accept: application/$format" "http://127.0.0.1:$GWAY_PORT/ipfs/$CID" > curl_output 2>&1 &&
    ipfs block get $CID > ipfs_block_output 2>&1 &&
    test_cmp ipfs_block_output curl_output &&
    test_should_contain "Content-Disposition: ${disposition}\; filename=\"${CID}.${format}\"" headers &&
    test_should_contain "Content-Type: application/$format" headers
  '

  # explicit dag-* format passed, attempt to parse as dag* variant
  ## Note: this works only for simple JSON that can be upgraded to  DAG-JSON.
  test_expect_success "GET $name with format=dag-$format interprets $format as dag-* variant and produces expected Content-Type and body" '
    CID=$(echo "{ \"test\": \"plain-json-that-can-also-be-dag-json\" }" | ipfs dag put --input-codec json --store-codec $format) &&
    curl -sD headers "http://127.0.0.1:$GWAY_PORT/ipfs/$CID?format=dag-$format" > curl_output_param 2>&1 &&
    ipfs dag get --output-codec dag-$format $CID > ipfs_dag_get_output 2>&1 &&
    test_cmp ipfs_dag_get_output curl_output_param &&
    test_should_contain "Content-Disposition: ${disposition}\; filename=\"${CID}.${format}\"" headers &&
    test_should_contain "Content-Type: application/vnd.ipld.dag-$format" headers &&
    curl -s -H "Accept: application/vnd.ipld.dag-$format" "http://127.0.0.1:$GWAY_PORT/ipfs/$CID" > curl_output_accept 2>&1 &&
    test_cmp curl_output_param curl_output_accept
  '

}

test_plain_codec "plain JSON codec" "json" "inline"
test_plain_codec "plain CBOR codec" "cbor" "attachment"

## Pathing, traversal over DAG-JSON and DAG-CBOR

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

test_expect_success "GET DAG-JSON traversal returns 501 if there is path remainder" '
  curl -sD - "http://127.0.0.1:$GWAY_PORT/ipfs/$DAG_JSON_TRAVERSAL_CID/foo?format=dag-json" > curl_output 2>&1 &&
  test_should_contain "501 Not Implemented" curl_output &&
  test_should_contain "reading IPLD Kinds other than Links (CBOR Tag 42) is not implemented" curl_output
'

test_expect_success "GET DAG-JSON traverses multiple links" '
  curl -s "http://127.0.0.1:$GWAY_PORT/ipfs/$DAG_JSON_TRAVERSAL_CID/foo/link/bar?format=dag-json" > curl_output 2>&1 &&
  jq --sort-keys . curl_output > actual &&
  echo "{ \"hello\": \"this is not a link\" }" | jq --sort-keys . > expected &&
  test_cmp expected actual
'

test_expect_success "GET DAG-CBOR traversal returns 501 if there is path remainder" '
  curl -sD - "http://127.0.0.1:$GWAY_PORT/ipfs/$DAG_CBOR_TRAVERSAL_CID/foo?format=dag-cbor" > curl_output 2>&1 &&
  test_should_contain "501 Not Implemented" curl_output &&
  test_should_contain "reading IPLD Kinds other than Links (CBOR Tag 42) is not implemented" curl_output
'

test_expect_success "GET DAG-CBOR traverses multiple links" '
  curl -s "http://127.0.0.1:$GWAY_PORT/ipfs/$DAG_CBOR_TRAVERSAL_CID/foo/link/bar?format=dag-json" > curl_output 2>&1 &&
  jq --sort-keys . curl_output > actual &&
  echo "{ \"hello\": \"this is not a link\" }" | jq --sort-keys . > expected &&
  test_cmp expected actual
'

## NATIVE TESTS for DAG-JSON (0x0129) and DAG-CBOR (0x71):
## DAG- regression tests for core behaviors when native DAG-(CBOR|JSON) is requested

test_native_dag () {
  name=$1
  format=$2
  disposition=$3
  CID=$4

  # GET without explicit format and Accept: text/html returns raw block

    test_expect_success "GET $name from /ipfs without explicit format returns the same payload as the raw block" '
    ipfs block get "/ipfs/$CID" > expected &&
    curl -sX GET "http://127.0.0.1:$GWAY_PORT/ipfs/$CID" -o curl_output &&
    test_cmp expected curl_output
    '

  # GET dag-cbor block via Accept and ?format and ensure both are the same as `ipfs block get` output

    test_expect_success "GET $name from /ipfs with format=dag-$format returns the same payload as the raw block" '
    ipfs block get "/ipfs/$CID" > expected &&
    curl -sX GET "http://127.0.0.1:$GWAY_PORT/ipfs/$CID?format=dag-$format" -o curl_ipfs_dag_param_output &&
    test_cmp expected curl_ipfs_dag_param_output
    '

    test_expect_success "GET $name from /ipfs for application/$format returns the same payload as format=dag-$format" '
    curl -sX GET "http://127.0.0.1:$GWAY_PORT/ipfs/$CID?format=dag-$format" -o expected &&
    curl -sX GET "http://127.0.0.1:$GWAY_PORT/ipfs/$CID?format=$format" -o plain_output &&
    test_cmp expected plain_output
    '

    test_expect_success "GET $name from /ipfs with application/vnd.ipld.dag-$format returns the same payload as the raw block" '
    ipfs block get "/ipfs/$CID" > expected_block &&
    curl -sX GET -H "Accept: application/vnd.ipld.dag-$format" "http://127.0.0.1:$GWAY_PORT/ipfs/$CID" -o curl_ipfs_dag_block_accept_output &&
    test_cmp expected_block curl_ipfs_dag_block_accept_output
    '

  # Make sure DAG-* can be requested as plain JSON or CBOR and response has plain Content-Type for interop purposes

    test_expect_success "GET $name with format=$format returns same payload as format=dag-$format but with plain Content-Type" '
    curl -s "http://127.0.0.1:$GWAY_PORT/ipfs/$CID?format=dag-$format" -o expected &&
    curl -sD plain_headers "http://127.0.0.1:$GWAY_PORT/ipfs/$CID?format=$format" -o plain_output &&
    test_should_contain "Content-Type: application/$format" plain_headers &&
    test_cmp expected plain_output
    '

    test_expect_success "GET $name with Accept: application/$format returns same payload as application/vnd.ipld.dag-$format but with plain Content-Type" '
    curl -s -H "Accept: application/vnd.ipld.dag-$format" "http://127.0.0.1:$GWAY_PORT/ipfs/$CID" > expected &&
    curl -sD plain_headers -H "Accept: application/$format" "http://127.0.0.1:$GWAY_PORT/ipfs/$CID" > plain_output &&
    test_should_contain "Content-Type: application/$format" plain_headers &&
    test_cmp expected plain_output
    '


  # Make sure expected HTTP headers are returned with the dag- block

    test_expect_success "GET response for application/vnd.ipld.dag-$format has expected Content-Type" '
    curl -svX GET -H "Accept: application/vnd.ipld.dag-$format" "http://127.0.0.1:$GWAY_PORT/ipfs/$CID" >/dev/null 2>curl_output &&
    test_should_contain "< Content-Type: application/vnd.ipld.dag-$format" curl_output
    '

    test_expect_success "GET response for application/vnd.ipld.dag-$format includes Content-Length" '
    BYTES=$(ipfs block get $CID | wc --bytes)
    test_should_contain "< Content-Length: $BYTES" curl_output
    '

    test_expect_success "GET response for application/vnd.ipld.dag-$format includes Content-Disposition" '
    test_should_contain "< Content-Disposition: ${disposition}\; filename=\"${CID}.${format}\"" curl_output
    '

    test_expect_success "GET response for application/vnd.ipld.dag-$format includes nosniff hint" '
    test_should_contain "< X-Content-Type-Options: nosniff" curl_output
    '

    test_expect_success "GET for application/vnd.ipld.dag-$format with query filename includes Content-Disposition with custom filename" '
    curl -svX GET -H "Accept: application/vnd.ipld.dag-$format" "http://127.0.0.1:$GWAY_PORT/ipfs/$CID?filename=foobar.$format" >/dev/null 2>curl_output_filename &&
    test_should_contain "< Content-Disposition: ${disposition}\; filename=\"foobar.$format\"" curl_output_filename
    '

    test_expect_success "GET for application/vnd.ipld.dag-$format with ?download=true forces Content-Disposition: attachment" '
    curl -svX GET -H "Accept: application/vnd.ipld.dag-$format" "http://127.0.0.1:$GWAY_PORT/ipfs/$CID?filename=foobar.$format&download=true" >/dev/null 2>curl_output_filename &&
    test_should_contain "< Content-Disposition: attachment" curl_output_filename
    '

  # Cache control HTTP headers
  # (basic checks, detailed behavior is tested in  t0116-gateway-cache.sh)

    test_expect_success "GET response for application/vnd.ipld.dag-$format includes Etag" '
    test_should_contain "< Etag: \"${CID}.dag-$format\"" curl_output
    '

    test_expect_success "GET response for application/vnd.ipld.dag-$format includes X-Ipfs-Path and X-Ipfs-Roots" '
    test_should_contain "< X-Ipfs-Path" curl_output &&
    test_should_contain "< X-Ipfs-Roots" curl_output
    '

    test_expect_success "GET response for application/vnd.ipld.dag-$format includes Cache-Control" '
    test_should_contain "< Cache-Control: public, max-age=29030400, immutable" curl_output
    '

  # HTTP HEAD behavior
  test_expect_success "HEAD $name with no explicit format returns HTTP 200" '
    curl -I "http://127.0.0.1:$GWAY_PORT/ipfs/$CID" -o output &&
    test_should_contain "HTTP/1.1 200 OK" output &&
    test_should_contain "Content-Type: application/vnd.ipld.dag-$format" output &&
    test_should_contain "Content-Length: " output
  '
  test_expect_success "HEAD $name with an explicit DAG-JSON format returns HTTP 200" '
    curl -I "http://127.0.0.1:$GWAY_PORT/ipfs/$CID?format=dag-json" -o output &&
    test_should_contain "HTTP/1.1 200 OK" output &&
    test_should_contain "Etag: \"$CID.dag-json\"" output &&
    test_should_contain "Content-Type: application/vnd.ipld.dag-json" output &&
    test_should_contain "Content-Length: " output
  '
  test_expect_success "HEAD $name with only-if-cached for missing block returns HTTP 412 Precondition Failed" '
    MISSING_CID=$(echo "{\"t\": \"$(date +%s)\"}" | ipfs dag put --store-codec=dag-${format}) &&
    ipfs block rm -f -q $MISSING_CID &&
    curl -I -H "Cache-Control: only-if-cached" "http://127.0.0.1:$GWAY_PORT/ipfs/$MISSING_CID" -o output &&
    test_should_contain "HTTP/1.1 412 Precondition Failed" output
  '

  # IPNS behavior (should be same as immutable /ipfs, but with different caching headers)
  # To keep tests small we only confirm payload is the same, and then only test delta around caching headers.

  test_expect_success "Prepare IPNS with dag-$format" '
    IPNS_ID=$(ipfs key gen --ipns-base=base36 --type=ed25519 ${format}_test_key | head -n1 | tr -d "\n") &&
    ipfs name publish --key ${format}_test_key --allow-offline -Q "/ipfs/$CID" > name_publish_out &&
    test_check_peerid "${IPNS_ID}" &&
    ipfs name resolve "${IPNS_ID}" > output &&
    printf "/ipfs/%s\n" "$CID" > expected &&
    test_cmp expected output
  '

  test_expect_success "GET $name from /ipns without explicit format returns the same payload as /ipfs" '
    curl -sX GET "http://127.0.0.1:$GWAY_PORT/ipfs/$CID" -o ipfs_output &&
    curl -sX GET "http://127.0.0.1:$GWAY_PORT/ipns/$IPNS_ID" -o ipns_output &&
    test_cmp ipfs_output ipns_output
  '

  test_expect_success "GET $name from /ipns without explicit format returns the same payload as /ipfs" '
    curl -sX GET "http://127.0.0.1:$GWAY_PORT/ipfs/$CID?format=dag-$format" -o ipfs_output &&
    curl -sX GET "http://127.0.0.1:$GWAY_PORT/ipns/$IPNS_ID?format=dag-$format" -o ipns_output &&
    test_cmp ipfs_output ipns_output
  '

  test_expect_success "GET $name from /ipns with explicit application/vnd.ipld.dag-$format has expected headers" '
    curl -svX GET -H "Accept: application/vnd.ipld.dag-$format" "http://127.0.0.1:$GWAY_PORT/ipns/$IPNS_ID" >/dev/null 2>curl_output &&
    test_should_not_contain "Cache-Control" curl_output &&
    test_should_contain "< Content-Type: application/vnd.ipld.dag-$format" curl_output &&
    test_should_contain "< Etag: \"${CID}.dag-$format\"" curl_output &&
    test_should_contain "< X-Ipfs-Path" curl_output &&
    test_should_contain "< X-Ipfs-Roots" curl_output
  '


  # When Accept header includes text/html and no explicit format is requested for DAG-(CBOR|JSON)
  # The gateway returns generated HTML index (see dag-index-html) for web browsers (similar to dir-index-html returned for unixfs dirs)
  # As this is generated, we don't return immutable Cache-Control, even on /ipfs (same as for dir-index-html)

  test_expect_success "GET $name on /ipfs with Accept: text/html returns HTML (dag-index-html)" '
    curl -sD - -H "Accept: text/html" "http://127.0.0.1:$GWAY_PORT/ipfs/$CID" > curl_output 2>&1 &&
    test_should_not_contain "Content-Disposition" curl_output &&
    test_should_not_contain "Cache-Control" curl_output &&
    test_should_contain "Etag: \"DagIndex-" curl_output &&
    test_should_contain "Content-Type: text/html" curl_output &&
    test_should_contain "</html>" curl_output
  '

  test_expect_success "GET $name on /ipns with Accept: text/html returns HTML (dag-index-html)" '
    curl -sD - -H "Accept: text/html" "http://127.0.0.1:$GWAY_PORT/ipns/$IPNS_ID" > curl_output 2>&1 &&
    test_should_not_contain "Content-Disposition" curl_output &&
    test_should_not_contain "Cache-Control" curl_output &&
    test_should_contain "Etag: \"DagIndex-" curl_output &&
    test_should_contain "Content-Type: text/html" curl_output &&
    test_should_contain "</html>" curl_output
  '


}

test_native_dag "DAG-JSON" "json" "inline" "$DAG_JSON_TRAVERSAL_CID"
test_native_dag "DAG-CBOR" "cbor" "attachment" "$DAG_CBOR_TRAVERSAL_CID"

test_kill_ipfs_daemon

test_done

# vim: set ts=2 sw=2 et:
