# Dataset description/sources

- fixtures.car
  - raw CARv1

- k51....ipns-record
  - ipns record, encoded with protocol buffer

generated with:

```sh
# using ipfs version 0.21.0-dev (03a98280e3e642774776cd3d0435ab53e5dfa867)
FILE_CID=$(echo "Hello IPFS" | ipfs add --cid-version 1 -q)
IPNS_KEY=$(ipfs key gen ipns-record)

ipfs dag export ${FILE_CID} > fixtures.car

# publish a record valid for a 100 years
ipfs name publish --key=ipns-record --quieter --ttl=876600h --lifetime=876600h /ipfs/${FILE_CID}
ipfs routing get /ipns/${IPNS_KEY} > ${IPNS_KEY}.ipns-record

echo IPNS_KEY=${IPNS_KEY}
echo FILE_CID=${FILE_CID} # A file containing "Hello IPFS"

# IPNS_KEY=k51qzi5uqu5dh71qgwangrt6r0nd4094i88nsady6qgd1dhjcyfsaqmpp143ab
# FILE_CID=bafkreidfdrlkeq4m4xnxuyx6iae76fdm4wgl5d4xzsb77ixhyqwumhz244 # A file containing Hello IPFS
```
