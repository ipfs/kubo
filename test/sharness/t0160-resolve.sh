#!/bin/sh

test_description="Test resolve command"

. lib/test-lib.sh

test_init_ipfs

test_expect_success "resolve: prepare files" '
	mkdir -p a/b &&
	echo "a/b/c" >a/b/c &&
	a_hash=$(ipfs add -q -r a | tail -n1) &&
	b_hash=$(ipfs add -q -r a/b | tail -n1) &&
	c_hash=$(ipfs add -q -r a/b/c | tail -n1)
'

test_name_publish() {
	ref="$1"; shift
	publish_args="$(shellquote "$@")"

	test_expect_success_1941 "resolve: name publish $ref${1:+ ($publish_args)}" '
		id_hash=$(ipfs id -f="<id>") &&
		ipfs name publish '"$publish_args"' "$ref"
	'
}

test_name_resolve() {
	ref="$1"

	test_expect_success_1941 "resolve: name resolve $ref" '
		printf "%s" "$ref" >expected_nameval &&
		ipfs name resolve >actual_nameval &&
		test_cmp expected_nameval actual_nameval
	'
}

test_resolve() {
	src="$1"
	dst="$2"

	# Just use test_expect_success once #1941 is fixed.
	case "$src" in
	  /ipns/*) test_cmd=test_expect_success_1941 ;;
	  *)       test_cmd=test_expect_success ;;
	esac

	"$test_cmd" "resolve succeeds: $src" '
		ipfs resolve -r "$src" >actual
	'

	"$test_cmd" "resolved correctly: $src -> $dst" '
		printf "%s" "$dst" >expected &&
		test_cmp expected actual
	'
}

# Any parameters are passed to ipfs name publish.
test_common() {
	test_resolve "/ipfs/$a_hash" "/ipfs/$a_hash"
	test_resolve "/ipfs/$a_hash/b" "/ipfs/$b_hash"
	test_resolve "/ipfs/$a_hash/b/c" "/ipfs/$c_hash"
	test_resolve "/ipfs/$b_hash/c" "/ipfs/$c_hash"

	test_name_publish "/ipfs/$a_hash" "$@"
	test_name_resolve "/ipfs/$a_hash"
	test_resolve "/ipns/$id_hash" "/ipfs/$a_hash"
	test_resolve "/ipns/$id_hash/b" "/ipfs/$b_hash"
	test_resolve "/ipns/$id_hash/b/c" "/ipfs/$c_hash"

	test_name_publish "/ipfs/$b_hash" "$@"
	test_name_resolve "/ipfs/$b_hash"
	test_resolve "/ipns/$id_hash" "/ipfs/$b_hash"
	test_resolve "/ipns/$id_hash/c" "/ipfs/$c_hash"

	test_name_publish "/ipfs/$c_hash" "$@"
	test_name_resolve "/ipfs/$c_hash"
	test_resolve "/ipns/$id_hash" "/ipfs/$c_hash"
}

# should work offline
test_common --ttl=0s

# should work offline with non-zero TTL; cache should not be in effect.
test_common # Default TTL
test_common --ttl=1h

# should work online
test_launch_ipfs_daemon
test_common --ttl=0s

# The following tests test the caching by publishing a new name and expecting
# the previous resolve result to stay cached.  The first test_(name_)resolve
# after a test_name_publish will generate the cache entry and any subsequent
# test_resolve invocations will use that cache entry until the expiry timer
# finishes (test_timer_wait).

TTL=10

test_name_publish "/ipfs/$a_hash" --ttl="${TTL}s"
test_name_resolve "/ipfs/$a_hash"

# The cache entry has expired when this finishes.
test_timer_start EXPIRY_TIMER "${TTL}s"
# Publish a new version now, the previous version should still be cached.
test_name_publish "/ipfs/$b_hash" --ttl="${TTL}s"

test_resolve "/ipns/$id_hash" "/ipfs/$a_hash"
test_resolve "/ipns/$id_hash/b" "/ipfs/$b_hash"
test_resolve "/ipns/$id_hash/b/c" "/ipfs/$c_hash"

# Make sure the expiry timer is still running, otherwise the result might be
# wrong.  If this fails, we will need to increase TTL above to give enough time
# for the tests.
test_expect_success "tests did not take too long" '
	test_timer_is_running "$EXPIRY_TIMER"
'
test_expect_success "previous version is no longer cached" '
	test_timer_wait "$EXPIRY_TIMER"
'

test_name_resolve "/ipfs/$b_hash"

test_timer_start EXPIRY_TIMER "${TTL}s"
# Publish a new version now, the previous version should still be cached.
test_name_publish "/ipfs/$c_hash" # Default TTL for the final one.

test_resolve "/ipns/$id_hash" "/ipfs/$b_hash"
test_resolve "/ipns/$id_hash/c" "/ipfs/$c_hash"

test_expect_success "tests did not take too long" '
	test_timer_is_running "$EXPIRY_TIMER"
'
test_expect_success "previous version is no longer cached" '
	test_timer_wait "$EXPIRY_TIMER"
'

test_name_resolve "/ipfs/$c_hash"
test_resolve "/ipns/$id_hash" "/ipfs/$c_hash"

test_kill_ipfs_daemon

test_done
