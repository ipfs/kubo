# Dataset description/sources

- carv1-basic.car
  - raw CARv1
  - Source: https://ipld.io/specs/transport/car/fixture/carv1-basic/carv1-basic.car

- carv1-basic.json
  - description of the contents and layout of the raw CAR, encoded in DAG-JSON
  - Source: https://ipld.io/specs/transport/car/fixture/carv1-basic/carv1-basic.json

- test-dag.car + deterministic.car
  - raw CARv1

generated with:

```sh
# using ipfs version 0.18.1
mkdir -p subdir &&
echo "hello application/vnd.ipld.car" > subdir/ascii.txt &&
ROOT_DIR_CID=$(ipfs add -Qrw --cid-version 1 subdir) &&
FILE_CID=$(ipfs resolve -r /ipfs/$ROOT_DIR_CID/subdir/ascii.txt | cut -d "/" -f3) &&
ipfs dag export $ROOT_DIR_CID > test-dag.car &&
ipfs dag export $FILE_CID > deterministic.car &&

echo ROOT_DIR_CID=${ROOT_DIR_CID} # ./
echo FILE_CID=${FILE_CID} # /\subdir/ascii.txt

# ROOT_DIR_CID=bafybeiefu3d7oytdumk5v7gn6s7whpornueaw7m7u46v2o6omsqcrhhkzi # ./
# FILE_CID=bafkreifkam6ns4aoolg3wedr4uzrs3kvq66p4pecirz6y2vlrngla62mxm # /subdir/ascii.txt
```
