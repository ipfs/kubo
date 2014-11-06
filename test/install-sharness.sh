#!/bin/sh
# install sharness.sh
#
# Copyright (c) 2014 Juan Batiz-Benet
# MIT Licensed; see the LICENSE file in this repository.
#

# settings
version=50229a79ba22b2f13ccd82451d86570fecbd194c
hash1=eeaf96630fc25ec58fb678b64ef9772d5eb92f64
url=https://raw.githubusercontent.com/mlafeldt/sharness/$version/sharness.sh
file=sharness.sh

# download it
wget -q $url -O $file.test

# verify it's the right file
hash2=`cat $file.test | shasum | cut -c1-40`
if test "$hash1" != "$hash2"; then
  echo "$file verification failed"
  echo "$hash1 != $hash2"
  rm $file.test
  exit -1
fi

# ok, move it into place
mv $file.test $file
exit 0
