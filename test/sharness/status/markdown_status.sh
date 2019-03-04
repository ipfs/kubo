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

log "Generate markdown impl status"
MD_STATUS="$TMPDIR/markdown_impl_status"
cat <<EOF >"$MD_STATUS"
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
done <"$TXT_STATUS" >>"$MD_STATUS"

printf "\n" >>"$MD_STATUS"

# Output
cat "$MD_STATUS"
