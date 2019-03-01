#!/bin/sh

USAGE="$0 [-h] [-v]"

usage() {
	echo "$USAGE"
	echo "	Use result files from dot_prove_parser.sh and "
	echo "	coverage_helper.sh to generate a status file "
	echo "	that show which ipfs commands are working, not "
	echo "  working, or not tested according to sharness tests."
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
TMP_TMPL="/tmp/implementation_status.$DATE.XXXXXX"
TMPDIR=$(mktemp -d "$TMP_TMPL") ||
	die "could not 'mktemp -d $TMP_TMPL'"

PARSED_DOT_PROVE='results/dot_prove.txt'

test -f "$PARSED_DOT_PROVE" ||
	die "could not find '$PARSED_DOT_PROVE'"

COVERAGE_RESULTS='results/coverage.txt'

test -f "$COVERAGE_RESULTS" ||
	die "could not find '$COVERAGE_RESULTS'"

log "Generate implementation status"
TMP_STATUS="$TMPDIR/impl_status"
cp "$COVERAGE_RESULTS" "$TMP_STATUS" ||
	die "could not cp '$COVERAGE_RESULTS' into '$TMP_STATUS'"
while read -r tst status
do
	out="GOOD:$tst"
	test "$status" = "failures" && out="BAD:$tst"
	perl -pi -e "s/$tst/$out/" "$TMP_STATUS" ||
		die "could not substitute '$tst' in '$TMP_STATUS'"
done < "$PARSED_DOT_PROVE"

cat "$TMP_STATUS"
