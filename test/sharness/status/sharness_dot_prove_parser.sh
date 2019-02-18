#!/bin/sh

USAGE="$0 [-h] [-v]"

usage() {
	echo "$USAGE"
	echo "	Parse '.prove' files created by running sharness tests with prove"
	echo "	See: https://perldoc.perl.org/prove.html"
	echo "	Options:"
	echo "		-h|--help: print this usage message and exit"
	echo "		-v|--verbose: print logs of what happens"
	exit 0
}

# Example of using `prove` with Sharness tests
# 
# $ cd test/sharness
# $ rm -f .prove
# $ prove --jobs 4 --state=save t[0-9]*.sh
#
# Notes:
#
#   - removing the .prove file is necessary before starting otherwise
#     `prove` might add the new results to the existing .prove file
#   - the `--jobs 4` might speed up things, but it might also create
#     problems if many tests use the same ipfs daemon at the same time
#
# Other example of how to use this script:
# 
# $ cd test/sharness
# $ rm -f .prove
# $ prove --state=save t0250-files-api.sh t0275-cid-security.sh
# $ ./sharness_dot_prove_parser.sh
# t0250 failures
# t0275 passes
#

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

log "Check that the .prove file exists"
PROVE_FILE="sharness/.prove"
test -e "$PROVE_FILE" || die "could not find '$PROVE_FILE'"
test -f "$PROVE_FILE" || die "'$PROVE_FILE' is not a file"

log "Create temporary directory"
DATE=$(date +"%Y-%m-%dT%H:%M:%SZ")
TMP_TMPL="/tmp/dot_prove_parser.$DATE.XXXXXX"
TMPDIR=$(mktemp -d "$TMP_TMPL") ||
	die "could not 'mktemp -d $TMP_TMPL'"

log "Parse .prove file"
TMP_PARSED="$TMPDIR/prove_parsed"
egrep -e '^  t[0-9]+-.*\.sh:' \
      -e '    total_passes: [0-9]+' \
      -e '    total_failures: [0-9]+' "$PROVE_FILE" >"$TMP_PARSED" ||
	die "failed to parse '$PROVE_FILE' into '$TMP_PARSED'"

log "Clean resulting file"
TMP_CLEANED="$TMPDIR/prove_cleaned"
perl -0777 -pe 's/ +(t\d+)-\S+\.sh:\n\s+total_(failures|passes): \d+/$1 $2/gs' "$TMP_PARSED" >"$TMP_CLEANED" ||
	die "failed to clean up '$TMP_PARSED' into '$TMP_CLEANED'"

# Output
cat "$TMP_CLEANED"
