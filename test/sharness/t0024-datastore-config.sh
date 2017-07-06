#!/bin/sh

test_description="Test datastore config"

. lib/test-lib.sh

test_init_ipfs

test_launch_ipfs_daemon
test_kill_ipfs_daemon

SPEC_ORIG=$(cat <<EOF)
{
  "mounts": [
    {
      "child": {
        "path": "blocks",
        "shardFunc": "/repo/flatfs/shard/v1/next-to-last/2",
        "sync": true,
        "type": "flatfs"
      },
      "mountpoint": "/blocks",
      "prefix": "flatfs.datastore",
      "type": "measure"
    },
    {
      "child": {
        "compression": "none",
        "path": "datastore",
        "type": "levelds"
      },
      "mountpoint": "/",
      "prefix": "leveldb.datastore",
      "type": "measure"
    }
  ],
  "type": "mount"
}
EOF

SPEC_NOSYNC=$(cat <<'EOF')
{
  "mounts": [
    {
      "child": {
        "path": "blocks",
        "shardFunc": "/repo/flatfs/shard/v1/next-to-last/2",
        "sync": false,
        "type": "flatfs"
      },
      "mountpoint": "/blocks",
      "prefix": "flatfs.datastore",
      "type": "measure"
    },
    {
      "child": {
        "compression": "none",
        "path": "datastore",
        "type": "levelds"
      },
      "mountpoint": "/",
      "prefix": "leveldb.datastore",
      "type": "measure"
    }
  ],
  "type": "mount"
}
EOF

SPEC_NEWSHARDFUN=$(cat <<'EOF')
{
  "mounts": [
    {
      "child": {
        "path": "blocks",
        "shardFunc": "/repo/flatfs/shard/v1/next-to-last/3",
        "sync": true,
        "type": "flatfs"
      },
      "mountpoint": "/blocks",
      "prefix": "flatfs.datastore",
      "type": "measure"
    },
    {
      "child": {
        "compression": "none",
        "path": "datastore",
        "type": "levelds"
      },
      "mountpoint": "/",
      "prefix": "leveldb.datastore",
      "type": "measure"
    }
  ],
  "type": "mount"
}
EOF

test_expect_success "change runtime value in spec config" '
  ipfs config --json Datastore.Spec "$SPEC_NOSYNC"
'

test_launch_ipfs_daemon
test_kill_ipfs_daemon

test_expect_success "change on-disk value in spec config" '
  ipfs config --json Datastore.Spec "$SPEC_NEWSHARDFUN"
'

test_expect_success "can not launch daemon after on-disk value change" '
  test_must_fail ipfs daemon
'

test_done
