# Dataset Description / Sources

TestGatewayHAMTDirectory.car generated with:

```bash
ipfs version
# ipfs version 0.19.0

export HAMT_DIR=bafybeiggvykl7skb2ndlmacg2k5modvudocffxjesexlod2pfvg5yhwrqm
export IPFS_PATH=$(mktemp -d)

# Init and start daemon, ensure we have an empty repository.
ipfs init --empty-repo
ipfs daemon &> /dev/null &
export IPFS_PID=$!

# Retrieve the directory listing, forcing the daemon to download all required DAGs. Kill daemon.
curl -o dir.html http://127.0.0.1:8080/ipfs/$HAMT_DIR/
kill $IPFS_PID

# Get the list with all the downloaded refs and sanity check.
ipfs refs local > required_refs
cat required_refs | wc -l
# 962

# Get the list of all the files CIDs inside the directory and sanity check.
cat dir.html| pup '#content tbody .ipfs-hash attr{href}' | sed 's/\/ipfs\///g;s/\?filename=.*//g' > files_refs
cat files_refs | wc -l
# 10100

# Make and export our fixture.
ipfs files mkdir --cid-version 1 /fixtures
cat required_refs | xargs -I {} ipfs files cp /ipfs/{} /fixtures/{}
cat files_refs | ipfs files write --create /fixtures/files_refs
export FIXTURE_CID=$(ipfs files stat --hash /fixtures/)
echo $FIXTURE_CID
# bafybeig3yoibxe56aolixqa4zk55gp5sug3qgaztkakpndzk2b2ynobd4i
ipfs dag export $FIXTURE_CID > TestGatewayHAMTDirectory.car
```

TestGatewayMultiRange.car generated with:


```sh
ipfs version
# ipfs version 0.19.0

export FILE_CID=bafybeiae5abzv6j3ucqbzlpnx3pcqbr2otbnpot7d2k5pckmpymin4guau
export IPFS_PATH=$(mktemp -d)

# Init and start daemon, ensure we have an empty repository.
ipfs init --empty-repo
ipfs daemon &> /dev/null &
export IPFS_PID=$!

# Get a specific byte range from the file. 
curl http://127.0.0.1:8080/ipfs/$FILE_CID -i -H "Range: bytes=1276-1279, 29839070-29839080"
kill $IPFS_PID

# Get the list with all the downloaded refs and sanity check.
ipfs refs local > required_refs
cat required_refs | wc -l
# 19

# Make and export our fixture.
ipfs files mkdir --cid-version 1 /fixtures
cat required_refs | xargs -I {} ipfs files cp /ipfs/{} /fixtures/{}
export FIXTURE_CID=$(ipfs files stat --hash /fixtures/)
echo $FIXTURE_CID
# bafybeicgsg3lwyn3yl75lw7sn4zhyj5dxtb7wfxwscpq6yzippetmr2w3y
ipfs dag export $FIXTURE_CID > TestGatewayMultiRange.car
```
