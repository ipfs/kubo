#!/bin/sh
set -e

# Execute the daemon subcommand by default
/sbin/tini -s -- /usr/local/bin/start_ipfs daemon \
    --migrate=true \
    --agent-version-suffix=docker \
    ${CMD_EXTRA_FLAGS}
