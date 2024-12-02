#!/bin/sh
# install sharness.sh
#
# Copyright (c) 2014, 2022 Juan Batiz-Benet, Piotr Galar
# MIT Licensed; see the LICENSE file in this repository.
#

gitrepo=ipfs/sharness
githash=803df39d3cba16bb7d493dd6cd8bc5e29826da61

if test ! -n "$clonedir" ; then
  clonedir=lib
fi
sharnessdir=sharness
gitdir="$clonedir/$sharnessdir/.git"

die() {
  echo >&2 "$@"
  exit 1
}

if test -d "$clonedir/$sharnessdir"; then
  giturl="git@github.com:${gitrepo}.git"
  echo "Checking if $giturl is already cloned (and if its origin is correct)"
  if ! test -d "$gitdir" || test "$(git --git-dir "$gitdir" remote get-url origin)" != "$giturl"; then
    echo "Removing $clonedir/$sharnessdir"
    rm -rf "$clonedir/$sharnessdir" || die "Could not remove $clonedir/$sharnessdir"
  fi
fi

if ! test -d "$clonedir/$sharnessdir"; then
  giturl="https://github.com/${gitrepo}.git"
  echo "Cloning $giturl into $clonedir/$sharnessdir"
  git clone "$giturl" "$clonedir/$sharnessdir" || die "Could not clone $giturl into $clonedir/$sharnessdir"
fi


echo "Changing directory to $clonedir/$sharnessdir"
cd "$clonedir/$sharnessdir" || die "Could not cd into '$clonedir/$sharnessdir' directory"

echo "Checking if $githash is already fetched"
if ! git show "$githash" >/dev/null 2>&1; then
  echo "Fetching $githash"
  git fetch origin "$githash" || die "Could not fetch $githash"
fi

echo "Resetting to $githash"
git reset --hard "$githash" || die "Could not reset to $githash"

exit 0
