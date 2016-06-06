#!/bin/sh

#
# This script will add or update files in a directly (recursively)
# without copying the data into the datastore.  When run the first
# time it will add all the files.  When run the again it will readd
# any modified or new files.  Invalid blocks due to changed or removed
# files will be cleaned out.
#
# NOTE: Zero length files will always be readded.  Files with the same
# content will also take turns being being readded.
#

# Exit on any error
set -e

LC_ALL=C

if [ "$#" -ne 1 ]; then
    echo "usage: $0 DIR"
    exit 1
fi

# under "$DIR".
#
verify() {
    ipfs filestore ls -q "$DIR"/ | xargs_r ipfs filestore verify --porcelain "$@"
}

#
# First figure out what we already have in the filestore
#
verify -v2 > verify.res 2> verify.err

# Get a list of files that need to be updated
cat verify.res | awk -F'\t' '$2 != "ok" {print $4}' | sort -u > verify.notok

# Get a list of all files in the filestore
cat verify.res | cut -f4 | sort -u > prev-files

#
# Now figure out what we have in the filesystem
#
find "$DIR" -type f | sort -u > cur-files

# Get a list of changed files
comm -12 verify.notok cur-files > changed-files

# Get a list of new files to add
comm -13 prev-files cur-files > new-files

#
# Readd any changed or new files
#
cat changed-files new-files | xargs_r -d '\n' ipfs filestore add

#
# Manually clean the filestore.  Done manually so we only clean he
# files under $DIR
#
# Step 1: remove bad blocks
verify -v6 \
     | tee verify2.res \
     | awk '$2 == "changed" || $2 == "no-file" {print $3}' \
     | xargs_r ipfs filestore rm --direct --force

# Step 2: remove incomplete files, the "-l0" is important as it tells
# us not to try and verify individual blocks just list root nodes
# that are now incomplete.
verify -v2 -l0 \
     | tee verify3.res \
     | awk '$2 == "incomplete" {print $3}' \
     | xargs_r ipfs filestore rm --direct

