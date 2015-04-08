package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	dstore "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	flatfs "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/flatfs"
	leveldb "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/leveldb"
	dsq "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore/query"
	migrate "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-migrate"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"
	mfsr "github.com/ipfs/go-ipfs/repo/fsrepo/migrations"

	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
)

var _ = context.Background

const peerKeyName = "peer.key"

type migration struct{}

func (m migration) Versions() string {
	return "1-to-2"
}

func (m migration) Reversible() bool {
	return true
}

func (m migration) Apply(opts migrate.Options) error {
	repo := mfsr.RepoPath(opts.Path)

	if err := repo.CheckVersion("1"); err != nil {
		return err
	}

	// 1) move key out of config, into its own path
	err := moveKeyOutOfConfig(opts.Path)
	if err != nil {
		return err
	}

	// 2) Transfer blocks out of leveldb into flatDB
	err = transferBlocksToFlatDB(opts.Path)
	if err != nil {
		return err
	}

	// 3) move ipfs path from .go-ipfs to .ipfs
	newpath, err := moveIpfsDir(opts.Path)
	if err != nil {
		return err
	}

	// 4) Update version number
	repo = mfsr.RepoPath(newpath)
	err = repo.WriteVersion("2")
	if err != nil {
		return err
	}

	return nil
}

func (m migration) Revert(opts migrate.Options) error {
	repo := mfsr.RepoPath(opts.Path)
	if err := repo.CheckVersion("2"); err != nil {
		return err
	}

	// 1) Move directory back to .go-ipfs
	npath, err := reverseIpfsDir(opts.Path)
	if err != nil {
		return err
	}

	// 2) move blocks back from flatfs to leveldb
	err = transferBlocksFromFlatDB(npath)
	if err != nil {
		return err
	}

	// 3) move key back into config
	err = moveKeyIntoConfig(npath)
	if err != nil {
		return err
	}

	// 4) change version number back down
	repo = mfsr.RepoPath(npath)
	err = repo.WriteVersion("1")
	if err != nil {
		return err
	}

	return nil
}

func transferBlocksToFlatDB(repopath string) error {
	r, err := fsrepo.Open(repopath)
	if err != nil {
		return err
	}

	blockspath := path.Join(repopath, "blocks")
	err = os.Mkdir(blockspath, 0777)
	if err != nil {
		return err
	}

	fds, err := flatfs.New(blockspath, 4)
	if err != nil {
		return err
	}

	return transferBlocks(r.Datastore(), fds, "/b/", "")
}

func transferBlocksFromFlatDB(repopath string) error {

	ldbpath := path.Join(repopath, "datastore")
	blockspath := path.Join(repopath, "blocks")
	fds, err := flatfs.New(blockspath, 4)
	if err != nil {
		return err
	}

	ldb, err := leveldb.NewDatastore(ldbpath, nil)
	if err != nil {
		return err
	}

	err = transferBlocks(fds, ldb, "", "/b/")
	if err != nil {
		return err
	}

	// Now remove the blocks directory
	err = os.RemoveAll(blockspath)
	if err != nil {
		return err
	}

	return nil
}

func transferBlocks(from, to dstore.Datastore, fpref, tpref string) error {
	q := dsq.Query{Prefix: fpref, KeysOnly: true}
	res, err := from.Query(q)
	if err != nil {
		return err
	}

	fmt.Println("Starting query")
	for result := range res.Next() {
		nkey := fmt.Sprintf("%s%s", tpref, result.Key[len(fpref):])

		fkey := dstore.NewKey(result.Key)
		val, err := from.Get(fkey)
		if err != nil {
			return err
		}

		err = to.Put(dstore.NewKey(nkey), val)
		if err != nil {
			return err
		}

		err = from.Delete(fkey)
		if err != nil {
			return err
		}
	}
	fmt.Println("Query done")

	return nil
}

func moveKeyOutOfConfig(repopath string) error {
	// Make keys directory
	keypath := path.Join(repopath, "keys")
	err := os.Mkdir(keypath, 0777)
	if err != nil {
		return err
	}

	// Grab the config
	cfg, err := loadConfigJSON(repopath)
	if err != nil {
		return err
	}

	// get the private key from it
	privKey, err := getPrivateKeyFromConfig(cfg)
	if err != nil {
		return err
	}

	keyfilepath := path.Join(keypath, peerKeyName)
	fi, err := os.OpenFile(keyfilepath, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	// Write our b64-protobuf encoded key
	_, err = fi.WriteString(privKey)
	if err != nil {
		return err
	}

	err = fi.Close()
	if err != nil {
		return err
	}

	// Now that the key is safely in its own file, remove it from the config
	err = clearPrivateKeyFromConfig(cfg)
	if err != nil {
		return err
	}

	err = saveConfigJSON(repopath, cfg)
	if err != nil {
		return err
	}

	return nil
}

// Part of the 2-to-1 revert process
func moveKeyIntoConfig(repopath string) error {
	// Make keys directory
	keypath := path.Join(repopath, "keys")

	// Grab the config
	cfg, err := loadConfigJSON(repopath)
	if err != nil {
		return err
	}

	keyfilepath := path.Join(keypath, peerKeyName)
	pkey, err := ioutil.ReadFile(keyfilepath)
	if err != nil {
		return err
	}

	id, ok := cfg["Identity"]
	if !ok {
		return errors.New("expected to find an identity object in config")
	}
	identity, ok := id.(map[string]interface{})
	if !ok {
		return errors.New("expected Identity in config to be an object")
	}
	identity["PrivKey"] = string(pkey)

	err = saveConfigJSON(repopath, cfg)
	if err != nil {
		return err
	}

	// Now that the key is safely in the config, delete the file
	err = os.RemoveAll(keypath)
	if err != nil {
		return err
	}

	return nil
}

func moveIpfsDir(curpath string) (string, error) {
	newpath := strings.Replace(curpath, ".go-ipfs", ".ipfs", 1)
	return newpath, os.Rename(curpath, newpath)
}

func reverseIpfsDir(curpath string) (string, error) {
	newpath := strings.Replace(curpath, ".ipfs", ".go-ipfs", 1)
	return newpath, os.Rename(curpath, newpath)
}

func loadConfigJSON(repoPath string) (map[string]interface{}, error) {
	cfgPath := path.Join(repoPath, "config")
	fi, err := os.Open(cfgPath)
	if err != nil {
		return nil, err
	}

	var out map[string]interface{}
	err = json.NewDecoder(fi).Decode(&out)
	if err != nil {
		return nil, err
	}

	return out, nil
}

func saveConfigJSON(repoPath string, cfg map[string]interface{}) error {
	cfgPath := path.Join(repoPath, "config")
	fi, err := os.Create(cfgPath)
	if err != nil {
		return err
	}

	out, err := json.MarshalIndent(cfg, "", "\t")
	if err != nil {
		return err
	}

	_, err = fi.Write(out)
	if err != nil {
		return err
	}

	return nil
}

func getPrivateKeyFromConfig(cfg map[string]interface{}) (string, error) {
	ident, ok := cfg["Identity"]
	if !ok {
		return "", errors.New("no identity found in config")
	}

	identMap, ok := ident.(map[string]interface{})
	if !ok {
		return "", errors.New("expected Identity to be object (map)")
	}

	privkey, ok := identMap["PrivKey"]
	if !ok {
		return "", errors.New("no PrivKey field found in Identity")
	}

	privkeyStr, ok := privkey.(string)
	if !ok {
		return "", errors.New("expected PrivKey to be a string")
	}

	return privkeyStr, nil
}

func clearPrivateKeyFromConfig(cfg map[string]interface{}) error {
	ident, ok := cfg["Identity"]
	if !ok {
		return errors.New("no identity found in config")
	}

	identMap, ok := ident.(map[string]interface{})
	if !ok {
		return errors.New("expected Identity to be object (map)")
	}

	delete(identMap, "PrivKey")
	return nil
}

func main() {
	m := migration{}
	migrate.Main(&m)
}
