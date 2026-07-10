#!/usr/bin/env bash
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test add -w"

add_w_m='QmfQvQTXHsyYienvGRssPwMVkKnKQwwzLu6ZVw7PXwNyNW'

add_w_1='added QmbjYHUDk1Gyo4AsW3fvYNQKFPYBiouf3ZXsWj3jKM7qJb 5gl-49d-hiv
added QmRMHDN3yQEVaBoqZTFJzzLCJKM7cXjrwC1vPre4ytAGJY '

add_w_12='added QmbjYHUDk1Gyo4AsW3fvYNQKFPYBiouf3ZXsWj3jKM7qJb 5gl-49d-hiv
added QmUcd1kxjfCtTzikaUoiBxwcpDLP3LZPm9ZVQGYcvju9Ek 5sbh
added QmSjuqiWnbecpY4s1bVsPZcDDUSCkueYG5dqHrD1fgyE9j '

add_w_d1='added QmYnSe2PFiNZJa7cYC8RgR9cYwQvGpJ2nPzPSMbUQ5oUPE qtdpbyso0v-4kqvv/-6ui6l5
added QmTYxpZwLSbkmJQTWg7nKCoDEtuR49VEFdPYhPm1HG55cy qtdpbyso0v-4kqvv/0c2kxfc
added QmZnsfUT5E3djaRksy3RRZyr2QigBmMQY4iiDDwxjeyas7 qtdpbyso0v-4kqvv/axh1njptsdzqprt
added QmbaQFvqv6bGN2GxBwdVDfq8TQsVqzk5NzJtxhS5KMPg49 qtdpbyso0v-4kqvv/ty6i3ekg4itu
added QmZK1KsXaca74aGy7vxGa7P7iDYHkD2dbeCKSpbDhCTWNo qtdpbyso0v-4kqvv/xdia7yqcajaa
added QmYZ1DDDJX1ctJwcR2u1s1g55NAKDzen1UEv5K2HXKiXTw qtdpbyso0v-4kqvv
added QmWExmHkW8LPWRFPG3ShA8pZ85HJyHZNdUtZ2KES6L6D2V '

add_w_d1_v1='added bafkreif5hpd5taevj7qqwa5r6lxuox7hv6kierq5kdrpzrsy3bffpk3huu zrdvq-njj/7dpcx0-gpe_q
added bafkreihumb5ymjqwlcwpszmlhrk7aijkikg52btue7nbo2l5adhedyfa3u zrdvq-njj/e-csi80v27py_
added bafkreif3cpzthr3rjpiviarjhj33tayeezl3x7qnvxzcpcjdu3d2ugusme zrdvq-njj/hlq37u
added bafkreihzwjsaxxvsijd42kwnsrqvrt2hawuorkm6vyd5umvtstlr73d7si zrdvq-njj/pfe4ior
added bafkreihisgyou3syhv3vlkvvkdcr2z4llm7mzjgqngnu5td7edd4f3rvem zrdvq-njj/tagnaif9buu3hdjb
added bafybeiftsbpfo5rqhkicc6aum7cfh7jnxxq6xndywqolu3bjrprxzqv5om zrdvq-njj
added bafybeiciybmlydumimklcfpu3djexxqkcf4xlyxicqxrzhj5fqcpib53g4 '

add_w_d2='added QmT9DXymk1mHchycqx6PjAK63LbvwHyxXGAp9y1eKLCE8v -n7bk0ea/-5d96-jaahmvel9m
added QmYxTtuyoJaJ5C1uKZKB6LnXej8q3pWd6V6kDgmdVAogcs -n7bk0ea/86j7sr
added QmdX62pVn3cH8K971NjzQc9spnsnj7EerMhf2ZyRbSXWi5 -n7bk0ea/b0t256vl-2
added QmWZuSFJZXvnuyUh3n5jyvo1sHq7iCXoxsX9nyFRrBZMeY -n7bk0ea/smfg4hh
added QmPEyjt1dMrybQ2REvn7EMdsGjotC9nZ5owV6uzgAihmwt -n7bk0ea/yyq2
added QmTifQpY79ZwD5eWVXLWxSTNNtzRjkyukfJWShHCCG6EcB 17dk
added QmbjYHUDk1Gyo4AsW3fvYNQKFPYBiouf3ZXsWj3jKM7qJb 5gl-49d-hiv
added QmPGt5rA9w2RmD4iXfj7AAXTe7arL3zAAzPVcvci9HoAEZ zrdvq-njj/7dpcx0-gpe_q
added QmTG3XNWX6TXthCgDtPde47w4AnWjY4iK3aFTjQ8coj9Pt zrdvq-njj/e-csi80v27py_
added QmQXLPzm8GsiQbmydRmN6KeT677xPMptUqg3yhpXfyZ2db zrdvq-njj/hlq37u
added QmdLTEsDGvX17mPfx81ZPcjB981ihJjRzTJdUREEocEQT9 zrdvq-njj/pfe4ior
added QmNcUreqvcEtwpSUPTGRV18jRwi79XtzvjzvWrux7BQrZF zrdvq-njj/tagnaif9buu3hdjb
added QmcmisZ5iFyP1mfg84fBiC4VCq1GQ8LSjTNUqF6Tn14Ru8 -n7bk0ea
added QmbRzhy3RhM7UAcEuKAz5iSXcPrvXAHYT92VjR6UqJPrpE zrdvq-njj
added Qma1GfvBaM6fHAPqcxD3VB3MxEPBdH6N975vu6owr94PXP '

add_w_r='QmNYozfEiFSPsB9dihjmY94obCgmpNLJVG14we2Vo4Qp5j'

. lib/test-lib.sh

test_add_w() {

  test_expect_success "random-files is installed" '
    type random-files
  '

  test_expect_success "random-files generates test files" '
    random-files --seed 7547632 --files 5 --dirs 2 --depth 3 m &&
    echo "$add_w_m" >expected &&
    ipfs add -Q -r m >actual_w_m &&
    test_sort_cmp expected actual_w_m
  '

  # test single file
  test_expect_success "ipfs add -w (single file) succeeds" '
    ipfs add -w m/5gl-49d-hiv >actual_w_1
  '

  test_expect_success "ipfs add -w (single file) is correct" '
    echo "$add_w_1" >expected &&
    test_sort_cmp expected actual_w_1
  '

  # test two files together
  test_expect_success "ipfs add -w (multiple) succeeds" '
    ipfs add -w m/5gl-49d-hiv m/5sbh >actual_w_12
  '

  test_expect_success "ipfs add -w (multiple) is correct" '
    echo "$add_w_12" >expected  &&
    test_sort_cmp expected actual_w_12
  '

  test_expect_success "ipfs add -w (multiple) succeeds" '
    ipfs add -w m/5sbh m/5gl-49d-hiv >actual_w_12_a
  '

  test_expect_success "ipfs add -w (multiple) orders" '
    echo "$add_w_12" >expected  &&
    test_sort_cmp expected actual_w_12_a
  '

  # test a directory
  test_expect_success "ipfs add -w -r (dir) succeeds" '
    ipfs add -r -w m/mxrw/qtdpbyso0v-4kqvv >actual_w_d1
  '

  test_expect_success "ipfs add -w -r (dir) is correct" '
    echo "$add_w_d1" >expected &&
    test_sort_cmp expected actual_w_d1
  '

  # test files and directory
  test_expect_success "ipfs add -w -r <many> succeeds" '
    ipfs add -w -r m/mxrw/17dk \
      m/qz88-v7g2f9z/-n7bk0ea m/mxrw/zrdvq-njj m/5gl-49d-hiv >actual_w_d2
  '

  test_expect_success "ipfs add -w -r <many> is correct" '
    echo "$add_w_d2" >expected &&
    test_sort_cmp expected actual_w_d2
  '

  # test -w -r m/* == -r m
  test_expect_success "ipfs add -w -r m/* == add -r m  succeeds" '
    ipfs add -Q -w -r m/* >actual_w_m2
  '

  test_expect_success "ipfs add -w -r m/* == add -r m  is correct" '
    echo "$add_w_m" >expected &&
    test_sort_cmp expected actual_w_m2
  '

  # test repeats together
  test_expect_success "ipfs add -w (repeats) succeeds" '
    ipfs add -Q -w -r m/mxrw/17dk m/qz88-v7g2f9z/-n7bk0ea \
      m/mxrw/zrdvq-njj m/5gl-49d-hiv m/mxrw m/mxrw m/5gl-49d-hiv \
      m/5gl-49d-hiv m/qz88-v7g2f9z/0uzy28j2h \
      m/qz88-v7g2f9z/-n7bk0ea/-5d96-jaahmvel9m >actual_w_r
  '

  test_expect_success "ipfs add -w (repeats) is correct" '
    echo "$add_w_r" >expected  &&
    test_sort_cmp expected actual_w_r
  '

  test_expect_success "ipfs add -w -r (dir) --cid-version=1 succeeds" '
    ipfs add -r -w --cid-version=1 m/mxrw/zrdvq-njj >actual_w_d1_v1
  '

  test_expect_success "ipfs add -w -r (dir) --cid-version=1 is correct" '
    echo "$add_w_d1_v1" >expected &&
    test_sort_cmp expected actual_w_d1_v1
  '

  test_expect_success "ipfs add -w -r -n (dir) --cid-version=1 succeeds" '
    ipfs add -r -w -n --cid-version=1 m/mxrw/zrdvq-njj >actual_w_d1_v2
  '

  test_expect_success "ipfs add -w -r -n (dir) --cid-version=1 is correct" '
    echo "$add_w_d1_v1" > expected &&
    test_sort_cmp expected actual_w_d1_v2
  '
}

test_init_ipfs

test_add_w

test_launch_ipfs_daemon

test_add_w

test_kill_ipfs_daemon

test_done
