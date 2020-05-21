# Use go-ipfs as a library and enable experimental features

Before moving on to this tutorial, you must read first the initial [`go-ipfs` as a library tutorial](../go-ipfs-as-a-library/README.md)
as it gives insights on how to create a repository, the daemon and add a file.

There is only one thing that differs from this example and the first tutorial, which is the function [`createTempRepo`](../go-ipfs-as-a-library/main.go#L49):

```go
func createTempRepo(ctx context.Context) (string, error) {
	repoPath, err := ioutil.TempDir("", "ipfs-shell")
	if err != nil {
		return "", fmt.Errorf("failed to get temp dir: %s", err)
	}

	// Create a config with default options and a 2048 bit key
	cfg, err := config.Init(ioutil.Discard, 2048)
	if err != nil {
		return "", err
	}

	// Create the repo with the config
	err = fsrepo.Init(repoPath, cfg)
	if err != nil {
		return "", fmt.Errorf("failed to init ephemeral node: %s", err)
	}

	return repoPath, nil
}
```

When creating the repository, you can define custom settings on the repository, such as enabling [experimental
features](../../experimental-features.md) or customizing the gateway endpoint.

To do such things, you should modify the variable `cfg`. For example, to enable the sharding experiment, you would modify the function to:

```go
func createTempRepo(ctx context.Context) (string, error) {
	repoPath, err := ioutil.TempDir("", "ipfs-shell")
	if err != nil {
		return "", fmt.Errorf("failed to get temp dir: %s", err)
	}

	// Create a config with default options and a 2048 bit key
	cfg, err := config.Init(ioutil.Discard, 2048)
	if err != nil {
		return "", err
	}
	
	// https://github.com/ipfs/go-ipfs/blob/master/docs/experimental-features.md#ipfs-filestore
	cfg.Experimental.FilestoreEnabled = true
	// https://github.com/ipfs/go-ipfs/blob/master/docs/experimental-features.md#ipfs-urlstore
	cfg.Experimental.UrlstoreEnabled = true
	// https://github.com/ipfs/go-ipfs/blob/master/docs/experimental-features.md#directory-sharding--hamt
	cfg.Experimental.ShardingEnabled = true
	// https://github.com/ipfs/go-ipfs/blob/master/docs/experimental-features.md#ipfs-p2p
	cfg.Experimental.Libp2pStreamMounting = true
	// https://github.com/ipfs/go-ipfs/blob/master/docs/experimental-features.md#p2p-http-proxy
	cfg.Experimental.P2pHttpProxy = true
	// https://github.com/ipfs/go-ipfs/blob/master/docs/experimental-features.md#quic
	cfg.Experimental.QUIC = true
	// https://github.com/ipfs/go-ipfs/blob/master/docs/experimental-features.md#strategic-providing
	cfg.Experimental.StrategicProviding = true

	// Create the repo with the config
	err = fsrepo.Init(repoPath, cfg)
	if err != nil {
		return "", fmt.Errorf("failed to init ephemeral node: %s", err)
	}

	return repoPath, nil
}
```

There are many other options that you can find through the [documentation](https://godoc.org/github.com/ipfs/go-ipfs-config#Config).
