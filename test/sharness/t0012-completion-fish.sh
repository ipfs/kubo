#!/usr/bin/env bash

test_description="Test generated fish completions"

. lib/test-lib.sh

test_expect_success "'ipfs commands completion fish' succeeds" '
  ipfs commands completion fish > completions.fish
'

test_expect_success "generated completions completes 'ipfs version'" '
  fish -c "source completions.fish && complete -C \"ipfs ver\" | grep -q \"version.Show IPFS version information.\" "
'

test_done

