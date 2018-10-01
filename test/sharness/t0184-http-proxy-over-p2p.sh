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
    echo -e "HTTP/1.1 $status_code\nContent-length: $length\n\n$body" | nc -l $WEB_SERVE_PORT &
    REMOTE_SERVER_PID=$!
}


function setup_receiver_ipfs() {
    #
    # setup RECEIVER IPFS daemon
    #
    IPFS_PATH=$(mktemp -d)
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
    ipfs p2p listen /x/test /ip4/127.0.0.1/tcp/$WEB_SERVE_PORT >> $RECEIVER_LOG 2>&1
}


function setup_sender_ipfs() {
    #
    # setup SENDER IPFS daemon
    #
    IPFS_PATH=$(mktemp -d)
    SENDER_LOG=$IPFS_PATH/ipfs.log
    ipfs init >> $SENDER_LOG 2>&1
    ipfs config --json Experimental.Libp2pStreamMounting true >> $SENDER_LOG 2>&1
    ipfs daemon >> $SENDER_LOG 2>&1 &
    SENDER_PID=$!
    sleep 5
}


function teardown_sender_and_receiver() {
    kill -9 $SENDER_PID $RECEIVER_PID > /dev/null 2>&1
    sleep 5
}

function curl_check_response_code() {
    local expected_status_code=$1
    local path_stub=${2:-http/$RECEIVER_ID/test/index.txt}
    local status_code=$(curl -s --write-out %{http_code} --output /dev/null http://localhost:5001/proxy/$path_stub)

    if [[ $status_code -ne $expected_status_code ]];
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
        echo "Found status-code "$STATUS_CODE", expected "$expected_status_code
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


#test_expect_success 'handle proxy http request propogates error response from remote' '
#serve_http_once "SORRY GUYS, I LOST IT" "404 Not Found" &&
#setup_receiver_ipfs &&
#setup_sender_ipfs &&
#curl_send_proxy_request_and_check_response 404 "SORRY GUYS, I LOST IT"
#'
#kill -9 $REMOTE_SERVER_PID
#teardown_sender_and_receiver

test_expect_success 'handle proxy http request when remote server not available ' '
setup_receiver_ipfs &&
setup_sender_ipfs &&
curl_check_response_code "000"
'
teardown_sender_and_receiver

test_expect_success 'handle proxy http request ' '
serve_http_once "THE WOODS ARE LOVELY DARK AND DEEP" &&
setup_receiver_ipfs &&
setup_sender_ipfs &&
curl_send_proxy_request_and_check_response 200 "THE WOODS ARE LOVELY DARK AND DEEP"
'
kill -9 $REMOTE_SERVER_PID
teardown_sender_and_receiver



test_expect_success 'handle proxy http request invalid request' '
setup_receiver_ipfs &&
setup_sender_ipfs &&
curl_check_response_code 404 DERPDERPDERP
'
teardown_sender_and_receiver

test_expect_success 'handle proxy http request unknown proxy peer ' '
setup_receiver_ipfs &&
setup_sender_ipfs &&
curl_check_response_code 400 http/unknown_peer/test/index.txt
'
teardown_sender_and_receiver

test_done
