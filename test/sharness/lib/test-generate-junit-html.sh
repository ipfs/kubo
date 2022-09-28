#!/bin/sh

dependencies=(
  "url=https://repo1.maven.org/maven2/net/sf/saxon/Saxon-HE/11.4/Saxon-HE-11.4.jar;md5=9f35652962ffe9b687fb7e3726600f03"
  "url=https://raw.githubusercontent.com/pl-strflt/ant/2b021a50901ada753a3b62d1f90e5785b1f2beb1/src/etc/junit-frames-saxon.xsl;md5=f4c6f8912a45a9cf272fa1909720c316"
  "url=https://raw.githubusercontent.com/pl-strflt/ant/2b021a50901ada753a3b62d1f90e5785b1f2beb1/src/etc/junit-noframes-saxon.xsl;md5=dd3d96f8a8cce4c386bfa017e948ad57"
)

dependenciesdir="lib/dependencies"
mkdir -p "$dependenciesdir"

for dependency in "${dependencies[@]}"; do
  url="$(echo "$dependency" | cut -d ';' -f 1 | cut -d '=' -f 2)"
  md5="$(echo "$dependency" | cut -d ';' -f 2 | cut -d '=' -f 2)"
  filename="$(basename "$url")"
  if test -f "$dependenciesdir/$filename" && test "$(md5sum "$dependenciesdir/$filename" | cut -d ' ' -f 1)" = "$md5"; then
    echo "Using cached $filename"
  else
    echo "Downloading $filename"
    curl --retry 5 --no-progress-meter --output "$dependenciesdir/$filename" "$url"
    if test "$(md5sum "$dependenciesdir/$filename" | cut -d ' ' -f 1)" != "$md5"; then
      echo "$(md5sum "$dependenciesdir/$filename" | cut -d ' ' -f 1)"
      echo "$md5"
      echo "Downloaded $filename has wrong md5sum"
      exit 1
    fi
  fi
done

case "$1" in
  "frames")
    java -jar lib/saxon/saxon-he-11.4.jar \
      -s:test-results/sharness.xml \
      -xsl:lib/ant/junit-frames-saxon.xsl \
      output.dir=test-results/sharness-html
    ;;
  "no-frames")
    java -jar lib/saxon/saxon-he-11.4.jar \
      -s:test-results/sharness.xml \
      -xsl:lib/ant/junit-noframes-saxon.xsl \
      -o:test-results/sharness.html
    ;;
  *)
    echo "Usage: $0 [frames|no-frames]"
    exit 1
    ;;
esac
