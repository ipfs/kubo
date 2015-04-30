# ipfs launchd agent

A bare-bones launchd agent file for ipfs. To have launchd automatically run the ipfs daemon for you, do the following:

    mkdir -p ~/Library/LaunchAgents
    cp misc/launchctl/io.ipfs.ipfs-daemon.plist ~/Library/LaunchAgents/
    launchctl load ~/Library/LaunchAgents/io.ipfs.ipfs-daemon.plist

Note that the `ipfs` binary must be on the *system* PATH for this to work. Adding a symlink in /usr/bin works well enough for me.
