#!/bin/sh

USAGE="$0 [-h] [-v]"

usage() {
	echo "$USAGE"
	echo "	Use sharness_dot_prove_parser.sh and sharness_test_coverage_helper.sh"
	echo "	to generate a status file that show which ipfs commands are working, not"
	echo "	working, or not tested according to sharness tests."
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

export VERBOSE

log "Parse .prove file"
DOT_PROVE_PARSER='sharness_dot_prove_parser.sh'
test -f "$DOT_PROVE_PARSER" || die "could not find '$DOT_PROVE_PARSER'"
TMP_CLEANED="$TMPDIR/cleaned_dot_prove"
./"$DOT_PROVE_PARSER" >"$TMP_CLEANED" || exit


log "Get command coverage report"
COVERAGE_HELPER='sharness_test_coverage_helper.sh'
test -f "$COVERAGE_HELPER" || die "could not find '$COVERAGE_HELPER'"
TMP_COVERAGE="$TMPDIR/test_coverage"
./"$COVERAGE_HELPER" >"$TMP_COVERAGE" || exit


log "Generate implementation status"
TMP_STATUS="$TMPDIR/impl_status"
cp "$TMP_COVERAGE" "$TMP_STATUS" ||
	die "could not cp '$TMP_COVERAGE' into '$TMP_STATUS'"
while read -r tst status
do
	out="GOOD($tst)"
	test "$status" = "failures" && out="BAD($tst)"
	perl -pi -e "s/$tst/$out/" "$TMP_STATUS" ||
		die "could not substitute '$tst' in '$TMP_STATUS'"
done < "$TMP_CLEANED"

# Output
cat "$TMP_STATUS"
