#!/usr/bin/env bash

test_description="Test sharness tests are correctly written"

. lib/test-lib.sh

for file in $(find ..  -maxdepth 1 -name 't*.sh' -type f); do
    test_expect_success "test in $file finishes" '
      grep -q "^test_done\b" "$file"
    '

    test_expect_success "test in $file has a description" '
              grep -q "^test_description=" "$file"
            '

    # We have some tests that manually kill.
    case "$(basename "$file")" in
        t0060-daemon.sh|t0023-shutdown.sh) continue ;;
    esac

    test_expect_success "test in $file has matching ipfs start/stop" '
      awk "/^ *[^#]*test_launch_ipfs_daemon/ { if (count != 0) { exit(1) }; count++ } /^ *[^#]*test_kill_ipfs_daemon/ { if (count != 1) { exit(1) }; count-- } END { exit(count) }" "$file"
    '
done

test_done
