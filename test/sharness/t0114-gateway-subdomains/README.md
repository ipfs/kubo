# Dataset description/sources

- fixtures.car
  - raw CARv1

generated with:

```sh
# using ipfs version 0.18.1

# CIDv0to1 is necessary because raw-leaves are enabled by default during
# "ipfs add" with CIDv1 and disabled with CIDv0
CID_VAL="hello"
CIDv1=$(echo $CID_VAL | ipfs add --cid-version 1 -Q)
CIDv0=$(echo $CID_VAL | ipfs add --cid-version 0 -Q)
CIDv0to1=$(echo "$CIDv0" | ipfs cid base32)
# sha512 will be over 63char limit, even when represented in Base36
CIDv1_TOO_LONG=$(echo $CID_VAL | ipfs add --cid-version 1 --hash sha2-512 -Q)

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

echo DIR_CID=${DIR_CID}

ipfs files mkdir /t0114/
ipfs files cp /ipfs/${CIDv1} /t0114/
ipfs files cp /ipfs/${CIDv0} /t0114/
ipfs files cp /ipfs/${CIDv0to1} /t0114/
ipfs files cp /ipfs/${DIR_CID} /t0114/
ipfs files cp /ipfs/${CIDv1_TOO_LONG} /t0114/

ROOT=`ipfs files stat /t0114/ --hash`

ipfs dag export ${ROOT} > ./fixtures.car

# CID_VAL="hello"
# CIDv1=bafkreicysg23kiwv34eg2d7qweipxwosdo2py4ldv42nbauguluen5v6am
# CIDv0=QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN
# CIDv0to1=bafybeiffndsajwhk3lwjewwdxqntmjm4b5wxaaanokonsggenkbw6slwk4
# CIDv1_TOO_LONG=bafkrgqhhyivzstcz3hhswshfjgy6ertgmnqeleynhwt4dlfsthi4hn7zgh4uvlsb5xncykzapi3ocd4lzogukir6ksdy6wzrnz6ohnv4aglcs
# DIR_CID=bafybeiht6dtwk3les7vqm6ibpvz6qpohidvlshsfyr7l5mpysdw2vmbbhe # ./testdirlisting
```
