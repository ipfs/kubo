# Dataset description/sources

- lotus_testnet_export_256_multiroot.car
  - Export of the first 256 block of the testnet chain, with 3 tipset roots. Exported from Lotus by @traviperson on 2019-03-18


- lotus_devnet_genesis.car
  - Source: https://github.com/filecoin-project/lotus/blob/v0.2.10/build/genesis/devnet.car

- lotus_testnet_export_128.car
  - Export of the first 128 block of the testnet chain, exported from Lotus by @traviperson on 2019-03-24


- lotus_devnet_genesis_shuffled_noroots.car
- lotus_testnet_export_128_shuffled_noroots.car
  - versions of the above with an **empty** root array, and having all blocks shuffled

- lotus_devnet_genesis_shuffled_nulroot.car
- lotus_testnet_export_128_shuffled_nulroot.car
  - versions identical to the above, but with a single "empty-block" root each ( in order to work around go-car not following the current "roots can be empty" spec )

- combined_naked_roots_genesis_and_128.car
  - only the roots of `lotus_devnet_genesis.car` and `lotus_testnet_export_128.car`,to be used in combination with the root-less parts to validate "transactional" pinning

- lotus_testnet_export_128_v2.car
- lotus_devnet_genesis_v2.car
  - generated with `car index lotus_testnet_export_128.car > lotus_testnet_export_128_v2.car`
  - install `go-car` CLI from https://github.com/ipld/go-car

- partial-dag-scope-entity.car
  - unixfs directory entity exported from gateway via `?format=car&dag-scope=entity` ([IPIP-402](https://github.com/ipfs/specs/pull/402))
  - CAR roots includes directory CID, but only the root block is included in the CAR, making the DAG incomplete
