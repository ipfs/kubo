# Dataset description/sources

- fixtures.car
  - raw CARv1

generated with:

```sh
# using ipfs version 0.18.1
mkdir -p dir &&
echo "hello application/vnd.ipld.raw" > dir/ascii.txt &&
ROOT_DIR_CID=$(ipfs add -Qrw --cid-version 1 dir) &&
FILE_CID=$(ipfs resolve -r /ipfs/$ROOT_DIR_CID/dir/ascii.txt | cut -d "/" -f3) &&
ipfs dag export $ROOT_DIR_CID > fixtures.car

echo ROOT_DIR_CID=${ROOT_DIR_CID} # ./
echo FILE_CID=${FILE_CID} # ./dir/ascii.txt

# ROOT_DIR_CID=bafybeie72edlprgtlwwctzljf6gkn2wnlrddqjbkxo3jomh4n7omwblxly # ./
# FILE_CID=bafkreihhpc5y2pqvl5rbe5uuyhqjouybfs3rvlmisccgzue2kkt5zq6upq # ./dir/ascii.txt
```
