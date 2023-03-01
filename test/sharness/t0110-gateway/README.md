# Dataset description/sources

- hamt-refs.car
  - raw CARv1

generated with:

```sh
# using ipfs version 0.18.1
export IPFS_PATH=$(mktemp -d)
ipfs init --empty-repo
ipfs daemon &
curl http://127.0.0.1:8080/ipfs/bafybeiaysi4s6lnjev27ln5icwm6tueaw2vdykrtjkwiphwekaywqhcjze/wikipedia_en_all_maxi_2021-02.zim -i -H "Range: bytes=2000-2002, 40000000000-40000000002"
killall ipfs
ipfs refs local >> refs
ipfs files mkdir --cid-version 1 /Test
cat refs | xargs -I {} ipfs files cp /ipfs/{} /Test/{}
ipfs files stat /Test
# Grab CID: bafybeigcvrf7fvk7i3fdhxgk6saqdpw6spujwfxxkq5cshy5kxdjc674ua
ipfs dag export bafybeigcvrf7fvk7i3fdhxgk6saqdpw6spujwfxxkq5cshy5kxdjc674ua > ./refs.car
```
