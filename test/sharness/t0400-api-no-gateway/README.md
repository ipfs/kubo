# Dataset description/sources

- fixtures.car
  - raw CARv1

generated with:

```sh
HASH=$(echo "testing" | ipfs add -q)
ipfs dag export $HASH > fixtures.car

echo HASH=${HASH} # a file containing the string "testing"

# HASH=QmNYERzV2LfD2kkfahtfv44ocHzEFK1sLBaE7zdcYT2GAZ # a file containing the string "testing"
```
