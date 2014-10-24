# Test framework for go-ipfs
#
# Copyright (c) 2014 Christian Couder
#
# We are using sharness (https://github.com/mlafeldt/sharness)
# which was extracted from the Git test framework.

# You need either sharness to be installed system-wide
# or to set SHARNESS_DIRECTORY properly

if test -z "$SHARNESS_DIRECTORY"
then
	SHARNESS_DIRECTORY=/usr/local/share/sharness
fi

SHARNESS_LIB="$SHARNESS_DIRECTORY/sharness.sh"

test -f "$SHARNESS_LIB" || {
	echo >&2 "Cannot find sharness.sh in: $SHARNESS_DIRECTORY"
	echo >&2 "Please install Sharness system-wide or set the"
	echo >&2 "SHARNESS_DIRECTORY environment variable."
	echo >&2 "See: https://github.com/mlafeldt/sharness"
	exit 1
}

. "$SHARNESS_LIB" || {
	echo >&2 "Cannot source: $SHARNESS_LIB"
	echo >&2 "Please check Sharness installation."
	exit 1
}

# Please put go-ipfs specific shell functions below

