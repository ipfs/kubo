#!/usr/bin/env bash
#
# Copyright (c) 2018 Kejie Zhang
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test add --name"

add_name_m='QmazHkwx6mPmmCEi1jR5YzjjQd1g5XzKfYQLzRAg7x5uUk'

add_name_1='added Qme987pqNBhZZXy4ckeXiR7zaRQwBabB7fTgHurW2yJfNu 4r93'

add_name_2='added Qme987pqNBhZZXy4ckeXiR7zaRQwBabB7fTgHurW2yJfNu 4r93
added Qmf82PSsMpUHcrqxa69KG6Qp5yeK7K9BTizXgG3nvzWcNG '

add_name_3='added Qme987pqNBhZZXy4ckeXiR7zaRQwBabB7fTgHurW2yJfNu myfile.txt
added QmZbStPUUoRr1hA9GZyKx7pyskZvCczPrf6XSK6A9HSr1i '

add_name_4='added Qme987pqNBhZZXy4ckeXiR7zaRQwBabB7fTgHurW2yJfNu myfile.txt'

. lib/test-lib.sh

test_add_name() {

  test_expect_success "go-random-files is installed" '
    type random-files
  '

  test_expect_success "random-files generates test files" '
    random-files --seed 7547632 --files 5 --dirs 2 --depth 3 m &&
    echo "$add_name_m" >expected &&
    ipfs add -q -r m | tail -n1 >actual &&
    echo $actual
    test_sort_cmp expected actual
  '

  # test --name without -w
  test_expect_success "ipfs add --name is correct" '
    echo "$add_name_1" >expected &&
    ipfs add m/4r93 --name myfile.txt >actual
    test_sort_cmp expected actual
  '

  # test --name with -w
  test_expect_success "ipfs add -w --name is correct" '
    echo "$add_name_2" >expected &&
    ipfs add m/4r93 -w --name myfile.txt >actual
    test_sort_cmp expected actual
  '

  # test --name with -w and cat
    test_expect_success "cat file | ipfs add -w --name is correct" '
      echo "$add_name_3" >expected &&
      cat m/4r93 | ipfs add -w --name myfile.txt >actual
      test_sort_cmp expected actual
  '

  # test --name with cat but without -w
      test_expect_success "cat file | ipfs add --name is correct" '
      echo "$add_name_4" >expected &&
      cat m/4r93 | ipfs add --name myfile.txt >actual
      test_sort_cmp expected actual
  '
}

test_init_ipfs

test_add_name

test_launch_ipfs_daemon

test_add_name

test_kill_ipfs_daemon

test_done
