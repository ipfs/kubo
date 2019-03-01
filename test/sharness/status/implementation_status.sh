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
		--no_markdown)
			NO_MARKDOWN=1 ;;
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

test -z "$NO_MARKDOWN" || {
	cat "$TMP_STATUS"
	exit
}

log "Reduce number of ipfs lines"
TMP_MD_STATUS_1="$TMPDIR/markdown_impl_status_1"
cp "$TMP_STATUS" "$TMP_MD_STATUS_1" ||
	die "could not cp '$TMP_STATUS' into '$TMP_MD_STATUS_1'"
perl -pi -e 'BEGIN{undef $/;} s/ipfs (\w+) ([\w-]+ )*--([\w-]+)\nipfs \w+ ([\w-]+ )*-(\w)/ipfs $1 $2-$5|--$3/smg' "$TMP_MD_STATUS_1" ||
	die "could not reduce number of ipfs lines in '$TMP_MD_STATUS_1'"
TMP_MD_STATUS_2="$TMPDIR/markdown_impl_status_2"
cp "$TMP_MD_STATUS_1" "$TMP_MD_STATUS_2" ||
	die "could not cp '$TMP_MD_STATUS_1' into '$TMP_MD_STATUS_2'"
perl -pi -e 'BEGIN{undef $/;} s/ipfs (\w+) ([\w-]+ )*-(\w)\nipfs \w+ ([\w-]+ )*-([\w-]+)/ipfs $1 $2-$3|--$5/smg' "$TMP_MD_STATUS_2" ||
	die "could not reduce number of ipfs lines in '$TMP_MD_STATUS_2'"
TMP_MD_STATUS_3="$TMPDIR/markdown_impl_status_3"
cp "$TMP_MD_STATUS_2" "$TMP_MD_STATUS_3" ||
	die "could not cp '$TMP_MD_STATUS_2' into '$TMP_MD_STATUS_3'"
perl -pi -e 'BEGIN{undef $/;} s/ipfs (\w+) ([\w-]+ )*--([\w-]+)\nipfs \w+ ([\w-]+ )*--([\w-]+)/ipfs $1 $2-$5|--$3/smg' "$TMP_MD_STATUS_3" ||
	die "could not reduce number of ipfs lines in '$TMP_MD_STATUS_3'"

#cat "$TMP_MD_STATUS_3"
#exit

log "Reduce number of test lines"
TMP_MD_STATUS_4="$TMPDIR/markdown_impl_status_4"
perl -ne '

BEGIN {
	sub print_tests {
		if (%t) {
			foreach my $k (sort keys %t) {
				$sum = 0;
				map { $sum += $_; } @{$n{$k}};
				print "$k: " . $sum . " ";
			}
			print "\n";
			%t = ();
			%n = ();
		} else {
			print "???\n";
		}
	}
}

if (m/^\s+((\d+) ((GOOD|BAD):)?(t\d+))$/) {
	push(@{$n{$4 ? $4 : "XXX"}}, $2);
	push(@{$t{$4 ? $4 : "XXX"}}, $5);
	$started = 1;
} elsif (m/^\s*$/ and $started) {
	print_tests();
	print;
} else {
	print unless (m/^\s*$/);
}

END {
	print_tests();
}

' "$TMP_MD_STATUS_3"  > "$TMP_MD_STATUS_4"

log "Generate markdown impl status"
TMP_MD_STATUS_5="$TMPDIR/markdown_impl_status_5"
cat <<EOF >"$TMP_MD_STATUS_5"
# IPFS Implementation Status

> Legend: :green_apple: Done &nbsp; :lemon: In Progress &nbsp; :tomato: Missing &nbsp; :chestnut: Not planned

| Command                                      | Go Impl                                      |
| -------------------------------------------- | :------------------------------------------: |
EOF

while read -r line
do
	if expr "$line" : "^ipfs " >/dev/null
	then
		#echo "ipfs line: $line"
		printf "| %*s |" 44 "$line"
	elif test -n "$line"
	then
		#echo "other line: $line"
		printf " %*s |" 44 "$line"
	else
		#echo "empty line: $line"
		printf "\n"
	fi
done <"$TMP_MD_STATUS_4" >>"$TMP_MD_STATUS_5"

printf "\n" >>"$TMP_MD_STATUS_5"

# Output
cat "$TMP_MD_STATUS_5"
