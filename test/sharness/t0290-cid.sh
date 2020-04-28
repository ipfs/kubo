#!/usr/bin/env bash

test_description="Test cid commands"

. lib/test-lib.sh

# note: all "ipfs cid" commands should work without requiring a repo

CIDv0="QmS4ustL54uo8FzR9455qaxZwuMiUhyvMcX9Ba8nUH4uVv"
CIDv1="zdj7WZAAFKPvYPPzyJLso2hhxo8a7ZACFQ4DvvfrNXTHidofr"
CIDb32="bafybeibxm2nsadl3fnxv2sxcxmxaco2jl53wpeorjdzidjwf5aqdg7wa6u"

CIDbase="QmYNmQKp6SuaVrpgWRsPTgCQCnpxUYGq76YEKBXuj2N4H6"
CIDb32pb="bafybeievd6mwe6vcwnkwo3eizs3h7w3a34opszbyfxziqdxguhjw7imdve"
CIDb32raw="bafkreievd6mwe6vcwnkwo3eizs3h7w3a34opszbyfxziqdxguhjw7imdve"

test_expect_success "cid base32 works" '
  echo $CIDb32 > expected &&
  ipfs cid base32 $CIDv0 > actual1 &&
  test_cmp actual1 expected &&
  ipfs cid base32 $CIDv1 > actual2 &&
  test_cmp expected actual2
'

test_expect_success "cid format -v 1 -b base58btc" '
  echo $CIDv1 > expected &&
  ipfs cid format -v 1 -b base58btc $CIDv0 > actual1 &&
  test_cmp actual1 expected &&
  ipfs cid format -v 1 -b base58btc $CIDb32 > actual2 &&
  test_cmp expected actual2
'

test_expect_success "cid format -v 0" '
  echo $CIDv0 > expected &&
  ipfs cid format -v 0 $CIDb32 > actual &&
  test_cmp expected actual
'

cat <<EOF > various_cids
QmZZRTyhDpL5Jgift1cHbAhexeE1m2Hw8x8g7rTcPahDvo
 QmPhk6cJkRcFfZCdYam4c9MKYjFG9V29LswUnbrFNhtk2S
bafybeihtwdtifv43rn5cyilnmkwofdcxi2suqimmo62vn3etf45gjoiuwy
bafybeiek4tfxkc4ov6jsmb63fzbirrsalnjw24zd5xawo2fgxisd4jmpyq
zdj7WgYfT2gfsgiUxzPYboaRbP9H9CxZE5jVMK9pDDwCcKDCR
zdj7WbTaiJT1fgatdet9Ei9iDB5hdCxkbVyhyh8YTUnXMiwYi
uAXASIDsp4T3Wnd6kXFOQaljH3GFK_ixkjMtVhB9VOBrPK3bp
 uAXASIDdmmyANeytvXUriuy4BO0lfd2eR0UjygabF6CAzfsD1
EOF

cat <<EOF > various_cids_base32
bafybeifgwyq5gs4l2mru5klgwjfmftjvkmbyyjurbupuz2bst7mhmg2hwa
bafybeiauil46g3lb32jemjbl7yspca3twdcg4wwkbsgdgvgdj5fpfv2f64
bafybeihtwdtifv43rn5cyilnmkwofdcxi2suqimmo62vn3etf45gjoiuwy
bafybeiek4tfxkc4ov6jsmb63fzbirrsalnjw24zd5xawo2fgxisd4jmpyq
bafybeifffq3aeaymxejo37sn5fyaf7nn7hkfmzwdxyjculx3lw4tyhk7uy
bafybeiczsscdsbs7ffqz55asqdf3smv6klcw3gofszvwlyarci47bgf354
bafybeib3fhqt3vu532sfyu4qnjmmpxdbjl7cyzemznkyih2vhanm6k3w5e
bafybeibxm2nsadl3fnxv2sxcxmxaco2jl53wpeorjdzidjwf5aqdg7wa6u
EOF

cat <<EOF > various_cids_v1
zdj7WgefqQm5HogBQ2bckZuTYYDarRTUZi51GYCnerHD2G86j
zdj7WWnzU3Nbu5rYGWZHKigUXBtAwShs2SHDCM1TQEvC9TeCN
zdj7WmqAbpsfXgiRBtZP1oAP9QWuuY3mqbc5JhpxJkfT3vYCu
zdj7Wen5gtfr7AivXip3zYd1peuq2QfKrqAn4FGiciVWb96YB
zdj7WgYfT2gfsgiUxzPYboaRbP9H9CxZE5jVMK9pDDwCcKDCR
zdj7WbTaiJT1fgatdet9Ei9iDB5hdCxkbVyhyh8YTUnXMiwYi
zdj7WZQrAvnY5ge3FNg5cmCsNwsvpYjdtu2yEmnWYQ4ES7Nzk
zdj7WZAAFKPvYPPzyJLso2hhxo8a7ZACFQ4DvvfrNXTHidofr
EOF

test_expect_success "cid base32 works from stdin" '
  cat various_cids | ipfs cid base32 > actual &&
  test_cmp various_cids_base32 actual
'

test_expect_success "cid format -v 1 -b base58btc works from stdin" '
  cat various_cids | ipfs cid format -v 1 -b base58btc > actual &&
  test_cmp various_cids_v1 actual
'

cat <<EOF > bases_expect
      0  identity
0    48  base2
b    98  base32
B    66  base32upper
c    99  base32pad
C    67  base32padupper
f   102  base16
F    70  base16upper
m   109  base64
M    77  base64pad
t   116  base32hexpad
T    84  base32hexpadupper
u   117  base64url
U    85  base64urlpad
v   118  base32hex
V    86  base32hexupper
z   122  base58btc
Z    90  base58flickr
EOF

cat <<EOF > codecs_expect
   85  raw
  112  protobuf
  113  cbor
  120  git-raw
  144  eth-block
  145  eth-block-list
  146  eth-tx-trie
  147  eth-tx
  148  eth-tx-receipt-trie
  149  eth-tx-receipt
  150  eth-state-trie
  151  eth-account-snapshot
  152  eth-storage-trie
  176  bitcoin-block
  177  bitcoin-tx
  192  zcash-block
  193  zcash-tx
  224  decred-block
  225  decred-tx
  240  dash-block
  241  dash-tx
EOF

cat <<EOF > hashes_expect
    0  identity
   17  sha1
   18  sha2-256
   19  sha2-512
   20  sha3-512
   21  sha3-384
   22  sha3-256
   23  sha3-224
   25  shake-256
   26  keccak-224
   27  keccak-256
   28  keccak-384
   29  keccak-512
   86  dbl-sha2-256
45588  blake2b-160
45589  blake2b-168
45590  blake2b-176
45591  blake2b-184
45592  blake2b-192
45593  blake2b-200
45594  blake2b-208
45595  blake2b-216
45596  blake2b-224
45597  blake2b-232
45598  blake2b-240
45599  blake2b-248
45600  blake2b-256
45601  blake2b-264
45602  blake2b-272
45603  blake2b-280
45604  blake2b-288
45605  blake2b-296
45606  blake2b-304
45607  blake2b-312
45608  blake2b-320
45609  blake2b-328
45610  blake2b-336
45611  blake2b-344
45612  blake2b-352
45613  blake2b-360
45614  blake2b-368
45615  blake2b-376
45616  blake2b-384
45617  blake2b-392
45618  blake2b-400
45619  blake2b-408
45620  blake2b-416
45621  blake2b-424
45622  blake2b-432
45623  blake2b-440
45624  blake2b-448
45625  blake2b-456
45626  blake2b-464
45627  blake2b-472
45628  blake2b-480
45629  blake2b-488
45630  blake2b-496
45631  blake2b-504
45632  blake2b-512
45652  blake2s-160
45653  blake2s-168
45654  blake2s-176
45655  blake2s-184
45656  blake2s-192
45657  blake2s-200
45658  blake2s-208
45659  blake2s-216
45660  blake2s-224
45661  blake2s-232
45662  blake2s-240
45663  blake2s-248
45664  blake2s-256
EOF

test_expect_success "cid bases" '
  cut -c 10- bases_expect > expect &&
  ipfs cid bases > actual &&
  test_cmp expect actual
'

test_expect_success "cid bases --prefix" '
  cut -c 1-3,10- bases_expect > expect &&
  ipfs cid bases --prefix > actual &&
  test_cmp expect actual
'

test_expect_success "cid bases --prefix --numeric" '
  ipfs cid bases --prefix --numeric > actual &&
  test_cmp bases_expect actual
'

test_expect_success "cid codecs" '
  cut -c 8- codecs_expect > expect &&
  ipfs cid codecs > actual
  test_cmp expect actual
'

test_expect_success "cid codecs --numeric" '
  ipfs cid codecs --numeric > actual &&
  test_cmp codecs_expect actual
'

test_expect_success "cid hashes" '
  cut -c 8- hashes_expect > expect &&
  ipfs cid hashes > actual
  test_cmp expect actual
'

test_expect_success "cid hashes --numeric" '
  ipfs cid hashes --numeric > actual &&
  test_cmp hashes_expect actual
'

test_expect_success "cid format -c raw" '
  echo $CIDb32raw > expected &&
  ipfs cid format --codec raw -b base32 $CIDb32pb > actual &&
  test_cmp actual expected
'

test_expect_success "cid format -c protobuf -v 0" '
  echo $CIDbase > expected &&
  ipfs cid format --codec protobuf -v 0 $CIDb32raw > actual &&
  test_cmp actual expected
'


test_done
