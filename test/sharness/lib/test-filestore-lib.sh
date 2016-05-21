client_err() {
    printf "$@\n\nUse 'ipfs add --help' for information about this command\n"
}

test_add_cat_file() {
    cmd=$1
    dir=$2
    
    test_expect_success "ipfs add succeeds" '
    	echo "Hello Worlds!" >mountdir/hello.txt &&
        ipfs $cmd "$dir"/mountdir/hello.txt >actual
    '

    test_expect_success "ipfs add output looks good" '
    	HASH="QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH" &&
        echo "added $HASH hello.txt" >expected &&
    	test_cmp expected actual
    '

    test_expect_success "ipfs cat succeeds" '
    	ipfs cat "$HASH" >actual
    '

    test_expect_success "ipfs cat output looks good" '
    	echo "Hello Worlds!" >expected &&
    	test_cmp expected actual
    '
}

test_post_add() {
    cmd=$1
    dir=$2
    
    test_expect_success "fail after file move" '
        mv mountdir/hello.txt mountdir/hello2.txt
        test_must_fail ipfs cat "$HASH" >/dev/null
    '

    test_expect_success "okay again after moving back" '
        mv mountdir/hello2.txt mountdir/hello.txt &&
        ipfs cat "$HASH" >/dev/null
    '

    test_expect_success "fail after file move" '
        mv mountdir/hello.txt mountdir/hello2.txt
        test_must_fail ipfs cat "$HASH" >/dev/null
    '

    test_expect_success "okay after re-adding under new name" '
        ipfs $cmd "$dir"/mountdir/hello2.txt 2> add.output &&
        ipfs cat "$HASH" >/dev/null
    '

    test_expect_success "restore state" '
        mv mountdir/hello2.txt mountdir/hello.txt &&
        ipfs $cmd "$dir"/mountdir/hello.txt 2> add.output &&
        ipfs cat "$HASH" >/dev/null
    '

    test_expect_success "fail after file change" '
        # note: filesize shrinks
        echo "hello world!" >mountdir/hello.txt &&
        test_must_fail ipfs cat "$HASH" >cat.output
    '

    test_expect_success "fail after file change, same size" '
        # note: filesize does not change
        echo "HELLO WORLDS!" >mountdir/hello.txt &&
        test_must_fail ipfs cat "$HASH" >cat.output
    '
}

test_add_cat_5MB() {
    cmd=$1
    dir=$2
    
    test_expect_success "generate 5MB file using go-random" '
    	random 5242880 41 >mountdir/bigfile
    '

    test_expect_success "sha1 of the file looks ok" '
    	echo "11145620fb92eb5a49c9986b5c6844efda37e471660e" >sha1_expected &&
    	multihash -a=sha1 -e=hex mountdir/bigfile >sha1_actual &&
    	test_cmp sha1_expected sha1_actual
    '

    test_expect_success "'ipfs add bigfile' succeeds" '
    	ipfs $cmd "$dir"/mountdir/bigfile >actual
    '

    test_expect_success "'ipfs add bigfile' output looks good" '
    	HASH="QmSr7FqYkxYWGoSfy8ZiaMWQ5vosb18DQGCzjwEQnVHkTb" &&
    	echo "added $HASH bigfile" >expected &&
    	test_cmp expected actual
    '

    test_expect_success "'ipfs cat' succeeds" '
    	ipfs cat "$HASH" >actual
    '

    test_expect_success "'ipfs cat' output looks good" '
    	test_cmp mountdir/bigfile actual
    '
}
