#!/bin/bash

case "$1" in
  "frames")
    java -jar lib/dependencies/SaxonHE11-4J/saxon-he-11.4.jar \
      -s:test-results/sharness.xml \
      -xsl:lib/dependencies/junit-frames-saxon.xsl \
      output.dir=$(pwd)/test-results/sharness-html
    ;;
  "no-frames")
    java -jar lib/dependencies/SaxonHE11-4J/saxon-he-11.4.jar \
      -s:test-results/sharness.xml \
      -xsl:lib/dependencies/junit-noframes-saxon.xsl \
      -o:test-results/sharness.html
    ;;
  *)
    echo "Usage: $0 [frames|no-frames]"
    exit 1
    ;;
esac
