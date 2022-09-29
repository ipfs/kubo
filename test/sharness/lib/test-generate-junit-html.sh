#!/bin/sh

dependencies=(
  "url=https://sourceforge.net/projects/saxon/files/Saxon-HE/11/Java/SaxonHE11-4J.zip;md5=8a4783d307c32c898f8995b8f337fd6b"
  "url=https://raw.githubusercontent.com/pl-strflt/ant/398d29d3fdc88a9f4124a45e87997c2154763d02/src/etc/junit-frames-saxon.xsl;md5=258b2d7a6c4d53dc22f33086d4fa0131"
  "url=https://raw.githubusercontent.com/pl-strflt/ant/398d29d3fdc88a9f4124a45e87997c2154763d02/src/etc/junit-noframes-saxon.xsl;md5=d65690e83721b387bb6653390f08e8d8"
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
