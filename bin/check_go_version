#!/bin/sh
#
# Check that the go version is at least equal to a minimum version
# number.
#
# Call it for example like this:
#
#   $ check_go_version "1.5.2"
#

USAGE="$0 GO_MIN_VERSION"

die() {
    printf >&2 "fatal: %s\n" "$@"
    exit 1
}

# Get arguments

test "$#" -eq "1" || die "This program must be passed exactly 1 arguments" "Usage: $USAGE"

GO_MIN_VERSION="$1"

UPGRADE_MSG="Please take a look at https://golang.org/doc/install to install or upgrade go."

die_upgrade() {
    printf >&2 "fatal: %s\n" "$@"
    printf >&2 "=> %s\n" "$UPGRADE_MSG"
    exit 1
}

major_number() {
    vers="$1"

    # Hack around 'expr' exiting with code 1 when it outputs 0
    case "$vers" in
        0) echo "0" ;;
        0.*) echo "0" ;;
        *) expr "$vers" : "\([^.]*\).*" || return 1
    esac
}

check_at_least_version() {
    MIN_VERS="$1"
    CUR_VERS="$2"
    PROG_NAME="$3"

    # Get major, minor and fix numbers for each version
    MIN_MAJ=$(major_number "$MIN_VERS") || die "No major version number in '$MIN_VERS' for '$PROG_NAME'"
    CUR_MAJ=$(major_number "$CUR_VERS") || die "No major version number in '$CUR_VERS' for '$PROG_NAME'"

    if MIN_MIN=$(expr "$MIN_VERS" : "[^.]*\.\([^.]*\).*"); then
        MIN_FIX=$(expr "$MIN_VERS" : "[^.]*\.[^.]*\.\([^.]*\).*") || MIN_FIX="0"
    else
        MIN_MIN="0"
        MIN_FIX="0"
    fi
    if CUR_MIN=$(expr "$CUR_VERS" : "[^.]*\.\([^.]*\).*"); then
        CUR_FIX=$(expr "$CUR_VERS" : "[^.]*\.[^.]*\.\([^.]*\).*") || CUR_FIX="0"
    else
        CUR_MIN="0"
        CUR_FIX="0"
    fi

    # Compare versions
    VERS_LEAST="$PROG_NAME version '$CUR_VERS' should be at least '$MIN_VERS'"
    test "$CUR_MAJ" -gt $(expr "$MIN_MAJ" - 1) || die_upgrade "$VERS_LEAST"
    test "$CUR_MAJ" -gt "$MIN_MAJ" || {
        test "$CUR_MIN" -gt $(expr "$MIN_MIN" - 1) || die_upgrade "$VERS_LEAST"
        test "$CUR_MIN" -gt "$MIN_MIN" || {
            test "$CUR_FIX" -ge "$MIN_FIX" || die_upgrade "$VERS_LEAST"
        }
    }
}

# Check that the go binary exist and is in the path

type go >/dev/null 2>&1 || die_upgrade "go is not installed or not in the PATH!"

# Check the go binary version

VERS_STR=$(go version 2>&1) || die "'go version' failed with output: $VERS_STR"

GO_CUR_VERSION=$(expr "$VERS_STR" : ".*go version go\([^ ]*\) .*") || die "Invalid 'go version' output: $VERS_STR"

check_at_least_version "$GO_MIN_VERSION" "$GO_CUR_VERSION" "go"
