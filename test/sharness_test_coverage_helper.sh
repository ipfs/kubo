#!/bin/sh

USAGE="$0 [-h] [-v]"

usage() {
    echo "$USAGE"
    echo "	Print sharness test coverage"
    echo "	Options:"
    echo "		-h|--help: print this usage message and exit"
    echo "		-v|--verbose: print logs of what happens"
    exit 0
}

log() {
    test -z "$VERBOSE" || echo "->" "$@"
}

die() {
    printf >&2 "fatal: %s\n" "$@"
    exit 1
}

# get user options
while [ "$#" -gt "0" ]; do
    # get options
    arg="$1"
    shift

    case "$arg" in
	-h|--help)
	    usage ;;
	-v|--verbose)
	    VERBOSE=1 ;;
	-*)
	    die "unrecognised option: '$arg'\n$USAGE" ;;
	*)
	    die "too many arguments\n$USAGE" ;;
    esac
done

log "Create temporary directory"
DATE=$(date +"%Y-%m-%dT%H:%M:%SZ")
TMPDIR=$(mktemp -d "/tmp/coverage_helper.$DATE.XXXXXX") ||
die "could not 'mktemp -d /tmp/coverage_helper.$DATE.XXXXXX'"

log "Grep the sharness tests"
CMD_RAW="$TMPDIR/ipfs_cmd_raw.txt"
git grep -n -E '\Wipfs\W' -- sharness/t*-*.sh >"$CMD_RAW" ||
die "Could not grep ipfs in the sharness tests"

log "Remove test_expect_{success,failure} lines"
CMD_EXPECT="$TMPDIR/ipfs_cmd_expect.txt"
egrep -v 'test_expect_.*ipfs' "$CMD_RAW" >"$CMD_EXPECT" ||
die "Could not remove test_expect_{success,failure} lines"

log "Remove comments"
CMD_COMMENT="$TMPDIR/ipfs_cmd_comment.txt"
egrep -v '^[^:]+:[^:]+:\s*#' "$CMD_EXPECT" >"$CMD_COMMENT" ||
die "Could not remove comments"

log "Remove test_description lines"
CMD_DESC="$TMPDIR/ipfs_cmd_description.txt"
egrep -v 'test_description=' "$CMD_COMMENT" >"$CMD_DESC" ||
die "Could not remove test_description lines"

log "Remove grep lines"
CMD_GREP="$TMPDIR/ipfs_cmd_grep.txt"
egrep -v '^[^:]+:[^:]+:\s*e?grep\W[^|]*\Wipfs' "$CMD_DESC" >"$CMD_GREP" ||
die "Could not remove grep lines"

log "Remove echo lines"
CMD_ECHO="$TMPDIR/ipfs_cmd_echo.txt"
egrep -v '^[^:]+:[^:]+:\s*echo\W[^|]*\Wipfs' "$CMD_GREP" >"$CMD_ECHO" ||
die "Could not remove echo lines"



log "Keep ipfs.*/ipfs/"
CMD_SLASH_OK="$TMPDIR/ipfs_cmd_slash_ok.txt"
egrep '\Wipfs\W.*/ipfs/' "$CMD_ECHO" >"$CMD_SLASH_OK"

log "Keep ipfs.*\.ipfs and \.ipfs.*ipfs"
CMD_DOT_OK="$TMPDIR/ipfs_cmd_dot_ok.txt"
egrep -e '\Wipfs\W.*\.ipfs' -e '\.ipfs.*\Wipfs\W' "$CMD_ECHO" >"$CMD_DOT_OK"

log "Remove /ipfs/"
CMD_SLASH="$TMPDIR/ipfs_cmd_slash.txt"
egrep -v '/ipfs/' "$CMD_ECHO" >"$CMD_SLASH" ||
die "Could not remove /ipfs/"

log "Remove .ipfs"
CMD_DOT="$TMPDIR/ipfs_cmd_dot.txt"
egrep -v '\.ipfs' "$CMD_SLASH" >"$CMD_DOT" ||
die "Could not remove .ipfs"


log "Print result"
cat "$CMD_DOT" "$CMD_SLASH_OK" "$CMD_DOT_OK" | sort

# Remove temp directory...
