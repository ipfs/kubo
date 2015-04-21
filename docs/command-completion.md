Command Completion
==================

Shell command completion is provided by the script at 
`/misc/completion/ipfs-completion.bash`.


Installation
------------
The simplest way to see it working is to run 
`source misc/completion/ipfs-completion.bash` straight from your shell. This
is only temporary and to fully enable it, you'll have to follow one of the steps
below.

### Bash on Linux
For bash, completion can be enabled in a couple of ways. One is to add
```bash
if [ -n "$GOPATH" ]; then
  source $GOPATH/src/github.com/ipfs/go-ipfs/misc/completion/ipfs-completion.bash
fi
```
into your `~/.bash_completion`. It will automatically be loaded the next time 
bash is loaded.
To enable ipfs command completion globally on your system you may also 
copy the completion script to `/etc/bash_completion.d/`.


Additional References
---------------------
* https://www.debian-administration.org/article/316/An_introduction_to_bash_completion_part_1
