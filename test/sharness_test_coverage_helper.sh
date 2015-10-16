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

log "Grep the sharness tests for ipfs commands"
CMD_RAW="$TMPDIR/ipfs_cmd_raw.txt"
git grep -n -E '\Wipfs\W' -- sharness/t*-*.sh >"$CMD_RAW" ||
die "Could not grep ipfs in the sharness tests"

grep_out() {
    pattern="$1"
    src="$TMPDIR/ipfs_cmd_${2}.txt"
    dst="$TMPDIR/ipfs_cmd_${3}.txt"
    desc="$4"

    log "Remove $desc"
    egrep -v "$pattern" "$src" >"$dst" || die "Could not remove $desc"
}

grep_out 'test_expect_.*ipfs' raw expect "test_expect_{success,failure} lines"
grep_out '^[^:]+:[^:]+:\s*#' expect comment "comments"
grep_out 'test_description=' comment desc "test_description lines"
grep_out '^[^:]+:[^:]+:\s*e?grep\W[^|]*\Wipfs' desc grep "grep lines"
grep_out '^[^:]+:[^:]+:\s*echo\W[^|]*\Wipfs' grep echo "echo lines"

grep_in() {
    pattern="$1"
    src="$TMPDIR/ipfs_cmd_${2}.txt"
    dst="$TMPDIR/ipfs_cmd_${3}.txt"
    desc="$4"

    log "Keep $desc"
    egrep "$pattern" "$src" >"$dst"
}

grep_in '\Wipfs\W.*/ipfs/' echo slash_in1 "ipfs.*/ipfs/"
grep_in '/ipfs/.*\Wipfs\W' echo slash_in2 "/ipfs/.*ipfs"

grep_out '/ipfs/' echo slash "/ipfs/"

grep_in '\Wipfs\W.*\.ipfs' slash dot_in1 "ipfs.*\.ipfs"
grep_in '\.ipfs.*\Wipfs\W' slash dot_in2 "\.ipfs.*ipfs"

grep_out '\.ipfs' slash dot ".ipfs"

log "Print result"
CMD_RES="$TMPDIR/ipfs_cmd_result.txt"
for f in dot slash_in1 slash_in2 dot_in1 dot_in2
do
    fname="$TMPDIR/ipfs_cmd_${f}.txt"
    cat "$fname" || die "Could not cat '$fname'"
done | sort | uniq >"$CMD_RES" || die "Could not write '$CMD_RES'"

log "Get all the ipfs commands from 'ipfs commands'"
CMD_CMDS="$TMPDIR/commands.txt"
ipfs commands >"$CMD_CMDS" || die "'ipfs commands' failed"

# Remove temp directory...
