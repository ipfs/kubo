#!/bin/sh

test_description="Test numbers"

. lib/test-lib.sh

test_expect_success "7 is prime" '
	test_number_is_prime 7
'

test_expect_success "9 is not prime" '
	test_must_fail test_number_is_prime 9
'

test_expect_success "7 is not square" '
	test_must_fail test_number_is_square 7
'

test_expect_success "9 is square" '
	test_number_is_square 9
'

test_done
