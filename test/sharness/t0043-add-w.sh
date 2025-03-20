#!/usr/bin/env bash
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test add -w"

add_w_m='QmbDfuW3tZ5PmAucyLBAMzVeETHCHM7Ho9CWdBvWxRGd3i'

add_w_1='added QmP9WCV5SjQRoxoCkgywzw4q5X23rhHJJXzPQt4VbNa9M5 0h0r91
added Qmave82G8vLbtx6JCokrrhLPpFNfWj5pbXobddiUASfpe3 '

add_w_12='added QmP9WCV5SjQRoxoCkgywzw4q5X23rhHJJXzPQt4VbNa9M5 0h0r91
added QmNUiT9caQy5zXvw942UYXkjLseQLWBkf7ZJD6RCfk8JgP 951op
added QmWXoq9vUtdNxmM16kvJRgyQdi4S4gfYSjd2MsRprBXWmG '

add_w_d1='added QmQKZCZKKL71zcMNpFFVcWzoh5dimX45mKgUu3LhvdaCRn 3s78oa/cb5v5v
added QmPng2maSno8o659Lu2QtKg2d2L53RMahoyK6wNkifYaxY 3s78oa/cnd062l-rh
added QmX3s7jJjFQhKRuGpDA3W4BYHdCWAyL3oB6U3iSoaYxVxs 3s78oa/es3gm9ck7b
added QmSUZXb48DoNjUPpX9Jue1mUpyCghEDZY62iif1JhdofoG 3s78oa/kfo77-6i_hp0ttz
added QmdC215Wp2sH47aw6R9CLBVa5uxJB4zEag1gtsKqjYGDb5 3s78oa/p91vs5t
added QmSEGJRYb5wrJRBxNsse91YJSpmgf5ikKRtCwvGZ1V1Nc2 3s78oa
added QmS2ML7DPVisc4gQtSrwMi3qwS9eyzGR7zVdwqwRPU9rGz '

add_w_d1_v1='added bafkreibpfapmbmf55elpipnoofmda7xbs5spthba2srrovnchttzplmrnm fvmq97/0vz12t0yf
added bafkreihc5hdzpjwbqy6b5r2h2oxbm6mp4sx4eqll253k6f5yijsismvoxy fvmq97/2hpfk8slf0
added bafkreihlmwk6pkk7klsmypmk2wfkgijbk7wavhtrcvgrfxvug7x5ndawge fvmq97/nda000755cd76
added bafkreigpntro6bt4m6c5pcnmvk24qyiq3lwffhwry7k2hqtretqhfsfvqa fvmq97/nsz0wsonz
added bafkreieeznfvzr6742npktcn4ajzxujst6j2uztwfninhvic4bbvm356u4 fvmq97/pq3f6t0
added bafybeiatm3oos62mm5hu4cmq234wipw2fjaqflq2cdqgc6i6dcgzamxwrm fvmq97
added bafybeifp4ioszjk2377psexdhk7thcxnpaj2wls4yifsntbgxzti7ds4uy '

add_w_d2='added QmP9WCV5SjQRoxoCkgywzw4q5X23rhHJJXzPQt4VbNa9M5 0h0r91
added QmPpv7rFgkBqMYKJok6kVixqJgAGkyPiX3Jrr7n9rU1gcv 1o8ef-25onywi
added QmW7zDxGpaJTRpte7uCvMA9eXJ5L274FfsFPK9pE5RShq9 2ju9tn-b09/-qw1d8j9
added QmNNm9D3pn8NXbuYSde614qbb9xE67g9TNV6zXePgSZvHj 2ju9tn-b09/03rfc61t4qq_m
added QmUYefaFAWka9LWarDeetQFe8CCSHaAtj4JR7YToYPSJyi 2ju9tn-b09/57dl-1lbjvu
added QmcMLvVinwJsHtYxTUXEoPd8XkbuyvJNffZ85PT11cWDc2 2ju9tn-b09/t8h1_w
added QmUTZE57VoF7xqWmrrcDNtDXrEs6znTQaRwmwkawGDs1GA 2ju9tn-b09/ugqi0nmv-1
added QmfX5q9CMquL4JnAuG4H13RXjTb9DncMfu9pvpEsWkECJk fvmq97/0vz12t0yf
added Qmdr3jR1UATLFeuoieBTHLNNwhCUJbgN5oat7U9X8TtfdZ fvmq97/2hpfk8slf0
added QmfUKgXSiE1wCQuX3Pws9FftthJuAMXrDWhG5EhhnmA6gQ fvmq97/nda000755cd76
added QmYM35pgHvLdKH8ssw9kJeiUY5kcjhb5h3BTiDhAgbsYYh fvmq97/nsz0wsonz
added QmNarBSVwzYjLeEjGMJqTNtRCYGCLGo6TJqd21hPi7WXFT fvmq97/pq3f6t0
added QmUNhQpFBZvfH4JyNxiE8QY31bZDpQHMmjSRRnbRZYZ3be 2ju9tn-b09
added QmWtZu8dv4XRK8zPmwbNjS6biqe4bGEF9J5zb51sBJCMro fvmq97
added QmYp7QoL8wRacLn9pJftJSkiiSmNGdWb7qT5ENDW2HXBcu '

add_w_r='QmUerh2irM8cngqJHLGKCn4AGBSyHYAUi8i8zyVzXKNYyb'

. lib/test-lib.sh

test_add_w() {

  test_expect_success "random-files is installed" '
    type random-files
  '

  test_expect_success "random-files generates test files" '
    random-files --seed 7547632 --files 5 --dirs 2 --depth 3 m &&
    echo "$add_w_m" >expected &&
    ipfs add -Q -r m >actual &&
    test_sort_cmp expected actual
  '

  # test single file
  test_expect_success "ipfs add -w (single file) succeeds" '
    ipfs add -w m/0h0r91 >actual
  '

  test_expect_success "ipfs add -w (single file) is correct" '
    echo "$add_w_1" >expected &&
    test_sort_cmp expected actual
  '

  # test two files together
  test_expect_success "ipfs add -w (multiple) succeeds" '
    ipfs add -w m/0h0r91 m/951op >actual
  '

  test_expect_success "ipfs add -w (multiple) is correct" '
    echo "$add_w_12" >expected  &&
    test_sort_cmp expected actual
  '

  test_expect_success "ipfs add -w (multiple) succeeds" '
    ipfs add -w m/951op m/0h0r91 >actual
  '

  test_expect_success "ipfs add -w (multiple) orders" '
    echo "$add_w_12" >expected  &&
    test_sort_cmp expected actual
  '

  # test a directory
  test_expect_success "ipfs add -w -r (dir) succeeds" '
    ipfs add -r -w m/9m7mh3u51z3b/3s78oa >actual
  '

  test_expect_success "ipfs add -w -r (dir) is correct" '
    echo "$add_w_d1" >expected &&
    test_sort_cmp expected actual
  '

  # test files and directory
  test_expect_success "ipfs add -w -r <many> succeeds" '
    ipfs add -w -r m/9m7mh3u51z3b/1o8ef-25onywi \
      m/vck_-2/2ju9tn-b09 m/9m7mh3u51z3b/fvmq97 m/0h0r91 >actual
  '

  test_expect_success "ipfs add -w -r <many> is correct" '
    echo "$add_w_d2" >expected &&
    test_sort_cmp expected actual
  '

  # test -w -r m/* == -r m
  test_expect_success "ipfs add -w -r m/* == add -r m  succeeds" '
    ipfs add -Q -w -r m/* >actual
  '

  test_expect_success "ipfs add -w -r m/* == add -r m  is correct" '
    echo "$add_w_m" >expected &&
    test_sort_cmp expected actual
  '

  # test repeats together
  test_expect_success "ipfs add -w (repeats) succeeds" '
    ipfs add -Q -w -r m/9m7mh3u51z3b/1o8ef-25onywi m/vck_-2/2ju9tn-b09 \
      m/9m7mh3u51z3b/fvmq97 m/0h0r91 m/9m7mh3u51z3b m/9m7mh3u51z3b m/0h0r91 \
      m/0h0r91 m/vck_-2/0dl083je2 \
      m/vck_-2/2ju9tn-b09/-qw1d8j9 >actual
  '

  test_expect_success "ipfs add -w (repeats) is correct" '
    echo "$add_w_r" >expected  &&
    test_sort_cmp expected actual
  '

  test_expect_success "ipfs add -w -r (dir) --cid-version=1 succeeds" '
    ipfs add -r -w --cid-version=1 m/9m7mh3u51z3b/fvmq97 >actual
  '

  test_expect_success "ipfs add -w -r (dir) --cid-version=1 is correct" '
    echo "$add_w_d1_v1" >expected &&
    test_sort_cmp expected actual
  '

  test_expect_success "ipfs add -w -r -n (dir) --cid-version=1 succeeds" '
    ipfs add -r -w -n --cid-version=1 m/9m7mh3u51z3b/fvmq97 >actual
  '

  test_expect_success "ipfs add -w -r -n (dir) --cid-version=1 is correct" '
    echo "$add_w_d1_v1" > expected &&
    test_sort_cmp expected actual
  '
}

test_init_ipfs

test_add_w

test_launch_ipfs_daemon

test_add_w

test_kill_ipfs_daemon

test_done
