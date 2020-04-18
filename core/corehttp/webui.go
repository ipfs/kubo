package corehttp

// TODO: move to IPNS
const WebUIPath = "/ipfs/bafybeidatpz2hli6fgu3zul5woi27ujesdf5o5a7bu622qj6ugharciwjq"

// this is a list of all past webUI paths.
var WebUIPaths = []string{
	WebUIPath,
	"/ipfs/QmfQkD8pBSBCBxWEwFSu4XaDVSWK6bjnNuaWZjMyQbyDub",
	"/ipfs/QmXc9raDM1M5G5fpBnVyQ71vR4gbnskwnB9iMEzBuLgvoZ",
	"/ipfs/QmenEBWcAk3tN94fSKpKFtUMwty1qNwSYw3DMDFV6cPBXA",
	"/ipfs/QmUnXcWZC5Ve21gUseouJsH5mLAyz5JPp8aHsg8qVUUK8e",
	"/ipfs/QmSDgpiHco5yXdyVTfhKxr3aiJ82ynz8V14QcGKicM3rVh",
	"/ipfs/QmRuvWJz1Fc8B9cTsAYANHTXqGmKR9DVfY5nvMD1uA2WQ8",
	"/ipfs/QmQLXHs7K98JNQdWrBB2cQLJahPhmupbDjRuH1b9ibmwVa",
	"/ipfs/QmXX7YRpU7nNBKfw75VG7Y1c3GwpSAGHRev67XVPgZFv9R",
	"/ipfs/QmXdu7HWdV6CUaUabd9q2ZeA4iHZLVyDRj3Gi4dsJsWjbr",
	"/ipfs/QmaaqrHyAQm7gALkRW8DcfGX3u8q9rWKnxEMmf7m9z515w",
	"/ipfs/QmSHDxWsMPuJQKWmVA1rB5a3NX2Eme5fPqNb63qwaqiqSp",
	"/ipfs/QmctngrQAt9fjpQUZr7Bx3BsXUcif52eZGTizWhvcShsjz",
	"/ipfs/QmS2HL9v5YeKgQkkWMvs1EMnFtUowTEdFfSSeMT4pos1e6",
	"/ipfs/QmR9MzChjp1MdFWik7NjEjqKQMzVmBkdK3dz14A6B5Cupm",
	"/ipfs/QmRyWyKWmphamkMRnJVjUTzSFSAAZowYP4rnbgnfMXC9Mr",
	"/ipfs/QmU3o9bvfenhTKhxUakbYrLDnZU7HezAVxPM6Ehjw9Xjqy",
	"/ipfs/QmPhnvn747LqwPYMJmQVorMaGbMSgA7mRRoyyZYz3DoZRQ",
	"/ipfs/QmQNHd1suZTktPRhP7DD4nKWG46ZRSxkwHocycHVrK3dYW",
	"/ipfs/QmNyMYhwJUS1cVvaWoVBhrW8KPj1qmie7rZcWo8f1Bvkhz",
	"/ipfs/QmVTiRTQ72qiH4usAGT4c6qVxCMv4hFMUH9fvU6mktaXdP",
	"/ipfs/QmYcP4sp1nraBiCYi6i9kqdaKobrK32yyMpTrM5JDA8a2C",
	"/ipfs/QmUtMmxgHnDvQq4bpH6Y9MaLN1hpfjJz5LZcq941BEqEXs",
	"/ipfs/QmPURAjo3oneGH53ovt68UZEBvsc8nNmEhQZEpsVEQUMZE",
	"/ipfs/QmeSXt32frzhvewLKwA1dePTSjkTfGVwTh55ZcsJxrCSnk",
	"/ipfs/QmcjeTciMNgEBe4xXvEaA4TQtwTRkXucx7DmKWViXSmX7m",
	"/ipfs/QmfNbSskgvTXYhuqP8tb9AKbCkyRcCy3WeiXwD9y5LeoqK",
	"/ipfs/QmPkojhjJkJ5LEGBDrAvdftrjAYmi9GU5Cq27mWvZTDieW",
	"/ipfs/Qmexhq2sBHnXQbvyP2GfUdbnY7HCagH2Mw5vUNSBn2nxip",
}

var WebUIOption = RedirectOption("webui", WebUIPath)
