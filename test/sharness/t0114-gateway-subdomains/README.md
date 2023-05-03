# Dataset description/sources

- fixtures.car
  - raw CARv1

- QmUKd....ipns-record
  - ipns record, encoded with protocol buffer

- 12D3K....ipns-record
  - ipns record, encoded with protocol buffer

Generated with:

```sh
# using ipfs version 0.21.0-dev (03a98280e3e642774776cd3d0435ab53e5dfa867)

# CIDv0to1 is necessary because raw-leaves are enabled by default during
# "ipfs add" with CIDv1 and disabled with CIDv0
CID_VAL="hello"
CIDv1=$(echo $CID_VAL | ipfs add --cid-version 1 -Q)
CIDv0=$(echo $CID_VAL | ipfs add --cid-version 0 -Q)
CIDv0to1=$(echo "$CIDv0" | ipfs cid base32)
# sha512 will be over 63char limit, even when represented in Base36
CIDv1_TOO_LONG=$(echo $CID_VAL | ipfs add --cid-version 1 --hash sha2-512 -Q)

echo CID_VAL=${CID_VAL}
echo CIDv1=${CIDv1}
echo CIDv0=${CIDv0}
echo CIDv0to1=${CIDv0to1}
echo CIDv1_TOO_LONG=${CIDv1_TOO_LONG}

# Directory tree crafted to test for edge cases like "/ipfs/ipfs/ipns/bar"
mkdir -p testdirlisting/ipfs/ipns &&
echo "hello" > testdirlisting/hello &&
echo "text-file-content" > testdirlisting/ipfs/ipns/bar &&
mkdir -p testdirlisting/api &&
mkdir -p testdirlisting/ipfs &&
echo "I am a txt file" > testdirlisting/api/file.txt &&
echo "I am a txt file" > testdirlisting/ipfs/file.txt &&
DIR_CID=$(ipfs add -Qr --cid-version 1 testdirlisting)

echo DIR_CID=${DIR_CID} # ./testdirlisting

ipfs files mkdir /t0114/
ipfs files cp /ipfs/${CIDv1} /t0114/
ipfs files cp /ipfs/${CIDv0} /t0114/
ipfs files cp /ipfs/${CIDv0to1} /t0114/
ipfs files cp /ipfs/${DIR_CID} /t0114/
ipfs files cp /ipfs/${CIDv1_TOO_LONG} /t0114/

ROOT=`ipfs files stat /t0114/ --hash`

ipfs dag export ${ROOT} > ./fixtures.car

# Then the keys

KEY_NAME=test_key_rsa_$RANDOM
RSA_KEY=$(ipfs key gen --ipns-base=b58mh --type=rsa --size=2048 ${KEY_NAME} | head -n1 | tr -d "\n")
RSA_IPNS_IDv0=$(echo "$RSA_KEY" | ipfs cid format -v 0)
RSA_IPNS_IDv1=$(echo "$RSA_KEY" | ipfs cid format -v 1 --mc libp2p-key -b base36)
RSA_IPNS_IDv1_DAGPB=$(echo "$RSA_IPNS_IDv0" | ipfs cid format -v 1 -b base36)

# publish a record valid for a 100 years
ipfs name publish --key ${KEY_NAME} --allow-offline -Q --ttl=876600h --lifetime=876600h "/ipfs/$CIDv1"
ipfs routing get /ipns/${RSA_KEY} > ${RSA_KEY}.ipns-record

echo RSA_KEY=${RSA_KEY}
echo RSA_IPNS_IDv0=${RSA_IPNS_IDv0}
echo RSA_IPNS_IDv1=${RSA_IPNS_IDv1}
echo RSA_IPNS_IDv1_DAGPB=${RSA_IPNS_IDv1_DAGPB}

KEY_NAME=test_key_ed25519_$RANDOM
ED25519_KEY=$(ipfs key gen --ipns-base=b58mh --type=ed25519 ${KEY_NAME} | head -n1 | tr -d "\n")
ED25519_IPNS_IDv0=$ED25519_KEY
ED25519_IPNS_IDv1=$(ipfs key list -l --ipns-base=base36 | grep ${KEY_NAME} | cut -d " " -f1 | tr -d "\n")
ED25519_IPNS_IDv1_DAGPB=$(echo "$ED25519_IPNS_IDv1" | ipfs cid format -v 1 -b base36 --mc dag-pb)

# ed25519 fits under 63 char limit when represented in base36
IPNS_ED25519_B58MH=$(ipfs key list -l --ipns-base b58mh | grep $KEY_NAME | cut -d" " -f1 | tr -d "\n")
IPNS_ED25519_B36CID=$(ipfs key list -l --ipns-base base36 | grep $KEY_NAME | cut -d" " -f1 | tr -d "\n")

# publish a record valid for a 100 years
ipfs name publish --key ${KEY_NAME} --allow-offline -Q --ttl=876600h --lifetime=876600h "/ipfs/$CIDv1"
ipfs routing get /ipns/${ED25519_KEY} > ${ED25519_KEY}.ipns-record

echo ED25519_KEY=${ED25519_KEY}
echo ED25519_IPNS_IDv0=${ED25519_IPNS_IDv0}
echo ED25519_IPNS_IDv1=${ED25519_IPNS_IDv1}
echo ED25519_IPNS_IDv1_DAGPB=${ED25519_IPNS_IDv1_DAGPB}
echo IPNS_ED25519_B58MH=${IPNS_ED25519_B58MH}
echo IPNS_ED25519_B36CID=${IPNS_ED25519_B36CID}

# CID_VAL=hello
# CIDv1=bafkreicysg23kiwv34eg2d7qweipxwosdo2py4ldv42nbauguluen5v6am
# CIDv0=QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN
# CIDv0to1=bafybeiffndsajwhk3lwjewwdxqntmjm4b5wxaaanokonsggenkbw6slwk4
# CIDv1_TOO_LONG=bafkrgqhhyivzstcz3hhswshfjgy6ertgmnqeleynhwt4dlfsthi4hn7zgh4uvlsb5xncykzapi3ocd4lzogukir6ksdy6wzrnz6ohnv4aglcs
# DIR_CID=bafybeiht6dtwk3les7vqm6ibpvz6qpohidvlshsfyr7l5mpysdw2vmbbhe # ./testdirlisting

# RSA_KEY=QmVujd5Vb7moysJj8itnGufN7MEtPRCNHkKpNuA4onsRa3
# RSA_IPNS_IDv0=QmVujd5Vb7moysJj8itnGufN7MEtPRCNHkKpNuA4onsRa3
# RSA_IPNS_IDv1=k2k4r8m7xvggw5pxxk3abrkwyer625hg01hfyggrai7lk1m63fuihi7w
# RSA_IPNS_IDv1_DAGPB=k2jmtxu61bnhrtj301lw7zizknztocdbeqhxgv76l2q9t36fn9jbzipo

# ED25519_KEY=12D3KooWLQzUv2FHWGVPXTXSZpdHs7oHbXub2G5WC8Tx4NQhyd2d
# ED25519_IPNS_IDv0=12D3KooWLQzUv2FHWGVPXTXSZpdHs7oHbXub2G5WC8Tx4NQhyd2d
# ED25519_IPNS_IDv1=k51qzi5uqu5dk3v4rmjber23h16xnr23bsggmqqil9z2gduiis5se8dht36dam
# ED25519_IPNS_IDv1_DAGPB=k50rm9yjlt0jey4fqg6wafvqprktgbkpgkqdg27tpqje6iimzxewnhvtin9hhq
# IPNS_ED25519_B58MH=12D3KooWLQzUv2FHWGVPXTXSZpdHs7oHbXub2G5WC8Tx4NQhyd2d
# IPNS_ED25519_B36CID=k51qzi5uqu5dk3v4rmjber23h16xnr23bsggmqqil9z2gduiis5se8dht36dam
```
