# ipfs launchd agent

A bare-bones launchd agent file for ipfs. To have launchd automatically run the ipfs daemon for you, run `./misc/launchd/install.sh`

Note that the `ipfs` binary must be on the *system* PATH for this to work. Adding a symlink in /usr/bin works well enough for me.
