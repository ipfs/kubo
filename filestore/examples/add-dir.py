#!/usr/bin/python3

#
# This script will add or update files in a directly (recursively)
# without copying the data into the datastore.  Unlike
# add-dir-simply.py it will use it's own file to keep track of what
# files are added to avoid the problem with duplicate files being
# re-added.
#
# This script will not clean out invalid entries from the filestore,
# for that you should use "filestore clean full" from time to time.
#

import sys
import os.path
import subprocess as sp

#
# Maximum length of command line, this may need to be lowerd on
# windows.
#

MAX_CMD_LEN = 120 * 1024

#
# Parse command line arguments
#

def print_err(*args, **kwargs):
    print(*args, file=sys.stderr, **kwargs)

if len(sys.argv) != 3:
    print_err("Usage: ", sys.argv[0], "DIR CACHE")
    sys.exit(1)

dir = sys.argv[1]
if not os.path.isabs(dir):
    print_err("directory name must be absolute:", dir)
    sys.exit(1)

cache = sys.argv[2]
if not os.path.isabs(cache):
    print_err("cache file name must be absolute:", dir)
    sys.exit(1)

#
# Global variables
#

before = [] # list of (hash mtime path) -- from data file
to_readd = set()
already_have = set()
toadd = {}

#
# Read in cache (if it exists) and determine hashes that need to be
# readded
#

if os.path.exists(cache):

    try:
        f = open(cache)
    except OSError as e:
        print_err("count not open cache file: ", e)
        sys.exit(1)

    for line in f:
        hash,mtime,path = line.rstrip('\n').split(' ', 2)
        try:
            new_mtime = "%.6f" % os.path.getmtime(path)
        except OSError as e:
            print_err("skipping", path)
            continue
        before.append((hash,mtime,path),)
        if mtime != new_mtime:
            to_readd.add(hash)
    
    del f

    os.rename(cache, cache+".old")

#
# Open new one for writing
#

try:
    f = open(cache, 'w')
except OSError as e:
    print_err("count write to cache file: ", e)
    os.rename(cache+".old", cache)
    sys.exit(1)

#
# Figure out what files don't need to be readded.  This is done by
# hash, not by filename so that if two files have the same content and
# one of them changes the original content will still be available.
#

for hash,mtime,path in before:
    if hash not in to_readd:
        already_have.add(path)
        print(hash,mtime,path, file=f)

# To cut back on memory usage
del before
del to_readd

#
# Figure out what files need to be re-added
#

for root, dirs, files in os.walk(dir):
    for file in files:
        try:
            path = os.path.join(root,file)
            if path not in already_have:
                mtime = "%.6f" % os.path.getmtime(path)
                #print("will add", path)
                toadd[path] = mtime
        except OSError as e:
            print_err("SKIPPING", path, ":", e)

#
# Finally, do the add.  Write results to the cache file as they are
# added.
#

print("adding files...")

errors = False

while toadd:

    cmd = ['ipfs', 'filestore', 'add']
    paths = []
    cmd_len = len(' '.join(cmd)) + 1
    for key in toadd.keys():
        cmd_len += len(key) + 1 + 8
        if cmd_len > MAX_CMD_LEN: break
        paths.append(key)

    pipe = sp.Popen(cmd+paths, stdout=sp.PIPE, bufsize=-1, universal_newlines=True)

    for line in pipe.stdout:
        try:
            _, hash, path = line.rstrip('\n').split(None, 2)
            mtime = toadd[path]
            del toadd[path]
            print(hash,mtime,path, file=f)
        except Exception as e:
            errors = True
            print_err("WARNING: problem when adding: ", path, ":", e)
            # don't abort, non-fatal error

    pipe.stdout.close()
    pipe.wait()
            
    if pipe.returncode != 0:
        errors = True
        print_err("ERROR: \"ipfs filestore add\" return non-zero exit code.")
        break

    for path in paths:
        if path in toadd:
            errors = True
            print_err("WARNING: ", path, "not added.")
            del toadd[path]

    print("added", len(paths), "files, ", len(toadd), "more to go.")

#
# Cleanup
#

f.close()

if errors:
    os.exit(1)
