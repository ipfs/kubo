#!/usr/bin/env bash

test_description="Test generated bash completions"

. lib/test-lib.sh

test_expect_success "'ipfs commands completion bash' succeeds" '
  ipfs commands completion bash > completions.bash
'

test_expect_success "generated completions defines '_ipfs'" '
  bash -c "source completions.bash && type -t _ipfs"
'

test_done
