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
CMDRAW="$TMPDIR/ipfs_cmd_raw.txt"
git grep -E '\Wipfs\W' -- sharness/t*-*.sh >"$CMDRAW" ||
die "Could not grep ipfs in the sharness tests"

log "Remove test_expect_{success,failure} lines"
CMDPROC1="$TMPDIR/ipfs_cmd_proc1.txt"
egrep -v 'test_expect_.*ipfs' "$CMDRAW" >"$CMDPROC1" ||
die "Could not remove test_expect_{success,failure} lines"

log "Remove comments"
CMDPROC2="$TMPDIR/ipfs_cmd_proc2.txt"
egrep -v '^\s*#' "$CMDPROC1" >"$CMDPROC2" ||
die "Could not remove comments"


log "Print result"
cat "$CMDPROC2"

# Remove temp directory...
