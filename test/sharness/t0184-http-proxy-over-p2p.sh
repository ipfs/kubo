#!/usr/bin/env bash

test_description="Test http proxy over p2p"

. lib/test-lib.sh

if ! test_have_prereq SOCAT; then
  skip_all="skipping '$test_description': socat is not available"
  test_done
fi

WEB_SERVE_PORT=5099
IPFS_GATEWAY_PORT=5199
SENDER_GATEWAY="http://127.0.0.1:$IPFS_GATEWAY_PORT"

function show_logs() {

    echo "*****************"
    echo "  RECEIVER LOG  "
    echo "*****************"
    iptb logs 1
    echo "*****************"
    echo "  SENDER LOG  "
    echo "*****************"
    iptb logs 0
    echo "*****************"
    echo "REMOTE_SERVER LOG"
    echo $REMOTE_SERVER_LOG
    echo "*****************"
    cat $REMOTE_SERVER_LOG
}

function start_http_server() {
    REMOTE_SERVER_LOG="server.log"
    rm -f $REMOTE_SERVER_LOG

    touch response
    socat tcp-listen:$WEB_SERVE_PORT,fork,bind=127.0.0.1,reuseaddr 'SYSTEM:cat response'!!CREATE:$REMOTE_SERVER_LOG &
    REMOTE_SERVER_PID=$!

    socat /dev/null tcp:127.0.01:$WEB_SERVE_PORT,retry=10
    return $?
}

function teardown_remote_server() {
    exec 7<&-
    kill $REMOTE_SERVER_PID > /dev/null 2>&1
    wait $REMOTE_SERVER_PID || true
}

function serve_content() {
    local body=$1
    local status_code=${2:-"200 OK"}
    local length=$((1 + ${#body}))
    echo -e "HTTP/1.1 $status_code\nContent-length: $length\n\n$body" > response
}

function curl_check_response_code() {
    local expected_status_code=$1
    local path_stub=${2:-p2p/$RECEIVER_ID/http/index.txt}
    local status_code=$(curl -s --write-out %{http_code} --output /dev/null $SENDER_GATEWAY/$path_stub)

    if [[ "$status_code" -ne "$expected_status_code" ]];
    then
        echo "Found status-code "$status_code", expected "$expected_status_code
        return 1
    fi

    return 0
}

function curl_send_proxy_request_and_check_response() {
    local expected_status_code=$1
    local expected_content=$2

    #
    # make a request to SENDER_IPFS via the proxy endpoint
    #
    CONTENT_PATH="retrieved-file"
    STATUS_CODE="$(curl -s -o $CONTENT_PATH --write-out %{http_code} $SENDER_GATEWAY/p2p/$RECEIVER_ID/http/index.txt)"

    #
    # check status code
    #
    if [[ "$STATUS_CODE" -ne "$expected_status_code" ]];
    then
        echo -e "Found status-code "$STATUS_CODE", expected "$expected_status_code
        show_logs
        return 1
    fi

    #
    # check content
    #
    RESPONSE_CONTENT="$(tail -n 1 $CONTENT_PATH)"
    if [[ "$RESPONSE_CONTENT" == "$expected_content" ]];
    then
        return 0
    else
        echo -e "Found response content:\n'"$RESPONSE_CONTENT"'\nthat differs from expected content:\n'"$expected_content"'"
        return 1
    fi
}

function curl_send_multipart_form_request() {
    local expected_status_code=$1
    local FILE_PATH="uploaded-file"
    FILE_CONTENT="curl will send a multipart-form POST request when sending a file which is handy"
    echo $FILE_CONTENT > $FILE_PATH
    #
    # send multipart form request
    #
    STATUS_CODE="$(curl -o /dev/null -s -F file=@$FILE_PATH --write-out %{http_code} $SENDER_GATEWAY/p2p/$RECEIVER_ID/http/index.txt)"
    #
    # check status code
    #
    if [[ "$STATUS_CODE" -ne "$expected_status_code" ]];
    then
        echo -e "Found status-code "$STATUS_CODE", expected "$expected_status_code
        return 1
    fi
    #
    # check request method
    #
    if ! grep "POST /index.txt" $REMOTE_SERVER_LOG > /dev/null;
    then
        echo "Remote server request method/resource path was incorrect"
        show_logs
        return 1
    fi
    #
    # check request is multipart-form
    #
    if ! grep "Content-Type: multipart/form-data;" $REMOTE_SERVER_LOG > /dev/null;
    then
        echo "Request content-type was not multipart/form-data"
        show_logs
        return 1
    fi
    return 0
}

test_expect_success 'configure nodes' '
    iptb testbed create -type localipfs -count 2 -force -init &&
    iptb run -- ipfs config --json "Routing.LoopbackAddressesOnLanDHT" true &&
    ipfsi 0 config --json Experimental.Libp2pStreamMounting true &&
    ipfsi 1 config --json Experimental.Libp2pStreamMounting true &&
    ipfsi 0 config --json Experimental.P2pHttpProxy true &&
    ipfsi 0 config --json Addresses.Gateway "[\"/ip4/127.0.0.1/tcp/$IPFS_GATEWAY_PORT\"]"
'

test_expect_success 'configure a subdomain gateway with /p2p/ path whitelisted' "
    ipfsi 0 config --json Gateway.PublicGateways '{
        \"example.com\": {
            \"UseSubdomains\": true,
            \"Paths\": [\"/p2p/\"]
        }
    }'
"

test_expect_success 'start and connect nodes' '
    iptb start -wait && iptb connect 0 1
'

test_expect_success 'setup p2p listener on the receiver' '
    ipfsi 1 p2p listen --allow-custom-protocol /http /ip4/127.0.0.1/tcp/$WEB_SERVE_PORT &&
    ipfsi 1 p2p listen /x/custom/http /ip4/127.0.0.1/tcp/$WEB_SERVE_PORT
'

test_expect_success 'setup environment' '
    RECEIVER_ID=$(ipfsi 1 id -f="<id>" --peerid-base=b58mh)
    RECEIVER_ID_CIDv1=$(ipfsi 1 id -f="<id>" --peerid-base=base36)
'

test_expect_success 'handle proxy http request sends bad-gateway when remote server not available ' '
    curl_send_proxy_request_and_check_response 502 ""
'

test_expect_success 'start http server' '
    start_http_server
'

test_expect_success 'handle proxy http request propagates error response from remote' '
    serve_content "SORRY GUYS, I LOST IT" "404 Not Found" &&
    curl_send_proxy_request_and_check_response 404 "SORRY GUYS, I LOST IT"
'

test_expect_success 'handle proxy http request ' '
    serve_content "THE WOODS ARE LOVELY DARK AND DEEP" &&
    curl_send_proxy_request_and_check_response 200 "THE WOODS ARE LOVELY DARK AND DEEP"
'

test_expect_success 'handle proxy http request invalid request' '
    curl_check_response_code 400 p2p/DERPDERPDERP
'

test_expect_success 'handle proxy http request unknown proxy peer ' '
    UNKNOWN_PEER="k51qzi5uqu5dlmbel1sd8rs4emr3bfosk9bm4eb42514r4lakt4oxw3a3fa2tm" &&
    curl_check_response_code 502 p2p/$UNKNOWN_PEER/http/index.txt
'

test_expect_success 'handle proxy http request to invalid proxy peer ' '
    curl_check_response_code 400 p2p/invalid_peer/http/index.txt
'

test_expect_success 'handle proxy http request to custom protocol' '
    serve_content "THE WOODS ARE LOVELY DARK AND DEEP" &&
    curl_check_response_code 200 p2p/$RECEIVER_ID/x/custom/http/index.txt
'

test_expect_success 'handle proxy http request to missing protocol' '
    serve_content "THE WOODS ARE LOVELY DARK AND DEEP" &&
    curl_check_response_code 502 p2p/$RECEIVER_ID/x/missing/http/index.txt
'

test_expect_success 'handle proxy http request missing the /http' '
    curl_check_response_code 400 p2p/$RECEIVER_ID/x/custom/index.txt
'

test_expect_success 'handle multipart/form-data http request' '
    serve_content "OK" &&
    curl_send_multipart_form_request 200
'

# OK: $peerid.p2p.example.com/http/index.txt
test_expect_success "handle http request to a subdomain gateway" '
  serve_content "SUBDOMAIN PROVIDES ORIGIN ISOLATION PER RECEIVER_ID" &&
  curl -H "Host: $RECEIVER_ID_CIDv1.p2p.example.com" -sD - $SENDER_GATEWAY/http/index.txt > p2p_response &&
  test_should_contain "SUBDOMAIN PROVIDES ORIGIN ISOLATION PER RECEIVER_ID" p2p_response
'

# FAIL: $peerid.p2p.example.com/p2p/$peerid/http/index.txt
test_expect_success "handle invalid http request to a subdomain gateway" '
  serve_content "SUBDOMAIN DOES NOT SUPPORT FULL /p2p/ PATH" &&
  curl -H "Host: $RECEIVER_ID_CIDv1.p2p.example.com" -sD - $SENDER_GATEWAY/p2p/$RECEIVER_ID/http/index.txt > p2p_response &&
  test_should_contain "400 Bad Request" p2p_response
'

# REDIRECT: example.com/p2p/$peerid/http/index.txt → $peerid.p2p.example.com/http/index.txt
test_expect_success "redirect http path request to subdomain gateway" '
  serve_content "SUBDOMAIN ROOT REDIRECTS /p2p/ PATH TO SUBDOMAIN" &&
  curl -H "Host: example.com" -sD - $SENDER_GATEWAY/p2p/$RECEIVER_ID/http/index.txt > p2p_response &&
  test_should_contain "Location: http://$RECEIVER_ID_CIDv1.p2p.example.com/http/index.txt" p2p_response
'

test_expect_success 'stop http server' '
    teardown_remote_server
'

test_expect_success 'stop nodes' '
    iptb stop
'

test_done
