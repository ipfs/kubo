# Dataset description/sources

- dag-cbor-traversal.car

- dag-json-traversal.car

- dag-pb.car

- dag-pb.json

- fixtures.car
  - raw CARv1

generated with:

```sh
# using ipfs version 0.18.1
mkdir -p rootDir/ipfs &&
mkdir -p rootDir/ipns &&
mkdir -p rootDir/api &&
mkdir -p rootDir/ą/ę &&
echo "{ \"test\": \"i am a plain json file\" }" > rootDir/ą/ę/t.json &&
echo "I am a txt file on path with utf8" > rootDir/ą/ę/file-źł.txt &&
echo "I am a txt file in confusing /api dir" > rootDir/api/file.txt &&
echo "I am a txt file in confusing /ipfs dir" > rootDir/ipfs/file.txt &&
echo "I am a txt file in confusing /ipns dir" > rootDir/ipns/file.txt &&
DIR_CID=$(ipfs add -Qr --cid-version 1 rootDir) &&
FILE_JSON_CID=$(ipfs files stat --enc=json /ipfs/$DIR_CID/ą/ę/t.json | jq -r .Hash) &&
FILE_CID=$(ipfs files stat --enc=json /ipfs/$DIR_CID/ą/ę/file-źł.txt | jq -r .Hash) &&
FILE_SIZE=$(ipfs files stat --enc=json /ipfs/$DIR_CID/ą/ę/file-źł.txt | jq -r .Size)
echo "$FILE_CID / $FILE_SIZE"

echo DIR_CID=${DIR_CID} # ./rootDir
echo FILE_JSON_CID=${FILE_JSON_CID} # ./rootDir/ą/ę/t.json
echo FILE_CID=${FILE_CID} # ./rootDir/ą/ę/file-źł.txt
echo FILE_SIZE=${FILE_SIZE}

ipfs dag export ${DIR_CID} > fixtures.car

# DIR_CID=bafybeiafyvqlazbbbtjnn6how5d6h6l6rxbqc4qgpbmteaiskjrffmyy4a # ./rootDir
# FILE_JSON_CID=bafkreibrppizs3g7axs2jdlnjua6vgpmltv7k72l7v7sa6mmht6mne3qqe # ./rootDir/ą/ę/t.json
# FILE_CID=bafkreialihlqnf5uwo4byh4n3cmwlntwqzxxs2fg5vanqdi3d7tb2l5xkm # ./rootDir/ą/ę/file-źł.txt
# FILE_SIZE=34
```
