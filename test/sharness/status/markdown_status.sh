#!/bin/sh

USAGE="$0 [-h] [-v]"

usage() {
	echo "$USAGE"
	echo "	Use result files from implementation_status.sh to generate "
	echo "	a status file in the Markdown format that show which ipfs "
	echo "  commands are working, not working, or not tested according "
	echo "  to sharness tests."
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

log "Create temporary directory"
DATE=$(date +"%Y-%m-%dT%H:%M:%SZ")
TMP_TMPL="/tmp/markdown_status.$DATE.XXXXXX"
TMPDIR=$(mktemp -d "$TMP_TMPL") ||
	die "could not 'mktemp -d $TMP_TMPL'"

TXT_STATUS='results/status.txt'

test -f "$TXT_STATUS" ||
	die "could not find '$TXT_STATUS'"

log "Reduce number of ipfs lines"
TMP_MD_STATUS_1="$TMPDIR/markdown_impl_status_1"
cp "$TXT_STATUS" "$TMP_MD_STATUS_1" ||
	die "could not cp '$TXT_STATUS' into '$TMP_MD_STATUS_1'"
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
