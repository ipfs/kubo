# Generic test functions for go-ipfs

ansi_strip() {
    sed 's/\x1b\[[0-9;]*m//g'
}

# Quote arguments for sh eval
shellquote() {
	_space=''
	for _arg
	do
		# On macOS, sed adds a newline character.
		# With a printf wrapper the extra newline is removed.
		printf "$_space'%s'" "$(printf "%s" "$_arg" | sed -e "s/'/'\\\\''/g;")"
		_space=' '
	done
	printf '\n'
}

# Echo the args, run the cmd, and then also fail,
# making sure a test case fails.
test_fsh() {
    echo "> $@"
    eval $(shellquote "$@")
    echo ""
    false
}

# Same as sharness' test_cmp but using test_fsh (to see the output).
# We have to do it twice, so the first diff output doesn't show unless it's
# broken.
test_cmp() {
	diff -q "$@" >/dev/null || test_fsh diff -u "$@"
}

# Same as test_cmp above, but we sort files before comparing them.
test_sort_cmp() {
	sort "$1" >"$1_sorted" &&
	sort "$2" >"$2_sorted" &&
	test_cmp "$1_sorted" "$2_sorted"
}

# Same as test_cmp above, but we standardize directory
# separators before comparing the files.
test_path_cmp() {
	sed -e "s/\\\\/\//g" "$1" >"$1_std" &&
	sed -e "s/\\\\/\//g" "$2" >"$2_std" &&
	test_cmp "$1_std" "$2_std"
}

# Docker

# This takes a Dockerfile, and a build context directory
docker_build() {
    docker build --rm -f "$1" "$2" | ansi_strip
}

# This takes an image as argument and writes a docker ID on stdout
docker_run() {
    docker run -d "$1"
}

# This takes a docker ID and a command as arguments
docker_exec() {
    docker exec -t "$1" /bin/sh -c "$2"
}

# This takes a docker ID as argument
docker_stop() {
    docker stop "$1"
}

# This takes a docker ID as argument
docker_rm() {
    docker rm -f -v "$1" > /dev/null
}

# This takes a docker image name as argument
docker_rmi() {
    docker rmi -f "$1" > /dev/null
}

# Test whether all the expected lines are included in a file. The file
# can have extra lines.
#
# $1 - Path to file with expected lines.
# $2 - Path to file with actual output.
#
# Examples
#
#   test_expect_success 'foo says hello' '
#       echo hello >expected &&
#       foo >actual &&
#       test_cmp expected actual
#   '
#
# Returns the exit code of the command set by TEST_CMP.
test_includes_lines() {
	sort "$1" >"$1_sorted" &&
	sort "$2" >"$2_sorted" &&
	comm -2 -3 "$1_sorted" "$2_sorted" >"$2_missing" &&
	[ ! -s "$2_missing" ] || test_fsh comm -2 -3 "$1_sorted" "$2_sorted"
}

# Depending on GNU seq availability is not nice.
# Git also has test_seq but it uses Perl.
test_seq() {
	test "$1" -le "$2" || return
	i="$1"
	j="$2"
	while test "$i" -le "$j"
	do
		echo "$i"
		i=$(expr "$i" + 1)
	done
}

b64decode() {
    case `uname` in
        Linux|FreeBSD) base64 -d ;;
        Darwin) base64 -D ;;
        *)
            echo "no compatible base64 command found" >&2
            return 1
    esac
}
