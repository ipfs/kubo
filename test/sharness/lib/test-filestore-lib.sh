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

test_add_empty_file() {
    cmd=$1
    dir=$2

    EMPTY_HASH="QmbFMke1KXqnYyBBWxB74N4c5SBnJMVAiMNRcGu6x1AwQH"

    test_expect_success "ipfs add on empty file succeeds" '
        ipfs block rm -f $EMPTY_HASH &&
        cat /dev/null >mountdir/empty.txt &&
        ipfs $cmd "$dir"/mountdir/empty.txt >actual
    '

    test_expect_success "ipfs add on empty file output looks good" '
        echo "added $EMPTY_HASH "$dir"/mountdir/empty.txt" >expected &&
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

test_add_symlinks() {
    opt=$1

    test_expect_success "creating files with symbolic links succeeds" '
        rm -rf files &&
        mkdir -p files/foo &&
        mkdir -p files/bar &&
        echo "some text" > files/foo/baz &&
        ln -s files/foo/baz files/bar/baz &&
        ln -s files/does/not/exist files/bad
    '

    test_expect_success "adding a symlink adds the link itself" '
        ipfs filestore add --logical -q $opt files/bar/baz > goodlink_out
    '

    test_expect_success "output looks good" '
        echo "QmdocmZeF7qwPT9Z8SiVhMSyKA2KKoA2J7jToW6z6WBmxR" > goodlink_exp &&
        test_cmp goodlink_exp goodlink_out
    '

    test_expect_success "adding a broken symlink works" '
        ipfs filestore add --logical -q  $opt files/bad > badlink_out
    '

    test_expect_success "output looks good" '
        echo "QmWYN8SEXCgNT2PSjB6BnxAx6NJQtazWoBkTRH9GRfPFFQ" > badlink_exp &&
        test_cmp badlink_exp badlink_out
    '
}

test_add_symlinks_fails_cleanly() {
    opt=$1

    test_expect_success "creating files with symbolic links succeeds" '
        rm -rf files &&
        mkdir -p files/foo &&
        mkdir -p files/bar &&
        echo "some text" > files/foo/baz &&
        ln -s files/foo/baz files/bar/baz &&
        ln -s files/does/not/exist files/bad
    '

    test_expect_success "adding a symlink fails cleanly" '
        test_must_fail ipfs filestore add --logical -q $opt files/bar/baz > goodlink_out
    '

	test_expect_success "ipfs daemon did not crash" '
		kill -0 $IPFS_PID
	'

    test_expect_success "adding a broken link fails cleanly" '
        test_must_fail ipfs filestore add --logical -q  $opt files/bad > badlink_out
    '

	test_expect_success "ipfs daemon did not crash" '
		kill -0 $IPFS_PID
	'
}

test_add_dir_w_symlinks() {
    opt=$1

    test_expect_success "adding directory with symlinks in it works" '
        ipfs filestore add --logical -q -r $opt files/ > dirlink_out
    '
}

filestore_test_w_daemon() {
    opt=$1

    test_init_ipfs

    test_launch_ipfs_daemon $opt

    test_expect_success "can't enable filestore while daemon is running" '
        test_must_fail ipfs filestore enable
    '

    test_kill_ipfs_daemon

    test_enable_filestore

    test_launch_ipfs_daemon $opt

    test_add_cat_file "filestore add " "`pwd`"

    test_post_add "filestore add " "`pwd`"

    test_add_empty_file "filestore add " "`pwd`"

    test_add_cat_5MB "filestore add " "`pwd`"

    test_add_mulpl_files "filestore add "

    test_expect_success "testing filestore add -r should fail" '
      mkdir adir &&
      echo "Hello Worlds!" > adir/file1 &&
      echo "HELLO WORLDS!" > adir/file2 &&
      random 5242880 41 > adir/file3 &&
      test_must_fail ipfs filestore add -r "`pwd`/adir"
    '
    rm -rf adir

    test_add_symlinks_fails_cleanly

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

#     test_expect_success "enable Filestore.APIServerSidePaths" '
#       ipfs config Filestore.APIServerSidePaths --bool true
#     '

#     test_launch_ipfs_daemon $opt

#     test_add_cat_file "filestore add -S" "`pwd`"

#     test_post_add "filestore add -S" "`pwd`"

#     test_add_empty_file "filestore add -S" "`pwd`"

#     test_add_cat_5MB "filestore add -S" "`pwd`"

#     test_add_mulpl_files "filestore add -S"

#     cat <<EOF > add_expect
# added QmQhAyoEzSg5JeAzGDCx63aPekjSGKeQaYs4iRf4y6Qm6w adir
# added QmSr7FqYkxYWGoSfy8ZiaMWQ5vosb18DQGCzjwEQnVHkTb `pwd`/adir/file3
# added QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH `pwd`/adir/file1
# added QmZm53sWMaAQ59x56tFox8X9exJFELWC33NLjK6m8H7CpN `pwd`/adir/file2
# EOF

#     test_expect_success "testing filestore add -S -r" '
#       mkdir adir &&
#       echo "Hello Worlds!" > adir/file1 &&
#       echo "HELLO WORLDS!" > adir/file2 &&
#       random 5242880 41 > adir/file3 &&
#       ipfs filestore add -S -r "`pwd`/adir" | LC_ALL=C sort > add_actual &&
#       test_cmp add_expect add_actual &&
#       ipfs cat QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH > cat_actual
#       test_cmp adir/file1 cat_actual
#     '

#     test_expect_success "filestore mv" '
#       HASH=QmQHRQ7EU8mUXLXkvqKWPubZqtxYPbwaqYo6NXSfS9zdCc &&
#       test_must_fail ipfs filestore mv $HASH "mountdir/bigfile-42-also" &&
#       ipfs filestore mv $HASH "`pwd`/mountdir/bigfile-42-also"
#     '

#     filestore_test_exact_paths '-S'

#     test_add_symlinks '-S'

#     test_add_dir_w_symlinks '-S'

#     test_kill_ipfs_daemon

}
