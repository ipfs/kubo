commander
============

[![Build Status](https://drone.io/github.com/gonuts/commander/status.png)](https://drone.io/github.com/gonuts/commander/latest)

``commander`` is a spin off of [golang](http://golang.org) ``go tool`` infrastructure to provide commands and sub-commands.

A ``commander.Command`` has a ``Subcommands`` field holding ``[]*commander.Command`` subcommands, referenced by name from the command line.

So a ``Command`` can have sub commands.

So you can have, _e.g._:
```sh
$ mycmd action1 [options...]
$ mycmd subcmd1 action1 [options...]
```

Example provided by:
- [hwaf](https://github.com/hwaf/hwaf)
- [examples/my-cmd](examples/my-cmd)

## Documentation
Is available on [godoc](http://godoc.org/github.com/gonuts/commander)

## Installation
Is performed with the usual:
```sh
$ go get github.com/gonuts/commander
```

## Example

See the simple ``my-cmd`` example command for how this all hangs
together [there](http://github.com/gonuts/commander/blob/master/examples/my-cmd/main.go):

```sh
$ my-cmd cmd1
my-cmd-cmd1: hello from cmd1 (quiet=true)

$ my-cmd cmd1 -q
my-cmd-cmd1: hello from cmd1 (quiet=true)

$ my-cmd cmd1 -q=0
my-cmd-cmd1: hello from cmd1 (quiet=false)

$ my-cmd cmd2
my-cmd-cmd2: hello from cmd2 (quiet=true)

$ my-cmd subcmd1 cmd1
my-cmd-subcmd1-cmd1: hello from subcmd1-cmd1 (quiet=true)

$ my-cmd subcmd1 cmd2
my-cmd-subcmd1-cmd2: hello from subcmd1-cmd2 (quiet=true)

$ my-cmd subcmd2 cmd1
my-cmd-subcmd2-cmd1: hello from subcmd2-cmd1 (quiet=true)

$ my-cmd subcmd2 cmd2
my-cmd-subcmd2-cmd2: hello from subcmd2-cmd2 (quiet=true)

$ my-cmd help
Usage:

	my-cmd command [arguments]

The commands are:

    cmd1        runs cmd1 and exits
    cmd2        runs cmd2 and exits
    subcmd1     subcmd1 subcommand. does subcmd1 thingies
    subcmd2     subcmd2 subcommand. does subcmd2 thingies

Use "my-cmd help [command]" for more information about a command.

Additional help topics:


Use "my-cmd help [topic]" for more information about that topic.


$ my-cmd help subcmd1
Usage:

	subcmd1 command [arguments]

The commands are:

    cmd1        runs cmd1 and exits
    cmd2        runs cmd2 and exits


Use "subcmd1 help [command]" for more information about a command.

Additional help topics:


Use "subcmd1 help [topic]" for more information about that topic.

```


## TODO

- automatically generate the bash/zsh/csh autocompletion lists
- automatically generate Readme examples text
- test cases

