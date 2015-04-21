Command Completion
==================

Shell command completion is provided by the script at 
`/misc/completion/ipfs-completion.bash`.

The simplest way to see it working is to run 
`source misc/completion/ipfs-completion.bash` straight from your shell. This
is only temporary and to fully enable it, you'll have to follow one of the steps
below.

Linux
-----
### Bash

For bash, completion can be enabled in a couple of ways. One is to add the line
`source $GOPATH/src/github.com/ipfs/go-ipfs/misc/completion/ipfs-completion.bash` 
into your `~/.bash_completion`. It will automatically be loaded the next time 
bash is loaded.
To enable ipfs command completion globally on your system you may also 
copy the completion script to `/etc/bash_completion.d/`.
