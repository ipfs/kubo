# Dataset description/sources

- inside-root.car

- outside-root.car

- fixtures.car
  - raw CARv1

generated with:

```sh
# ipfs version 0.18.1

mkdir -p rootDir/ipfs &&
mkdir -p rootDir/ipns &&
mkdir -p rootDir/api &&
mkdir -p rootDir/ą/ę &&
echo "I am a txt file on path with utf8" > rootDir/ą/ę/file-źł.txt &&
echo "I am a txt file in confusing /api dir" > rootDir/api/file.txt &&
echo "I am a txt file in confusing /ipfs dir" > rootDir/ipfs/file.txt &&
echo "I am a txt file in confusing /ipns dir" > rootDir/ipns/file.txt &&
DIR_CID=$(ipfs add -Qr --cid-version 1 rootDir) &&
FILE_CID=$(ipfs files stat --enc=json /ipfs/$DIR_CID/ą/ę/file-źł.txt | jq -r .Hash) &&
FILE_SIZE=$(ipfs files stat --enc=json /ipfs/$DIR_CID/ą/ę/file-źł.txt | jq -r .Size)
echo "$FILE_CID / $FILE_SIZE"

echo DIR_CID=${DIR_CID} # ./rootDir
echo FILE_CID=${FILE_CID} # ./rootDir/ą/ę/file-źł.txt
echo FILE_SIZE=${FILE_SIZE}

ipfs dag export ${DIR_CID} > ./fixtures.car

# DIR_CID=bafybeig6ka5mlwkl4subqhaiatalkcleo4jgnr3hqwvpmsqfca27cijp3i # ./rootDir
# FILE_CID=bafkreialihlqnf5uwo4byh4n3cmwlntwqzxxs2fg5vanqdi3d7tb2l5xkm # ./rootDir/ą/ę/file-źł.txt
# FILE_SIZE=34
```
