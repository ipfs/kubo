#!/usr/bin/env bash
#
# Copyright (c) 2017 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test keystore commands"

. lib/test-lib.sh

test_init_ipfs

test_key_cmd() {
# test key output format
test_expect_success "create an RSA key and test B58MH/B36CID output formats" '
PEERID=$(ipfs key gen --ipns-base=b58mh --type=rsa --size=2048 key_rsa) &&
test_check_rsa2048_b58mh_peerid $PEERID &&
ipfs key rm key_rsa &&
PEERID=$(ipfs key gen --ipns-base=base36 --type=rsa --size=2048 key_rsa) &&
test_check_rsa2048_base36_peerid $PEERID
'

test_expect_success "test RSA key sk export format" '
ipfs key export key_rsa &&
test_check_rsa2048_sk key_rsa.key &&
rm key_rsa.key
'

test_expect_success "test RSA key B58MH/B36CID multihash format" '
PEERID=$(ipfs key list --ipns-base=b58mh -l | grep key_rsa | head -n 1 | cut -d " " -f1) &&
test_check_rsa2048_b58mh_peerid $PEERID &&
PEERID=$(ipfs key list --ipns-base=base36 -l | grep key_rsa | head -n 1 | cut -d " " -f1) &&
test_check_rsa2048_base36_peerid $PEERID &&
ipfs key rm key_rsa
'

test_expect_success "create an ED25519 key and test B58MH/B36CID output formats" '
PEERID=$(ipfs key gen --ipns-base=b58mh --type=ed25519 key_ed25519) &&
test_check_ed25519_b58mh_peerid $PEERID &&
ipfs key rm key_ed25519 &&
PEERID=$(ipfs key gen --ipns-base=base36 --type=ed25519 key_ed25519) &&
test_check_ed25519_base36_peerid $PEERID
'

test_expect_success "test ED25519 key sk export format" '
ipfs key export key_ed25519 &&
test_check_ed25519_sk key_ed25519.key &&
rm key_ed25519.key
'

test_expect_success "test ED25519 key B58MH/B36CID multihash format" '
PEERID=$(ipfs key list --ipns-base=b58mh -l | grep key_ed25519 | head -n 1 | cut -d " " -f1) &&
test_check_ed25519_b58mh_peerid $PEERID &&
PEERID=$(ipfs key list --ipns-base=base36 -l | grep key_ed25519 | head -n 1 | cut -d " " -f1) &&
test_check_ed25519_base36_peerid $PEERID &&
ipfs key rm key_ed25519
'
# end of format test


  test_expect_success "create a new rsa key" '
    rsahash=$(ipfs key gen generated_rsa_key --type=rsa --size=2048)
    echo $rsahash > rsa_key_id
  '

  test_expect_success "create a new ed25519 key" '
    edhash=$(ipfs key gen generated_ed25519_key --type=ed25519)
    echo $edhash > ed25519_key_id
  '

  test_expect_success "export and import rsa key" '
    ipfs key export generated_rsa_key &&
    ipfs key rm generated_rsa_key &&
    ipfs key import generated_rsa_key generated_rsa_key.key > roundtrip_rsa_key_id &&
    test_cmp rsa_key_id roundtrip_rsa_key_id
  '

  test_expect_success "export and import ed25519 key" '
    ipfs key export generated_ed25519_key &&
    ipfs key rm generated_ed25519_key &&
    ipfs key import generated_ed25519_key generated_ed25519_key.key > roundtrip_ed25519_key_id &&
    test_cmp ed25519_key_id roundtrip_ed25519_key_id
  '

  test_expect_success "test export file option" '
    ipfs key export generated_rsa_key -o=named_rsa_export_file &&
    test_cmp generated_rsa_key.key named_rsa_export_file &&
    ipfs key export generated_ed25519_key -o=named_ed25519_export_file &&
    test_cmp generated_ed25519_key.key named_ed25519_export_file
  '
  
  test_expect_success "key export can't export self" '
    test_must_fail ipfs key export self 2>&1 | tee key_exp_out &&
    grep -q "Error: cannot export key with name" key_exp_out &&
    test_must_fail ipfs key export self -o=selfexport 2>&1 | tee key_exp_out &&
    grep -q "Error: cannot export key with name" key_exp_out
  '

  test_expect_success "key import can't import self" '
    ipfs key gen overwrite_self_import &&
    ipfs key export overwrite_self_import &&
    test_must_fail ipfs key import self overwrite_self_import.key 2>&1 | tee key_imp_out &&
    grep -q "Error: cannot import key with name" key_imp_out &&
    ipfs key rm overwrite_self_import &&
    rm overwrite_self_import.key
  '

  test_expect_success "add a default key" '
    ipfs key gen quxel
  '

  test_expect_success "all keys show up in list output" '
    echo generated_ed25519_key > list_exp &&
    echo generated_rsa_key >> list_exp &&
    echo quxel >> list_exp &&
    echo self >> list_exp
    ipfs key list > list_out &&
    test_sort_cmp list_exp list_out
  '

  test_expect_success "key hashes show up in long list output" '
    ipfs key list -l | grep $edhash > /dev/null &&
    ipfs key list -l | grep $rsahash > /dev/null
  '

  test_expect_success "key list -l contains self key with peerID" '
    PeerID="$(ipfs config Identity.PeerID)"
    ipfs key list -l --ipns-base=b58mh | grep "$PeerID\s\+self"
  '

  test_expect_success "key rm remove a key" '
    ipfs key rm generated_rsa_key
    echo generated_ed25519_key > list_exp &&
    echo quxel >> list_exp &&
    echo self >> list_exp
    ipfs key list > list_out &&
    test_sort_cmp list_exp list_out
  '

  test_expect_success "key rm can't remove self" '
    test_must_fail ipfs key rm self 2>&1 | tee key_rm_out &&
    grep -q "Error: cannot remove key with name" key_rm_out
  '

  test_expect_success "key rename rename a key" '
    ipfs key rename generated_ed25519_key fooed
    echo fooed > list_exp &&
    echo quxel >> list_exp &&
    echo self >> list_exp
    ipfs key list > list_out &&
    test_sort_cmp list_exp list_out
  '

  test_expect_success "key rename rename key output succeeds" '
    key_content=$(ipfs key gen key1 --type=rsa --size=2048) &&
    ipfs key rename key1 key2 >rs &&
    echo "Key $key_content renamed to key2" >expect &&
    test_cmp rs expect
  '

  test_expect_success "key rename can't rename self" '
    test_must_fail ipfs key rename self bar 2>&1 | tee key_rename_out &&
    grep -q "Error: cannot rename key with name" key_rename_out
  '

  test_expect_success "key rename can't overwrite self, even with force" '
    test_must_fail ipfs key rename -f fooed self 2>&1 | tee key_rename_out &&
    grep -q "Error: cannot overwrite key with name" key_rename_out
  '
}

test_check_rsa2048_sk() {
  sklen=$(ls -l $1 | awk '{print $5}') &&
  test "$sklen" -lt "1600" && test "$sklen" -gt "1000" || {
    echo "Bad RSA2048 sk '$1' with len '$sklen'"
    return 1
  }
}

test_check_ed25519_sk() {
  sklen=$(ls -l $1 | awk '{print $5}') &&
  test "$sklen" -lt "100" && test "$sklen" -gt "30" || {
    echo "Bad ED25519 sk '$1' with len '$sklen'"
    return 1
  }
}

test_key_cmd

test_done
