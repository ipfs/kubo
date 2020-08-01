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
test_expect_success "create an RSA key and test B58MH multihash output" '
PEERID=$(ipfs key gen -f=b58mh --type=rsa --size=2048 key_rsa) &&
test_check_rsa2048_b58mh_peerid $PEERID
'

test_expect_success "test RSA key sk export format" '
SK=$(ipfs key export key_rsa) &&
test_check_rsa2048_sk $SK
'

test_expect_success "test RSA key B36CID multihash format" '
PEERID=$(ipfs key list -f=b36cid -l | grep key_rsa | head -n 1 | cut -d " " -f1) &&
test_check_rsa2048_b36cid_peerid $PEERID &&
ipfs key rm key_rsa
'

test_expect_success "create an ED25519 key and test multihash output" '
PEERID=$(ipfs key gen -f=b36cid --type=ed25519 key_ed25519) &&
test_check_ed25519_b36cid_peerid $PEERID
'

test_expect_success "test ED25519 key sk export format" '
SK=$(ipfs key export key_ed25519) &&
test_check_ed25519_sk $SK
'

test_expect_success "test ED25519 key B36CID multihash format" '
PEERID=$(ipfs key list -f=b36cid -l | grep key_ed25519 | head -n 1 | cut -d " " -f1) &&
test_check_ed25519_b36cid_peerid $PEERID &&
ipfs key rm key_ed25519
'
# end of format test


  test_expect_success "create a new rsa key" '
    rsahash=$(ipfs key gen -f=b58mh foobarsa --type=rsa --size=2048)
  '

  test_expect_success "create a new ed25519 key" '
    edhash=$(ipfs key gen -f=b58mh bazed --type=ed25519)
  '
  
  test_expect_success "import an rsa key" '
    echo "B9bLmHeKLQU1hX23meSn2kJiNW7AZ31C6PBNSYumejXB13vxSVvViZDkEnchAH4BTs9yVnBNZKZYLwykaCohzxTntesTCVaBhZR2Br2Swav2NXwVBhfrUbaBTrR2248KfbVxSiUdkpFn8kcAmUGwp2KGMGRmq85WreGFDdAvzz8ruN2EFfWSHLc1YeUxHeUgKsQm3N13uF7q5x4qvjWM6yvMWNY7JtZrihT8BZQhgb2ezR4iYPjXZTtPZWsLbrrhUvHhxSn1NsTw6NZ7Jbs84qMXXnH56BmT8J9LugRhQQvgBquSoS1m7aeD2y1L1A7mueVDYGLDzgxRSt5CohY7VVMdheUpPiq44CicuYt5YgbbuE3wMHntn6sd9QSYw7f4SjjKdw7Jhy5fNW29SkHvpLfZKfqzBNhSfHsofXVEASDpfr5mqws1eQTztqvvZhHXQxAoxxP3PK6TfDnATdLdV3Cy5v6nLt7ppsBj54hif5EZneHMLeYP8bYLbELQ2fZdoprpnKVBsMY1nWvgrMKUTpLjoKB6bzAZYuXaYVtmrdhESmKCyE12yEyvCcD8DJQU6JjqaD1DyVSPNkL3ze26Mm3ZyiFEH7M4XirUsPLrsj41Qt1xGaVGNEdkdthihytTZxxnmgyAptZMUNfSviBfH1tVbfoXFtBGU8eYMLdSHFqxSktT1mqeiWatxMQZ8pTeA9VCvAp9RTSRFkTQR9uP6w6qTzRD9cFcH4HyCEc5TJpiZdP7u9RjaEo2S3P9VkHfmqCH4McpLgw7He9nm7rf9JA2Gh7ubTKy5e6dJUWojgYhGS4KGe3yKGFhLaNgRiME63fUEFSnN2ZvCSM9qsrj34q2h8962xBod9hCVEDfk4tfmHu1UHX5AGaW6mk3pzqKKVYTTWXi84JSH7vzKPmQuhwaAR9Ye3Jbdzehp4xhrT5aFCjnz3r5qNv2zz48Fq5bGc1RUh88PUMT3z6kuzv6B1eXTLYpeu9gGdjc5C9DQDTYPfcHWn7dSHr4AGV1sN6SwVy8W5LZdMAZaeXCDn9iXDwbeD2DYd2ozVCEzceygVzpVdnueNx5FmG6zHtGzfuStr4Jj85sbd2jUGh4ES2bMU41jw2gJ6ujjf6CrxZpCWhXz6NJpAS9njcDFXuspf7otbMjCB6TzwokJwEse31nGUZQdhQgXn23vnZtxwCV621uXFbm7xVACRZKeuXgw8VdEVaXGvf2V4DdhjZnjmePBbTeJ7WjABavLcpMqZJH7FgaLxazFqk9RXtnfUEbVAAhuZzxz6L8Z6axHwz3a4EZtALRjfFjn6xjaUtsWXYW8P6F7femM6UHx3qXMo43hKC7oxnd6Tfta972dgyQfSoBwWkWzB8cvaJreNh4bdLNkw6mty86NXGKyijv83LR1HjbnUoTwPbEMX8JyzLfMf3qiWzwf6MHrXprwygmEpNc6w8tNNivmcWyCX3wmPkMKK1bmi5TCHoUtRrxcyXKhmuyo6zzag8KyK6iZaRbMiFiUJBi5VYwidMutkexWo8SRqfSV5yp2kxswknmpeVTnXBhVEy3anMEiD6bV48AnbF6SfKAi2DGBFqxBFfpFEbYtPauHiYYzZX1epqvKxY23xA9J8FosMk4yYN4Ps7Rh" >> importkey
    imphash=$(ipfs key import -f=b58mh quxel $(cat importkey))
  '

  test_expect_success "exported key matches imported" '
    ipfs key export quxel >> exportkey &&
    test_cmp importkey exportkey
  '
  
  test_expect_success "key export can't export self" '
    test_must_fail ipfs key export self 2>&1 | tee key_exp_out &&
    grep -q "Error: cannot export key with name" key_exp_out
  '

  test_expect_success "key import can't import self" '
    test_must_fail ipfs key import self $(cat importkey) 2>&1 | tee key_imp_out &&
    grep -q "Error: cannot import key with name" key_imp_out
  '

  test_expect_success "all keys show up in list output" '
    echo bazed > list_exp &&
    echo foobarsa >> list_exp &&
    echo quxel >> list_exp &&
    echo self >> list_exp
    ipfs key list -f=b58mh > list_out &&
    test_sort_cmp list_exp list_out
  '

  test_expect_success "key hashes show up in long list output" '
    ipfs key list -f=b58mh -l | grep $edhash > /dev/null &&
    ipfs key list -f=b58mh -l | grep $rsahash > /dev/null
  '

  test_expect_success "key list -l contains self key with peerID" '
    PeerID="$(ipfs config Identity.PeerID)"
    ipfs key list -f=b58mh -l | grep "$PeerID\s\+self"
  '

  test_expect_success "key rm remove a key" '
    ipfs key rm foobarsa
    echo bazed > list_exp &&
    echo quxel >> list_exp &&
    echo self >> list_exp
    ipfs key list -f=b58mh > list_out &&
    test_sort_cmp list_exp list_out
  '

  test_expect_success "key rm can't remove self" '
    test_must_fail ipfs key rm self 2>&1 | tee key_rm_out &&
    grep -q "Error: cannot remove key with name" key_rm_out
  '

  test_expect_success "key rename rename a key" '
    ipfs key rename bazed fooed
    echo fooed > list_exp &&
    echo quxel >> list_exp &&
    echo self >> list_exp
    ipfs key list -f=b58mh > list_out &&
    test_sort_cmp list_exp list_out
  '

  test_expect_success "key rename rename key output succeeds" '
    key_content=$(ipfs key gen -f=b58mh key1 --type=rsa --size=2048) &&
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

test_key_cmd

test_done
