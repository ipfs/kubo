#!/usr/bin/env bash

test_description="Test http proxy over p2p"

. lib/test-lib.sh
WEB_SERVE_PORT=5099

function serve_http_once() {
    #
    # one shot http server (via nc) with static body
    #
    local body=$1
    local status_code=${2:-"200 OK"}
    local length=$(expr 1 + ${#body})
    REMOTE_SERVER_LOG=$(mktemp)
    echo -e "HTTP/1.1 $status_code\nContent-length: $length\n\n$body" | nc -l $WEB_SERVE_PORT > $REMOTE_SERVER_LOG &
    REMOTE_SERVER_PID=$!
}


function setup_receiver_ipfs() {
    #
    # setup RECEIVER IPFS daemon
    #
    local IPFS_PATH=$(mktemp -d)
    RECEIVER_LOG=$IPFS_PATH/ipfs.log

    ipfs init >> $RECEIVER_LOG 2>&1
    ipfs config --json Experimental.Libp2pStreamMounting true >> $RECEIVER_LOG 2>&1
    ipfs config --json Addresses.API "\"/ip4/127.0.0.1/tcp/6001\"" >> $RECEIVER_LOG 2>&1
    ipfs config --json Addresses.Gateway "\"/ip4/127.0.0.1/tcp/8081\"" >> $RECEIVER_LOG 2>&1
    ipfs config --json Addresses.Swarm "[\"/ip4/0.0.0.0/tcp/7001\", \"/ip6/::/tcp/7001\"]" >> $RECEIVER_LOG 2>&1
    ipfs daemon >> $RECEIVER_LOG 2>&1 &
    RECEIVER_PID=$!
    # wait for daemon to start.. maybe?
    # ipfs id returns empty string if we don't wait here..
    sleep 5
    RECEIVER_ID=$(ipfs id -f "<id>")
    #
    # start a p2p listener on RECIVER to the HTTP server with our content
    #
    ipfs p2p listen --allow-custom-protocol test /ip4/127.0.0.1/tcp/$WEB_SERVE_PORT >> $RECEIVER_LOG 2>&1
}


function setup_sender_ipfs() {
    #
    # setup SENDER IPFS daemon
    #
    local IPFS_PATH=$(mktemp -d)
    SENDER_LOG=$IPFS_PATH/ipfs.log
    ipfs init >> $SENDER_LOG 2>&1
    ipfs config --json Experimental.Libp2pStreamMounting true >> $SENDER_LOG 2>&1
    ipfs config --json Experimental.P2pHttpProxy true >> $RECEIVER_LOG 2>&1
    ipfs daemon >> $SENDER_LOG 2>&1 &
    SENDER_PID=$!
    sleep 5
}

function setup_sender_and_receiver_ipfs() {
    setup_receiver_ipfs && setup_sender_ipfs
}

function teardown_sender_and_receiver() {
    kill -9 $SENDER_PID $RECEIVER_PID > /dev/null 2>&1
    sleep 5
}

function teardown_remote_server() {
    kill -9 $REMOTE_SERVER_PID > /dev/null 2>&1
    sleep 5
}

function curl_check_response_code() {
    local expected_status_code=$1
    local path_stub=${2:-http/$RECEIVER_ID/test/index.txt}
    local status_code=$(curl -s --write-out %{http_code} --output /dev/null http://localhost:5001/proxy/http/$path_stub)

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
    CONTENT_PATH=$(mktemp)
    STATUS_CODE=$(curl -s -o $CONTENT_PATH --write-out %{http_code} http://localhost:5001/proxy/http/$RECEIVER_ID/test/index.txt)

    #
    # check status code
    #
    if [[ $STATUS_CODE -ne $expected_status_code ]];
    then
        echo -e "Found status-code "$STATUS_CODE", expected "$expected_status_code
        return 1
    fi

    #
    # check content
    #
    RESPONSE_CONTENT=$(tail -n 1 $CONTENT_PATH)
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
    local FILE_PATH=$(mktemp)
    FILE_CONTENT="curl will send a multipart-form POST request when sending a file which is handy"
    echo $FILE_CONTENT > $FILE_PATH
    #
    # send multipart form request
    #
    STATUS_CODE=$(curl -s -F file=@$FILE_PATH  http://localhost:5001/proxy/http/$RECEIVER_ID/test/index.txt)
    #
    # check status code
    #
    if [[ $STATUS_CODE -ne $expected_status_code ]];
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
        return 1
    fi
    #
    # check content received
    #
    if ! grep "$FILE_CONTENT" $REMOTE_SERVER_LOG > /dev/null;
    then
        echo "form-data-content was not correct"
        return 1
    fi
    #
    # check request is multipart-form
    #
    if ! grep "Content-Type: multipart/form-data;" $REMOTE_SERVER_LOG > /dev/null;
    then
        echo "Request content-type was not multipart/form-data"
        return 1
    fi
    return 0
}

teardown_sender_and_receiver
test_expect_success 'handle proxy http request propogates error response from remote' '
serve_http_once "SORRY GUYS, I LOST IT" "404 Not Found" &&
    setup_sender_and_receiver_ipfs &&
    curl_send_proxy_request_and_check_response 404 "SORRY GUYS, I LOST IT"
'
teardown_sender_and_receiver
teardown_remote_server

test_expect_success 'handle proxy http request sends bad-gateway when remote server not available ' '
setup_sender_and_receiver_ipfs &&
    curl_send_proxy_request_and_check_response 502 ""
'
teardown_sender_and_receiver

test_expect_success 'handle proxy http request ' '
serve_http_once "THE WOODS ARE LOVELY DARK AND DEEP" &&
    setup_sender_and_receiver_ipfs &&
    curl_send_proxy_request_and_check_response 200 "THE WOODS ARE LOVELY DARK AND DEEP"
'
teardown_sender_and_receiver

test_expect_success 'handle proxy http request invalid request' '
setup_sender_and_receiver_ipfs &&
    curl_check_response_code 400 DERPDERPDERP
'
teardown_sender_and_receiver

test_expect_success 'handle proxy http request unknown proxy peer ' '
setup_sender_and_receiver_ipfs &&
    curl_check_response_code 502 unknown_peer/test/index.txt
'
teardown_sender_and_receiver

test_expect_success 'handle multipart/form-data http request' '
serve_http_once "OK" &&
setup_sender_and_receiver_ipfs &&
curl_send_multipart_form_request
'
teardown_sender_and_receiver
teardown_remote_server


test_done
