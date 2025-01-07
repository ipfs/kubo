<h1 align="center">
  <br>
  <a href="#readme"><img src="https://github.com/ipfs-shipyard/nopfs/blob/41484a818e6542314f784da852fc41b76f2d48a6/logo.png?raw=true" alt="content blocking logo" title="content blocking in Kubo" width="200"></a>
  <br>
  Content Blocking in Kubo
  <br>
</h1>

Kubo ships with built-in support for denylist format from [IPIP-383](https://github.com/ipfs/specs/pull/383).

## Default behavior

Official Kubo build does not ship with any denylists enabled by default.

Content blocking is an opt-in decision made by the operator of `ipfs daemon`.

## How to enable blocking

Place a `*.deny` file in one of directories:

- `$IPFS_PATH/denylists/` (`$HOME/.ipfs/denylists/` if `IPFS_PATH` is not set)
- `$XDG_CONFIG_HOME/ipfs/denylists/` (`$HOME/.config/ipfs/denylists/` if `XDG_CONFIG_HOME` is not  set)
- `/etc/ipfs/denylists/` (global)

Files need to be present before starting the `ipfs daemon` in order to be watched for any new updates 
appended  once started.  Any other changes (such as removal of entries, prepending of entries, or 
insertion of new entries before the EOF at time of daemon starting) will not be detected or processed
after boot; a restart of the daemon will be required for them to be factored in.

If an entire new denylist file is added, `ipfs daemon` also needs to be restarted to track it.

CLI and Gateway users will receive errors in response to request impacted by a blocklist:

```
Error: /ipfs/QmQvjk82hPkSaZsyJ8vNER5cmzKW7HyGX5XVusK7EAenCN is blocked and cannot be provided
```

End user is not informed about the exact reason, see [How to
debug](#how-to-debug) if you need to find out which line of which denylist
caused the request to be blocked.

## Denylist file format

[NOpfs](https://github.com/ipfs-shipyard/nopfs) supports the format from [IPIP-383](https://github.com/ipfs/specs/pull/383).

Clear-text rules are simple: just put content paths to block, one per line.
Paths with unicode and whitespace need to be percent-encoded:

```
/ipfs/QmbWqxBEKC3P8tqsKc98xmWNzrzDtRLMiMPL8wBuTGsMnR
/ipfs/bafybeihfg3d7rdltd43u3tfvncx7n5loqofbsobojcadtmokrljfthuc7y/927%20-%20Standards/927%20-%20Standards.png
```

Sensitive content paths can be double-hashed to block without revealing them.
Double-hashed list example: https://badbits.dwebops.pub/badbits.deny

See [IPIP-383](https://github.com/ipfs/specs/pull/383) for detailed format specification and more examples.

## How to suspend blocking without removing denylists

Set `IPFS_CONTENT_BLOCKING_DISABLE` environment variable to `true` and restart the daemon.


## How to debug

Debug logging of `nopfs` subsystem can be enabled with `GOLOG_LOG_LEVEL="nopfs=debug"`

All block events are logged as warnings on a separate level named `nopfs-blocks`.

To only log requests for blocked content set `GOLOG_LOG_LEVEL="nopfs-blocks=warn"`:

```
WARN (...) QmRFniDxwxoG2n4AcnGhRdjqDjCM5YeUcBE75K8WXmioH3: blocked (test.deny:9)
```


