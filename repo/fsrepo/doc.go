// package fsrepo
//
// TODO explain the package roadmap...
//
//   .go-ipfs/
//   ├── client/
//   |   ├── client.lock          <------ protects client/ + signals its own pid
//   │   ├── ipfs-client.cpuprof
//   │   ├── ipfs-client.memprof
//   │   └── logs/
//   ├── config
//   ├── daemon/
//   │   ├── daemon.lock          <------ protects daemon/ + signals its own address
//   │   ├── ipfs-daemon.cpuprof
//   │   ├── ipfs-daemon.memprof
//   │   └── logs/
//   ├── datastore/
//   ├── repo.lock                <------ protects datastore/ and config
//   └── version
package fsrepo
