#!/usr/bin/env bash
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test add -w"

add_w_m='QmVFBKBbdv7HoMdjTT7adMyUsTvY4tVyr6aPTgrCWyFQo6'

add_w_1='added Qmb2LTzn7Yo2LHgmLUaHbaWuNGfVKS25sMYqK52okGa4yd 8e6j_
added QmcwexGV5CFFN1SqQNXqBhzbC6AxtXBdFm7Dv61VtLBTHU '

add_w_12='added Qmb2LTzn7Yo2LHgmLUaHbaWuNGfVKS25sMYqK52okGa4yd 8e6j_
added QmcytURLVEAaNDiNzWU5pZhL8MCTNmmiXrkF2ekPF4ZkNw ewxbo-p_ebq1k6z
added Qme6uNAJHyjjdJSHwejw4GK3fEtzrPA9rTNbBWVW8cJNqV '

add_w_d1='added QmReS7VZaSdf3ik6qJqcKka79hZoaZxt68RzxpeRM6vhcY efis44rh5jq69/4e9ygub65pqnd
added Qmegpr1gVLrkVeWmXCYmZVkJ3cNHcjKEi1RBRqE68UY9A7 efis44rh5jq69/eltgzx
added QmQ2szbNCuejR793HvBZ6pwpAPSTmomuCqvoUzoasqSAjw efis44rh5jq69/kv90ww6n0j
added QmaKAczxkiJFVpATJiZGnJ3eoS5wY3HhiLCFftCfgo47QM efis44rh5jq69/vrlkd8mxt8cty7
added QmXRAAykFSGPY3DGJJHV92FvKr4UcanfhZMdTGHiPAZEad efis44rh5jq69/wn2878
added QmUrRLM2ZVy9b553e1EDgaCKyeNq3sTFb94U7dk6F2Zzcp efis44rh5jq69
added QmdNfMuS4Bui9esinhVCEPBvKPeCxe2694sF6AwiCLC9ix '

add_w_d1_v1='added bafkreic4faiiaj2arxtiifvaskyj23cvfk26mr7gx7cy7h3pbyt7c5o7yy ks-7mqkzt29b2q/b-o3l9oc
added bafkreif37syqi44bdbsfi4ewhijlmxwhr4v3mrjjdururm5igvarzdd36u ks-7mqkzt29b2q/gdntkjcg26dvu27d
added bafkreie43jg3hzmkfdwippgyd4xza3rucauu4juw6dj4smbspkdsosbhte ks-7mqkzt29b2q/lc9jj
added bafkreiekjb6ae4dr6bfa6msnnulnla6zwikfhalgduuert5uiauua36cru ks-7mqkzt29b2q/u3mk_g-o61c4x
added bafkreifsgblf5nddvqpioje77fzu3arf4d4t4nt2h5e3lgmxx5izfzcjxy ks-7mqkzt29b2q/w44jbb0oc6
added bafybeidoqcknpakdk66xyii3vkiv7jkipu234xnw5jj6f6hjofictaasni ks-7mqkzt29b2q
added bafybeic2j5ttfehhhcti236asobqipzcb7tmuejkrcrw7ia22ykfenj5ly '

add_w_d2='added QmPHk2wm2iWm6qnVq3GNob4Qg6kwJJmv4fZrje4mouEeTh -ydbcn6
added Qmb2LTzn7Yo2LHgmLUaHbaWuNGfVKS25sMYqK52okGa4yd 8e6j_
added QmU4tJkTCtGzkmiYmrNBbEmAW7MXQEjsois3uwVfmyELPC ks-7mqkzt29b2q/b-o3l9oc
added QmW5446N9DuGrAvfxhc74cG6MPk1uoXPwZKAhXPttBZwvo ks-7mqkzt29b2q/gdntkjcg26dvu27d
added QmVLtgkve9nbJRxSv4FNL1a4X1gvmbLU7y6Xgaa9zc6GCZ ks-7mqkzt29b2q/lc9jj
added QmX4xbmXU77EkvBTV5v4FpXtV6naNkYuhV3foGasLG1FoE ks-7mqkzt29b2q/u3mk_g-o61c4x
added QmQTHSB5azu1ni3aSaZk9XCNW5suJv4QfmFJV8oM8rnSNM ks-7mqkzt29b2q/w44jbb0oc6
added QmfSUVemMEcSaZ7r7mFvyuGaPGiRckMhrZCYhrwsyza8o8 ndmuvqny42hqhc/-q5qip
added QmXbw2ApWBFHEtibxCKDNR4vGtsPS9SHnMUNSrb3pmcfoV ndmuvqny42hqhc/f-v9
added QmZoYh5nT8Yy8kx68c89tYetwhpF59psakvfNfkwS6jAqM ndmuvqny42hqhc/izsy8d_n0uke3s-k
added QmX2myYx9TSXmaFR8NoP7Zc2BQDUYyCYp8tPHnA8v14pux ndmuvqny42hqhc/nmg0an87ivw
added QmbpDTAwTGbhsxPD7Y3YSJUdppMRwEVtwpgaYDsJaXcTAq ndmuvqny42hqhc/y508xc5ex
added Qme3zK228nJztwRqSZShCvoxnixK7JLUSTdHJGth2aaDBX ks-7mqkzt29b2q
added QmPRfbjzfU2qW4XfFYqWMHC8YaRPjcPQ3y8qmsRgdrB1f7 ndmuvqny42hqhc
added QmexKKqpqkjcUS1xsaDv2pwPKWg8nSVeADuKk8bFd3nrZG '

add_w_r='QmW7rkP394QujXBQJAhgDpZ8eEYWHw9bTcpYmNU7Qs1Ugp'

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
    ipfs add -w m/8e6j_ >actual_w_1
  '

  test_expect_success "ipfs add -w (single file) is correct" '
    echo "$add_w_1" >expected &&
    test_sort_cmp expected actual_w_1
  '

  # test two files together
  test_expect_success "ipfs add -w (multiple) succeeds" '
    ipfs add -w m/8e6j_ m/ewxbo-p_ebq1k6z >actual_w_12
  '

  test_expect_success "ipfs add -w (multiple) is correct" '
    echo "$add_w_12" >expected  &&
    test_sort_cmp expected actual_w_12
  '

  test_expect_success "ipfs add -w (multiple) succeeds" '
    ipfs add -w m/ewxbo-p_ebq1k6z m/8e6j_ >actual_w_12_a
  '

  test_expect_success "ipfs add -w (multiple) orders" '
    echo "$add_w_12" >expected  &&
    test_sort_cmp expected actual_w_12_a
  '

  # test a directory
  test_expect_success "ipfs add -w -r (dir) succeeds" '
    ipfs add -r -w m/j2x5evmf3iu_99-n/efis44rh5jq69 >actual_w_d1
  '

  test_expect_success "ipfs add -w -r (dir) is correct" '
    echo "$add_w_d1" >expected &&
    test_sort_cmp expected actual_w_d1
  '

  # test files and directory
  test_expect_success "ipfs add -w -r <many> succeeds" '
    ipfs add -w -r m/j2x5evmf3iu_99-n/-ydbcn6 \
      m/phtgkon/ndmuvqny42hqhc m/j2x5evmf3iu_99-n/ks-7mqkzt29b2q m/8e6j_ >actual_w_d2
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
    ipfs add -Q -w -r m/j2x5evmf3iu_99-n/-ydbcn6 m/phtgkon/ndmuvqny42hqhc \
      m/j2x5evmf3iu_99-n/ks-7mqkzt29b2q m/8e6j_ m/j2x5evmf3iu_99-n m/j2x5evmf3iu_99-n m/8e6j_ \
      m/8e6j_ m/phtgkon/1g-nsct \
      m/phtgkon/ndmuvqny42hqhc/-q5qip >actual_w_r
  '

  test_expect_success "ipfs add -w (repeats) is correct" '
    echo "$add_w_r" >expected  &&
    test_sort_cmp expected actual_w_r
  '

  test_expect_success "ipfs add -w -r (dir) --cid-version=1 succeeds" '
    ipfs add -r -w --cid-version=1 m/j2x5evmf3iu_99-n/ks-7mqkzt29b2q >actual_w_d1_v1
  '

  test_expect_success "ipfs add -w -r (dir) --cid-version=1 is correct" '
    echo "$add_w_d1_v1" >expected &&
    test_sort_cmp expected actual_w_d1_v1
  '

  test_expect_success "ipfs add -w -r -n (dir) --cid-version=1 succeeds" '
    ipfs add -r -w -n --cid-version=1 m/j2x5evmf3iu_99-n/ks-7mqkzt29b2q >actual_w_d1_v2
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
