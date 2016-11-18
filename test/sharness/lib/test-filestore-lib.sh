client_err() {
    printf "$@\n\nUse 'ipfs add --help' for information about this command\n"
}


test_enable_filestore() {
    test_expect_success "enable filestore" '
        ipfs filestore enable
    '
}

test_add_cat_file() {
    cmd=$1
    dir=$2
    HASH=$3 # QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH
    
    test_expect_success "ipfs $cmd succeeds" '
    	echo "Hello Worlds!" >mountdir/hello.txt &&
        ipfs $cmd "$dir"/mountdir/hello.txt >actual
    '

    test_expect_success "ipfs $cmd output looks good" '
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

test_add_empty_file() {
    cmd=$1
    dir=$2

    EMPTY_HASH="QmbFMke1KXqnYyBBWxB74N4c5SBnJMVAiMNRcGu6x1AwQH"

    test_expect_success "ipfs $cmd on empty file succeeds" '
        ipfs block rm -f $EMPTY_HASH &&
        cat /dev/null >mountdir/empty.txt &&
        ipfs $cmd "$dir"/mountdir/empty.txt >actual
    '

    test_expect_success "ipfs $cmd on empty file output looks good" '
        echo "added $EMPTY_HASH empty.txt" >expected &&
        test_cmp expected actual
    '

    test_expect_success "ipfs cat on empty file succeeds" '
        ipfs cat "$EMPTY_HASH" >actual
    '

    test_expect_success "ipfs cat on empty file output looks good" '
        cat /dev/null >expected &&
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
    HASH=$3 # "QmSr7FqYkxYWGoSfy8ZiaMWQ5vosb18DQGCzjwEQnVHkTb"
    
    test_expect_success "generate 5MB file using go-random" '
    	random 5242880 41 >mountdir/bigfile
    '

    test_expect_success "sha1 of the file looks ok" '
    	echo "11145620fb92eb5a49c9986b5c6844efda37e471660e" >sha1_expected &&
    	multihash -a=sha1 -e=hex mountdir/bigfile >sha1_actual &&
    	test_cmp sha1_expected sha1_actual
    '

    test_expect_success "'ipfs $cmd bigfile' succeeds" '
    	ipfs $cmd "$dir"/mountdir/bigfile >actual
    '

    test_expect_success "'ipfs $cmd bigfile' output looks good" '
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

test_add_cat_200MB() {
    cmd=$1
    dir=$2
    HASH=$3 #"QmVbVLFLbz72tRSw3HMBh6ABKbRVavMQLoh2BzQ4dUSAYL"
    
    test_expect_success "generate 200MB file using go-random" '
    	random 209715200 41 >mountdir/hugefile
    '

    test_expect_success "sha1 of the file looks ok" '
    	echo "11146a3985bff32699f1874517ad0585bbd280efc1de" >sha1_expected &&
    	multihash -a=sha1 -e=hex mountdir/hugefile >sha1_actual &&
    	test_cmp sha1_expected sha1_actual
    '

    test_expect_success "'ipfs add hugefile' succeeds" '
    	ipfs $cmd "$dir"/mountdir/hugefile >actual
    '

    test_expect_success "'ipfs add hugefile' output looks good" '
    	echo "added $HASH hugefile" >expected &&
    	test_cmp expected actual
    '

    test_expect_success "'ipfs cat' succeeds" '
    	ipfs cat "$HASH" >actual
    '

    test_expect_success "'ipfs cat' output looks good" '
    	test_cmp mountdir/hugefile actual
    '

    test_expect_success "fail after file rm" '
        rm mountdir/hugefile actual &&
        test_must_fail ipfs cat "$HASH" >/dev/null
    '
}

test_add_mulpl_files() {
    cmd=$1

    test_expect_success "generate directory with several files" '
        mkdir adir &&
        echo "file1" > adir/file1 &&
        echo "file2" > adir/file2 &&
        echo "file3" > adir/file3
    '

    dir="`pwd`"/adir
    test_expect_success "add files by listing them all on command line" '
        ipfs $cmd "$dir"/file1 "$dir"/file2 "$dir"/file3 > add-expect
    '

    test_expect_success "all files added" '
        grep file1 add-expect &&
        grep file2 add-expect &&
        grep file3 add-expect
    '

    test_expect_success "cleanup" '
        rm -r adir
    '
}

