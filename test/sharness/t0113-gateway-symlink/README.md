# Dataset description/sources

- testfiles.car
  - raw CARv1

generated with:

```sh
mkdir testfiles &&
echo "content" > testfiles/foo &&
ln -s foo testfiles/bar &&
ROOT_DIR_CID=$(ipfs add -Qr testfiles) &&
ipfs dag export $ROOT_DIR_CID > testfiles.car

# ROOT_DIR_CID=QmWvY6FaqFMS89YAQ9NAPjVP4WZKA1qbHbicc9HeSKQTgt
```
