#!/usr/bin/env bash

test_description="Test sharness test indent"

. lib/test-lib.sh

for file in $(find .. -name 't*.sh' -type f); do
  if [ "$(basename "$file")" = "t0290-cid.sh" ]; then
      continue
  fi
  test_expect_success "indent in $file is not using tabs" '
    test_must_fail grep -P "^ *\t" $file
  '
done

test_done
