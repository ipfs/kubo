# ![](https://img.shields.io/badge/status-wip-orange.svg?style=flat-square) Keystore

**Authors(s):**
- [whyrusleeping](github.com/whyrusleeping)
- [Hector Sanjuan](github.com/hsanjuan)

**Abstract**

This spec provides definitions and operations for the keystore feature in IPFS.

# Table of Contents

- [Goals](#goals)
- [Planned Implementation](#planned-implementation)
  - [Key storage](#key-storage)
  - [Interface](#interface)
  - [Code changes and additions](#code-changes-and-additions)
  - [Structures](#structures)

## Goals

To have a secure, simple and user-friendly way of storing and managing keys
for use by ipfs. As well as the ability to share these keys, encrypt, decrypt,
sign and verify data.

## Planned Implementation

### Key storage

Storage layout and format is defined in the [`repository_fs`](repository_fs.md) part of the spec.

### Interface

#### ipfs key

```
USAGE
  ipfs key - Create and list IPNS name keypairs

  ipfs key

  'ipfs key gen' generates a new keypair for usage with IPNS and 'ipfs name
  publish'.
  
    > ipfs key gen --type=rsa --size=2048 mykey
    > ipfs name publish --key=mykey QmSomeHash
  
  'ipfs key list' lists the available keys.
  
    > ipfs key list
    self
    mykey
  		

SUBCOMMANDS
  ipfs key export <name>           - Export a keypair
  ipfs key gen <name>              - Create a new keypair
  ipfs key import <name> <key>     - Import a key and prints imported key id
  ipfs key list                    - List all local keypairs.
  ipfs key rename <name> <newName> - Rename a keypair.
  ipfs key rm <name>...            - Remove a keypair.
  ipfs key rotate                  - Rotates the IPFS identity.

  For more information about each command, use:
  'ipfs key <subcmd> --help'
```

#### ipfs crypt

**NOTE:** as of 2023 Q4, `ipfs crypt` commands are not implemented yet.

```
    ipfs crypt - Perform cryptographic operations using ipfs keypairs

SUBCOMMANDS:

    ipfs crypt sign <data>          - Generates a signature for the given data with a specified key
    ipfs crypt verify <data> <sig>  - Verify that the given data and signature match
    ipfs crypt encrypt <data>       - Encrypt the given data
    ipfs crypt decrypt <data>       - Decrypt the given data

DESCRIPTION:

    `ipfs crypt` is a command used to perform various cryptographic operations
    using ipfs keypairs, including: signing, verifying, encrypting and decrypting.
```

#### Some subcommands:

##### ipfs key Gen


```
USAGE
  ipfs key gen <name> - Create a new keypair

SYNOPSIS
  ipfs key gen [--type=<type> | -t] [--size=<size> | -s]
               [--ipns-base=<ipns-base>] [--] <name>

ARGUMENTS

  <name> - name of key to create

OPTIONS

  -t, --type   string - type of the key to create: rsa, ed25519. Default:
                        ed25519.
  -s, --size   int    - size of the key to generate.
  --ipns-base  string - Encoding used for keys: Can either be a multibase
                        encoded CID or a base58btc encoded multihash. Takes
                        {b58mh|base36|k|base32|b...}. Default: base36.
```

* * *

##### Key Send

```
USAGE
  ipfs key - Create and list IPNS name keypairs

SYNOPSIS
  ipfs key

DESCRIPTION

  'ipfs key gen' generates a new keypair for usage with IPNS and 'ipfs name
  publish'.
  
    > ipfs key gen --type=rsa --size=2048 mykey
    > ipfs name publish --key=mykey QmSomeHash
  
  'ipfs key list' lists the available keys.
  
    > ipfs key list
    self
    mykey
  		

SUBCOMMANDS
  ipfs key export <name>           - Export a keypair
  ipfs key gen <name>              - Create a new keypair
  ipfs key import <name> <key>     - Import a key and prints imported key id
  ipfs key list                    - List all local keypairs.
  ipfs key rename <name> <newName> - Rename a keypair.
  ipfs key rm <name>...            - Remove a keypair.
  ipfs key rotate                  - Rotates the IPFS identity.

  For more information about each command, use:
  'ipfs key <subcmd> --help'
```

##### Comments:

Ensure that the user knows the implications of sending a key.

* * *

##### Crypt Encrypt

```
    ipfs crypt encrypt <data> - Encrypt the given data with a specified key

ARGUMENTS:

    data                        - The filename of the data to be encrypted ("-" for stdin)

OPTIONS:

    -k, -key        string        - The name of the key to use for encryption (default: localkey)
    -o, -output     string        - The name of the output file (default: stdout)
    -c, -cipher     string        - The cipher to use for the operation
    -m, -mode       string        - The block cipher mode to use for the operation

DESCRIPTION:

    'ipfs crypt encrypt' is a command used to encrypt data so that only holders of a certain
    key can read it.
```

##### Comments:

This should probably just operate on raw data and not on DAGs.

* * *

##### Other Interface Changes

We will also need to make additions to support keys in other commands, these changes are as follows:

- `ipfs add`
    - Support for a `-encrypt-key` option, for block encrypting the file being added with the key
        - also adds an 'encrypted' node above the root unixfs node
    - Support for a `-sign-key` option to attach a signature node above the root unixfs node

- `ipfs block put`
    - Support for a `-encrypt-key` option, for encrypting the block before hashing and storing

- `ipfs object put`
    - Support for a `-encrypt-key` option, for encrypting the object before hashing and storing

- `ipfs name publish`
    - Support for a `-key` option to select which keyspace to publish to

### Code changes and additions

This sections outlines code organization around this feature.

#### Keystore package

The fsrepo carries a `keystore` that can be used to load/store keys. The keystore is implemented following this interface:

```go
// Keystore provides a key management interface
type Keystore interface {
	// Has returns whether or not a key exist in the Keystore
	Has(string) (bool, error)
	// Put stores a key in the Keystore, if a key with the same name already exists, returns ErrKeyExists
	Put(string, ci.PrivKey) error
	// Get retrieves a key from the Keystore if it exists, and returns ErrNoSuchKey
	// otherwise.
	Get(string) (ci.PrivKey, error)
	// Delete removes a key from the Keystore
	Delete(string) error
	// List returns a list of key identifier
	List() ([]string, error)
}
```

Note: Never store passwords as strings, strings cannot be zeroed out after they are used.
using a byte array allows you to write zeroes over the memory so that the users password
does not linger in memory.

#### Unixfs

- new node types, 'encrypted' and 'signed', probably shouldn't be in unixfs, just understood by it
- if new node types are not unixfs nodes, special consideration must be given to the interop

- DagReader needs to be able to access keystore to seamlessly stream encrypted data we have keys for
    - also needs to be able to verify signatures

#### Importer

- DagBuilderHelper needs to be able to encrypt blocks
    - Dag Nodes should be generated like normal, then encrypted, and their parents should
        link to the hash of the encrypted node
- DagBuilderParams should have extra parameters to accommodate creating a DBH that encrypts the blocks

#### New 'Encrypt' package

Should contain code for crypto operations on dags.

Encryption of dags should work by first generating a symmetric key, and using
that key to encrypt all the data. That key should then be encrypted with the
public key chosen and stored in the Encrypted DAG structure.

Note: One option is to simply add it to the key interface.

### Structures
Some tentative mockups (in json) of the new DAG structures for signing and encrypting

Signed DAG:
```
{
    "Links" : [
        {
            "Name":"@content",
            "Hash":"QmTheContent",
        }
    ],
    "Data": protobuf{
        "Type":"Signed DAG",
        "Signature": "thesignature",
        "PubKeyID": "QmPubKeyHash",
    }
}
```

Encrypted DAG:
```
{
    "Links" : [
        {
            "Name":"@content",
            "Hash":"QmRawEncryptedDag",
        }
    ],
    "Data": protobuf{
        "Type":"Encrypted DAG",
        "PubKeyID": "QmPubKeyHash",
        "Key": "ephemeral symmetric key, encrypted with public key",
    }
}
```
