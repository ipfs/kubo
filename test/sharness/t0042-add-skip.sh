#!/usr/bin/env bash
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test add and cat commands"

. lib/test-lib.sh

test_add_skip() {

  test_expect_success "'ipfs add -r' with hidden file succeeds" '
    mkdir -p mountdir/planets/.asteroids &&
    echo "mars.txt" >mountdir/planets/.gitignore &&
    echo "Hello Mars" >mountdir/planets/mars.txt &&
    echo "Hello Venus" >mountdir/planets/venus.txt &&
    echo "Hello Pluto" >mountdir/planets/.pluto.txt &&
    echo "Hello Charon" >mountdir/planets/.charon.txt &&
    echo "Hello Ceres" >mountdir/planets/.asteroids/ceres.txt &&
    echo "Hello Pallas" >mountdir/planets/.asteroids/pallas.txt &&
    ipfs add -r mountdir/planets >actual
  '

  test_expect_success "'ipfs add -r' did not include . files" '
    cat >expected <<-\EOF &&
added QmZy3khu7qf696i5HtkgL2NotsCZ8wzvNZJ1eUdA5n8KaV planets/mars.txt
added QmQnv4m3Q5512zgVtpbJ9z85osQrzZzGRn934AGh6iVEXz planets/venus.txt
added QmR8nD1Vzk5twWVC6oShTHvv7mMYkVh6dApCByBJyV2oj3 planets
EOF
    test_cmp expected actual
  '

  test_expect_success "'ipfs add -r --hidden' succeeds" '
    ipfs add -r --hidden mountdir/planets >actual
  '

  test_expect_success "'ipfs add -r --hidden' did include . files" '
    cat >expected <<-\EOF &&
added QmcAREBcjgnUpKfyFmUGnfajA1NQS5ydqRp7WfqZ6JF8Dx planets/.asteroids/ceres.txt
added QmZ5eaLybJ5GUZBNwy24AA9EEDTDpA4B8qXnuN3cGxu2uF planets/.asteroids/pallas.txt
added QmaowqjedBkUrMUXgzt9c2ZnAJncM9jpJtkFfgdFstGr5a planets/.charon.txt
added QmPHrRjTH8FskN3C2iv6BLekDT94o23KSL2u5qLqQqGhVH planets/.gitignore
added QmU4zFD5eJtRBsWC63AvpozM9Atiadg9kPVTuTrnCYJiNF planets/.pluto.txt
added QmZy3khu7qf696i5HtkgL2NotsCZ8wzvNZJ1eUdA5n8KaV planets/mars.txt
added QmQnv4m3Q5512zgVtpbJ9z85osQrzZzGRn934AGh6iVEXz planets/venus.txt
added Qmf6rbs5GF85anDuoxpSAdtuZPM9D2Yt3HngzjUVSQ7kDV planets/.asteroids
added QmczhHaXyb3bc9APMxe4MXbr87V5YDLKLaw3DZX3fK7HrK planets
EOF
    test_cmp expected actual
  '

  test_expect_success "'ipfs add -r --ignore-rules-path=.gitignore --hidden' succeeds" '
    (cd mountdir/planets && ipfs add -r --ignore-rules-path=.gitignore --hidden .) > actual
  '

  test_expect_success "'ipfs add -r --ignore-rules-path=.gitignore --hidden' did not include mars.txt file" '
    cat >expected <<-\EOF &&
added QmcAREBcjgnUpKfyFmUGnfajA1NQS5ydqRp7WfqZ6JF8Dx planets/.asteroids/ceres.txt
added QmZ5eaLybJ5GUZBNwy24AA9EEDTDpA4B8qXnuN3cGxu2uF planets/.asteroids/pallas.txt
added QmaowqjedBkUrMUXgzt9c2ZnAJncM9jpJtkFfgdFstGr5a planets/.charon.txt
added QmPHrRjTH8FskN3C2iv6BLekDT94o23KSL2u5qLqQqGhVH planets/.gitignore
added QmU4zFD5eJtRBsWC63AvpozM9Atiadg9kPVTuTrnCYJiNF planets/.pluto.txt
added QmQnv4m3Q5512zgVtpbJ9z85osQrzZzGRn934AGh6iVEXz planets/venus.txt
added Qmf6rbs5GF85anDuoxpSAdtuZPM9D2Yt3HngzjUVSQ7kDV planets/.asteroids
added QmaRsiaCYvc65RqHVAcv2tqyjZgQYgvaNqW1tQGsjfy4N5 planets
EOF
    test_cmp expected actual
  '

  test_expect_success "'ipfs add -r --ignore-rules-path=.gitignore --ignore .asteroids --ignore venus.txt --hidden' succeeds" '
    (cd mountdir/planets && ipfs add -r --ignore-rules-path=.gitignore --ignore .asteroids --ignore venus.txt --hidden .) > actual
  '

  test_expect_success "'ipfs add -r --ignore-rules-path=.gitignore --ignore .asteroids --ignore venus.txt --hidden' did not include ignored files" '
    cat >expected <<-\EOF &&
added QmaowqjedBkUrMUXgzt9c2ZnAJncM9jpJtkFfgdFstGr5a planets/.charon.txt
added QmPHrRjTH8FskN3C2iv6BLekDT94o23KSL2u5qLqQqGhVH planets/.gitignore
added QmU4zFD5eJtRBsWC63AvpozM9Atiadg9kPVTuTrnCYJiNF planets/.pluto.txt
added QmemuMahjSh7eYLY3hbz2q8sqMPnbQzBQeUdosqNiWChE6 planets
EOF
    test_cmp expected actual
  '

  test_expect_success "'ipfs add' includes hidden files given explicitly even without --hidden" '
    mkdir -p mountdir/dotfiles &&
    echo "set nocompatible" > mountdir/dotfiles/.vimrc
    cat >expected <<-\EOF &&
added QmT4uMRDCN7EMpFeqwvKkboszbqeW1kWVGrBxBuCGqZcQc .vimrc
EOF
    ipfs add mountdir/dotfiles/.vimrc >actual
    cat actual
    test_cmp expected actual
  '

}

# should work offline
test_init_ipfs
test_add_skip

# should work online
test_launch_ipfs_daemon
test_add_skip
test_kill_ipfs_daemon

test_done
