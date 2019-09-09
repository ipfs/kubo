/*
Package filesystem is an experimental package that implements the go-ipfs daemon plugin interface
and defines the plugin's config structure. The plugin itself exposes file system services over a multiaddr listener.

By default, we try to expose the IPFS namespace using the 9P2000.L protocol, over a unix domain socket
(located at $IPFS_PATH/filesystem.9P.sock)

To set the multiaddr listen address, you may use the environment variable $IPFS_FS_ADDR, or set the option in the node's config file
via `ipfs config --json 'Plugins.Plugins.filesystem.Config "Config":{"Service":{"9P":"/ip4/127.0.0.1/tcp/564"}}'`
To disable this plugin entirely, use: `ipfs config --json Plugins.Plugins.filesystem.Disabled true`
*/
package filesystem
