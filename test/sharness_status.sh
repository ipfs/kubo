#!/bin/sh

USAGE="$0 [-h] [-v]"

usage() {
	echo "$USAGE"
	echo "	Generate status files that show implementation status"
	echo "	or test coverage related to ipfs commands and sharness"
	echo "  tests and sharness test results."
	echo "	Options:"
	echo "		-h|--help: print this usage message and exit"
	echo "		-v|--verbose: print logs of what happens"
	echo "		--coverage: generate test coverage"
	echo "		--no_markdown: don't generate markdown output"
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
		--coverage)
			COVERAGE=1 ;;
		--no_markdown)
			NO_MARKDOWN=1 ;;
		-*)
			die "unrecognised option: '$arg'\n$USAGE" ;;
		*)
			die "too many arguments\n$USAGE" ;;
	esac
done

