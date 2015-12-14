# Generic test functions for go-ipfs

# Echo the args, run the cmd, and then also fail,
# making sure a test case fails.
test_fsh() {
    echo "> $@"
    eval "$@"
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

# Quote arguments for sh eval
shellquote() {
	_space=''
	for _arg
	do
		# On Mac OS, sed adds a newline character.
		# With a printf wrapper the extra newline is removed.
		printf "$_space'%s'" "$(printf "%s" "$_arg" | sed -e "s/'/'\\\\''/g;")"
		_space=' '
	done
	printf '\n'
}

# Docker

# This takes a directory, that should contain a Dockerfile, as argument
docker_build() {
    docker build --rm "$1"
}

# This takes an image as argument and writes a docker ID on stdout
docker_run() {
    docker run -it -d -p 8080:8080 -p 4001:4001 -p 5001:5001 "$1"
}

# This takes a docker ID and a command as arguments
docker_exec() {
    if test "$CIRCLE" = 1
    then
        sudo lxc-attach -n "$(docker inspect --format '{{.Id}}' $1)" -- /bin/bash -c "$2"
    else
	docker exec -i "$1" /bin/bash -c "$2"
    fi
}

# This takes a docker ID as argument
docker_stop() {
    docker stop "$1"
}
