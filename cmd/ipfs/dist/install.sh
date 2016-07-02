#!/bin/sh

# Looks for the best $PATH location
# Moves $bin to a valid location in $PATH

bin="ipfs" #bin file

if ! [ -f $bin ]; then
  echo "$bin is already installed or missing"
  exit 1
fi

if [ "$USER" != "root" ] || [ "$(whoami)" != 'root' ]; then
  echo "You need root privileges to install $bin"
  exit 2
fi

move() {
  mv "$bin" "$binpath/$bin"
  e=$?
  if [ $e -ne 0 ]; then
    echo "failed to install $binpath/$bin with error code $e"
    exit $e
  else
    chmod +x $binpath/$bin
    echo "installed $binpath/$bin"
    exit 0
  fi
}
e="'" #escape

for p in $e${PATH//":"/"' '"}$e; do
  case $p in
    $e[a-z/]*$e)
      if [ -d ${p//"'"/""} ]; then
        valid="$valid $p"
      else
        echo "$p is not a directory, shouldn´t be in \$PATH"
      fi
      ;;
    *)
      echo "ignoring invalid path $p"
      ;;
  esac
done

findbest() {
  for v in $valid; do
    case $v in
      $e/usr/bin$e) # /usr/bin
        l=9
        ;;
      $e/usr/[a-z]*bin$e) # /usr/sbin
        l=8
        ;;
      $e/usr/[a-z]*/bin$e) # /usr/local/bin
        l=7
        ;;
      $e/usr/[a-z]*/*bin$e) # /usr/local/sbin
        l=6
        ;;
      $e/bin$e) # /bin
        l=3
        ;;
      $e/[a-z]*bin$e) # /sbin
        l=2
        ;;
      $e/*$e) #Everything else
        l=1
        ;;
    esac
    echo $l $v
  done
}

best=`findbest | sort -n | tail -n 1`
best=${best//" "/"
"}
best=`echo "$best" | tail -n 1`

if [ -z "$best" ]; then
  echo "No valid location in \$PATH found"
  if [ -z "$PATH" ]; then
    echo "$PATH is not set"
    exit 5
  else
    echo "Using /bin as location..."
    best="/bin"
  fi
fi

binpath=${best//"'"/""} #unescape
move
