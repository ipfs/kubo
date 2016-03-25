#!/bin/sh

bin=ipfs

old_IFS="$IFS"
IFS=':'

echo "Welcome to the $bin installer."
echo "With your permission it will try to install the $bin binary into one of"
echo "the folders found in your \$PATH variable."
echo

if [ ! -f "$bin" ]; then
  echo "The '$bin' binary was not found in the current directory."
  echo "There is nothing to install."
  exit 1
fi

# Make sure that /usr/local/bin and /usr/bin are the first paths we try.
# Note that these paths might be already found in $PATH.
paths="/usr/local/bin:/usr/bin:$PATH"
bestpath=''

echo "We will now try to determine the best directory to install $bin into:"
echo
for binpath in $paths;
do
  if [ ! -d "$binpath" ]; then
    echo "$binpath does not seem to be a directory. Skipping..."
    continue
  fi
  if [ ! -w "$binpath" ]; then
    echo "$binpath does not seem to be writable. Skipping..."
    continue
  fi
  echo -n "$binpath looks like it might work"
  if [ -z "$bestpath" ]; then
    bestpath="$binpath"
    echo ". Let's take it!"
  else
    echo " but we already found a better path. Skipping..."
  fi
done

# If the user is not root do let them know that they might want to run
# this script as a privileged user.
if [ "$(id -u)" != "0" ]; then
  echo
  echo "(Note that your might want to run this script as a privileged user,"
  echo " many of the directories above might become writable.)"
fi

if [ -z "$bestpath" ]; then
  echo "No useful path was found."
  exit 1
fi

binpath="$bestpath"

echo
echo "$bin will be installed to '$binpath'"
while true; do
    read -p "Is that okay? [Y/n] " yn

    # Default to yes on empty response
    if [ -z "$yn" ]; then
      yn="Y"
    fi

    case $yn in
        [Yy]* ) break;;
        [Nn]* ) exit 0;;
        * ) echo "Please answer yes or no.";;
    esac
done

if mv "$bin" "$binpath"; then
  echo "Installed $bin into $binpath/$bin"
  exit 0
else
  echo "We were unable to install $bin into $binpath"
  echo "Please make sure that you can write into $binpath"
  echo "(possibly by running this script as a privileged user)"
  exit 1
fi

IFS="$old_IFS"
