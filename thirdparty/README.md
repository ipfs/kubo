thirdparty consists of Golang packages that contain no go-ipfs dependencies and
may be vendored jbenet/go-ipfs at a later date.

packages in under this directory _must not_ import packages under
`jbenet/go-ipfs` that are not also under `thirdparty`.
