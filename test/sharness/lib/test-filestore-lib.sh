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
        echo "added $HASH "$dir"/mountdir/hello.txt" >expected &&
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
    	echo "added $HASH "$dir"/mountdir/bigfile" >expected &&
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
    	HASH="QmVbVLFLbz72tRSw3HMBh6ABKbRVavMQLoh2BzQ4dUSAYL" &&
    	echo "added $HASH "$dir"/mountdir/hugefile" >expected &&
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

filestore_test_exact_paths() {
    opt=$1

    test_expect_success "prep for path checks" '
      mkdir mydir &&
      ln -s mydir dirlink &&
      echo "Hello Worlds!" > dirlink/hello.txt
    '

    test_expect_success "ipfs filestore add $opts adds under the expected path name (with symbolic links)" '
      FILEPATH="`pwd`/dirlink/hello.txt" &&
      ipfs filestore add $opt "$FILEPATH" &&
      echo "$FILEPATH" > ls-expected &&
      ipfs filestore ls-files -q QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH > ls-actual &&
      test_cmp ls-expected ls-actual
    '

    test_expect_success "ipfs filestore ls dirlink/ works as expected" '
      echo "QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH" > ls-expected
      ipfs filestore ls -q "`pwd`/dirlink/" > ls-actual
      test_cmp ls-expected ls-actual
    '

    test_expect_success "ipfs filestore add $opts --physical works as expected" '
      ipfs filestore rm QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH &&
      ( cd dirlink &&
        ipfs filestore add $opt --physical hello.txt
        FILEPATH="`pwd -P`/hello.txt" &&
        echo "$FILEPATH" > ls-expected &&
        ipfs filestore ls-files -q QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH > ls-actual &&
        test_cmp ls-expected ls-actual )
    '

    test_expect_success "ipfs filestore add $opts --logical works as expected" '
      ipfs filestore rm QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH &&
      ( cd dirlink &&
        ipfs filestore add $opt --logical hello.txt
        FILEPATH="`pwd -L`/hello.txt" &&
        echo "$FILEPATH" > ls-expected &&
        ipfs filestore ls-files -q QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH > ls-actual &&
        test_cmp ls-expected ls-actual )
    '

    test_expect_success "cleanup from path checks" '
      ipfs filestore rm QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH &&
      rm -rf mydir
    '
}

filestore_test_w_daemon() {
    opt=$1

    test_init_ipfs

    test_launch_ipfs_daemon $opt

    test_add_cat_file "filestore add " "`pwd`"

    test_post_add "filestore add " "`pwd`"

    test_add_cat_5MB "filestore add " "`pwd`"

    filestore_test_exact_paths

    test_expect_success "ipfs add -S fails unless enable" '
      echo "Hello Worlds!" >mountdir/hello.txt &&
      test_must_fail ipfs filestore add -S "`pwd`"/mountdir/hello.txt >actual
    '

    test_expect_success "filestore mv should fail" '
      HASH=QmQHRQ7EU8mUXLXkvqKWPubZqtxYPbwaqYo6NXSfS9zdCc &&
      random 5242880 42 >mountdir/bigfile-42 &&
      ipfs add mountdir/bigfile-42 &&
      test_must_fail ipfs filestore mv $HASH "`pwd`/mountdir/bigfile-42-also"
    '

    test_kill_ipfs_daemon

    test_expect_success "clean filestore" '
      ipfs filestore ls -q | xargs ipfs filestore rm &&
      test -z "`ipfs filestore ls -q`"
    '

    test_expect_success "enable Filestore.APIServerSidePaths" '
      ipfs config Filestore.APIServerSidePaths --bool true
    '

    test_launch_ipfs_daemon $opt

    test_add_cat_file "filestore add -S" "`pwd`"

    test_post_add "filestore add -S" "`pwd`"

    test_add_cat_5MB "filestore add -S" "`pwd`"

    cat <<EOF > add_expect
added QmQhAyoEzSg5JeAzGDCx63aPekjSGKeQaYs4iRf4y6Qm6w adir
added QmSr7FqYkxYWGoSfy8ZiaMWQ5vosb18DQGCzjwEQnVHkTb `pwd`/adir/file3
added QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH `pwd`/adir/file1
added QmZm53sWMaAQ59x56tFox8X9exJFELWC33NLjK6m8H7CpN `pwd`/adir/file2
EOF

    test_expect_success "testing filestore add -S -r" '
      mkdir adir &&
      echo "Hello Worlds!" > adir/file1 &&
      echo "HELLO WORLDS!" > adir/file2 &&
      random 5242880 41 > adir/file3 &&
      ipfs filestore add -S -r "`pwd`/adir" | LC_ALL=C sort > add_actual &&
      test_cmp add_expect add_actual &&
      ipfs cat QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH > cat_actual
      test_cmp adir/file1 cat_actual
    '

    test_expect_success "filestore mv" '
      HASH=QmQHRQ7EU8mUXLXkvqKWPubZqtxYPbwaqYo6NXSfS9zdCc &&
      test_must_fail ipfs filestore mv $HASH "mountdir/bigfile-42-also" &&
      ipfs filestore mv $HASH "`pwd`/mountdir/bigfile-42-also"
    '

    filestore_test_exact_paths '-S'

    test_kill_ipfs_daemon

}
