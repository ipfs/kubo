#!/usr/bin/env bash

cat > test-results/sharness.xml <<-EOF
<?xml version="1.1" encoding="UTF-8"?>
<testsuites name="sharness">
  $(find test-results -name '*.xml.part' | sort | xargs cat)
</testsuites>
EOF
