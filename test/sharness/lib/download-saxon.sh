#!/bin/bash

dependencies=(
  "url=https://raw.githubusercontent.com/pl-strflt/Saxon-HE/3e039cdbccf4efb9643736f34c839a3bae3402ae/11/Java/SaxonHE11-4J.zip;md5=8a4783d307c32c898f8995b8f337fd6b"
  "url=https://raw.githubusercontent.com/pl-strflt/ant/c781f7d79b92cc55530245d9554682a47f46851e/src/etc/junit-frames-saxon.xsl;md5=6eb013566903a91e4959413f6ff144d0"
  "url=https://raw.githubusercontent.com/pl-strflt/ant/c781f7d79b92cc55530245d9554682a47f46851e/src/etc/junit-noframes-saxon.xsl;md5=8d54882d5f9d32a7743ec675cc2e30ac"
)

dependenciesdir="lib/dependencies"
mkdir -p "$dependenciesdir"

get_md5() {
  md5sum "$1" | cut -d ' ' -f 1
}

for dependency in "${dependencies[@]}"; do
  url="$(echo "$dependency" | cut -d ';' -f 1 | cut -d '=' -f 2)"
  md5="$(echo "$dependency" | cut -d ';' -f 2 | cut -d '=' -f 2)"
  filename="$(basename "$url")"
  if test -f "$dependenciesdir/$filename" && test "$(get_md5 "$dependenciesdir/$filename")" = "$md5"; then
    echo "Using cached $filename"
  else
    echo "Downloading $filename"
    curl -L --max-redirs 5 --retry 5 --no-progress-meter --output "$dependenciesdir/$filename" "$url"
    actual_md5="$(get_md5 "$dependenciesdir/$filename")"
    if test "$actual_md5" != "$md5"; then
      echo "Downloaded $filename has wrong md5sum ('$actual_md5' != '$md5')"
      exit 1
    fi
    dirname=${filename%.*}
    extension=${filename#$dirname.}
    if test "$extension" = "zip"; then
      echo "Removing old $dependenciesdir/$dirname"
      rm -rf "$dependenciesdir/$dirname"
      echo "Unzipping $dependenciesdir/$filename"
      unzip "$dependenciesdir/$filename" -d "$dependenciesdir/$dirname"
    fi
  fi
done
