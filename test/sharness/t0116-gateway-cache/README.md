# Dataset description/sources

- fixtures.car
  - raw CARv1

generated with:

```sh
# using ipfs version 0.21.0-dev (03a98280e3e642774776cd3d0435ab53e5dfa867)

mkdir -p root2/root3/root4 &&
echo "hello" > root2/root3/root4/index.html &&
ROOT1_CID=$(ipfs add -Qrw --cid-version 1 root2)
ROOT2_CID=$(ipfs resolve -r /ipfs/$ROOT1_CID/root2 | cut -d "/" -f3)
ROOT3_CID=$(ipfs resolve -r /ipfs/$ROOT1_CID/root2/root3 | cut -d "/" -f3)
ROOT4_CID=$(ipfs resolve -r /ipfs/$ROOT1_CID/root2/root3/root4 | cut -d "/" -f3)
FILE_CID=$(ipfs resolve -r /ipfs/$ROOT1_CID/root2/root3/root4/index.html | cut -d "/" -f3)

TEST_IPNS_ID=$(ipfs key gen --ipns-base=base36 --type=ed25519 cache_test_key | head -n1 | tr -d "\n")
# publish a record valid for a 100 years
ipfs name publish --key cache_test_key --allow-offline -Q --ttl=876600h --lifetime=876600h "/ipfs/$ROOT1_CID"
ipfs routing get /ipns/${TEST_IPNS_ID} > ${TEST_IPNS_ID}.ipns-record

echo ROOT1_CID=${ROOT1_CID} # ./
echo ROOT2_CID=${ROOT2_CID} # ./root2
echo ROOT3_CID=${ROOT3_CID} # ./root2/root3
echo ROOT4_CID=${ROOT4_CID} # ./root2/root3/root4
echo FILE_CID=${FILE_CID} # ./root2/root3/root4/index.html
echo TEST_IPNS_ID=${TEST_IPNS_ID}

ipfs dag export ${ROOT1_CID} > ./fixtures.car

# ROOT1_CID=bafybeib3ffl2teiqdncv3mkz4r23b5ctrwkzrrhctdbne6iboayxuxk5ui # ./
# ROOT2_CID=bafybeih2w7hjocxjg6g2ku25hvmd53zj7og4txpby3vsusfefw5rrg5sii # ./root2
# ROOT3_CID=bafybeiawdvhmjcz65x5egzx4iukxc72hg4woks6v6fvgyupiyt3oczk5ja # ./root2/root3
# ROOT4_CID=bafybeifq2rzpqnqrsdupncmkmhs3ckxxjhuvdcbvydkgvch3ms24k5lo7q # ./root2/root3/root4
# FILE_CID=bafkreicysg23kiwv34eg2d7qweipxwosdo2py4ldv42nbauguluen5v6am # ./root2/root3/root4/index.html
# TEST_IPNS_ID=k51qzi5uqu5dlxdsdu5fpuu7h69wu4ohp32iwm9pdt9nq3y5rpn3ln9j12zfhe
```
