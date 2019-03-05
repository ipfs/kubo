#!/bin/sh

USAGE="$0 [-h] [-v] <files>"

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

# get user options
while [ "$#" -gt "0" ]; do
	# get options
	arg="$1"

	case "$arg" in
		-h|--help)
			usage ;;
		-v|--verbose)
			VERBOSE=1
			shift
			;;
		-*)
			die "unrecognised option: '$arg'\n$USAGE" ;;
		*)
			break ;;
	esac
done

for TXT_STATUS in "$@"
do
	test -f "$TXT_STATUS" ||
		die "could not find file '$TXT_STATUS'"
done

log "Create temporary directory"
DATE=$(date +"%Y-%m-%dT%H:%M:%SZ")
TMP_TMPL="/tmp/markdown_status.$DATE.XXXXXX"
TMPDIR=$(mktemp -d "$TMP_TMPL") ||
	die "could not 'mktemp -d $TMP_TMPL'"

# TODO: improve by parsing argument or something
get_legend() {
	echo "$(basename $1)"
}

log "Generate markdown impl status"
MD_STATUS="$TMPDIR/markdown_impl_status"

# Header
cat <<EOF >"$MD_STATUS"
# IPFS Implementation Status

> Legend: :green_apple: Done &nbsp; :lemon: In Progress &nbsp; :tomato: Missing &nbsp; :chestnut: Not planned

EOF

# 1st line of the table
printf "| Command                                      |" >>"$MD_STATUS"
for TXT_STATUS in "$@"
do
	printf " %*s |" 44 "$(get_legend $TXT_STATUS)" >>"$MD_STATUS"
done
echo >>"$MD_STATUS"

# 2nd line of the table
printf "| -------------------------------------------- |" >>"$MD_STATUS"
for TXT_STATUS in "$@"
do
	printf " :------------------------------------------: |" >>"$MD_STATUS"
done
echo >>"$MD_STATUS"

# Rest of the table
perl -e '

use strict;
use warnings;

my %hm;

# Reading files
for my $i (0..$#ARGV) {
	open(my $fh, "<", $ARGV[$i]) or die "Could not open \"$ARGV[$i]\": $!";

	my $key;
	while (my $row = <$fh>) {
	      chomp $row;
	      if ($row =~ m/^ipfs/) {
	            $key = $row;
	      } elsif ($row ne "") {
	            $hm{$key}[$i] = $row;
	      }
	}
}

# Printing
for my $k (sort keys %hm) {
	printf("| %*s |", 44, $k);
	for my $i (0..$#ARGV) {
		printf (" %*s |", 44, $hm{$k}[$i]);
	}
	print "\n";
}


' "$@" >>"$MD_STATUS"

# Output
cat "$MD_STATUS"
