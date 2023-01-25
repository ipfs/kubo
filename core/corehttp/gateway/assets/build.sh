#!/bin/sh

set -euo pipefail

function build() {
  rm -f $1
  sed '/<link rel=\"stylesheet\"/d' ./src/$1 > ./base-html.html
  (echo "<style>" && cat ./src/icons.css ./src/style.css | tr -d "\t\n\r" && echo && echo "</style>") > ./minified-wrapped-style.html
  sed '/<\/title>/ r ./minified-wrapped-style.html' ./base-html.html > ./$1
  rm ./base-html.html && rm ./minified-wrapped-style.html
}

build "directory-index.html"
build "dag-index.html"
