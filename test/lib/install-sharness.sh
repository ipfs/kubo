#!/bin/sh
# install sharness.sh
#
# Copyright (c) 2014 Juan Batiz-Benet
# MIT Licensed; see the LICENSE file in this repository.
#

# settings
version=50229a79ba22b2f13ccd82451d86570fecbd194c
urlprefix=https://raw.githubusercontent.com/mlafeldt/sharness/$version
installpath=lib/sharness

# files to download
sfile=sharness.sh
shash=eeaf96630fc25ec58fb678b64ef9772d5eb92f64

afile=aggregate-results.sh
ahash=948d6bc03222c5c00a1ed048068508d5ea1cce59

die() {
  echo >&2 $@
  exit 1
}

verified_download() {
  file=$1
  hash1=$2
  url=$urlprefix/$file

  # download it
  wget -q $url -O $file.test

  # verify it's the right file
  hash2=`cat $file.test | shasum | cut -c1-40`
  if test "$hash1" != "$hash2"; then
    echo "$file verification failed:"
    echo "  $hash1 != $hash2"
    return -1
  fi
  return 0
}

mkdir -p $installpath || die "Could not create 'sharness' directory"
cd $installpath || die "Could not cd into 'sharness' directory"

verified_download "$sfile" "$shash"; sok=$?
verified_download "$afile" "$ahash"; aok=$?
if test "$sok" != 0 || test "$aok" != 0; then
  rm $afile.test
  rm $sfile.test
  exit -1
fi

# ok, move things into place
mv $sfile.test $sfile
mv $afile.test $afile
chmod +x $sfile
chmod +x $afile
exit 0
