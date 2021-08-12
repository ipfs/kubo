## How to use index option

### 1.Some configuration before running ipfs node

If you haven't config file for your ipfs node, you can run this command:
```shell
ipfs init
```
It will generate the default config in the default path: **~/.ipfs/config**

- specify the Index switch in your custom config file, like that:

```
...
"Experimental": {
"FilestoreEnabled": false,
"UrlstoreEnabled": false,
"Index": true,
}
...
```

- specify the Environment variable for **INDEX_NODE_URL**, like that:
```shell
export INDEX_NODE_URL=http://127.0.0.1:3002
```

### 2.Run TimeRose service
```shell
./storetheindex daemon
```

### 3.Add file with index option

- run ipfs node
```shell
ipfs daemon
```

- add file with index option
```shell
ipfs add ./test.txt --index
```

### 4.Check whether the cid list written to TimeRose
```shell
./storetheindex get -fep http://127.0.0.1:3000 -proto http {cid}
```