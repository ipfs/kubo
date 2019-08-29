#!/usr/bin/env bash
#
# Copyright (c) 2019 Protocol Labs
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test plugin loading"

. lib/test-lib.sh

if ! test_have_prereq PLUGIN; then
    skip_all='skipping plugin tests, plugins not available'

    test_done
fi

test_init_ipfs

test_expect_success "ipfs id succeeds" '
  ipfs id
'

test_expect_success "make a bad plugin" '
  mkdir -p "$IPFS_PATH/plugins" &&
  echo foobar > "$IPFS_PATH/plugins/foo.so" &&
  chmod +x "$IPFS_PATH/plugins/foo.so"
'

test_expect_success "ipfs id fails due to a bad plugin" '
  test_expect_code 1 ipfs id
'

test_expect_success "cleanup bad plugin" '
  rm "$IPFS_PATH/plugins/foo.so"
'

test_expect_success "install test plugin" '
  go build \
    -asmflags=all="-trimpath=${GOPATH}" -gcflags=all="-trimpath=${GOPATH}" \
    -buildmode=plugin -o "$IPFS_PATH/plugins/example.so" ../t0280-plugin-data/example.go &&
  chmod +x "$IPFS_PATH/plugins/example.so"
'

test_plugin() {
  local loads="$1"
  local repo="$2"
  local config="$3"

  rm -f id_raw_output id_output id_output_expected

  test_expect_success "id runs" '
    ipfs id 2>id_raw_output >/dev/null
  '

  test_expect_success "filter test plugin output" '
    sed -ne "s/^testplugin //p" id_raw_output >id_output
  '

  if [ "$loads" != "true" ]; then
    test_expect_success "plugin doesn't load" '
      test_must_be_empty id_output
    '
  else
    test_expect_success "plugin produces the correct output" '
      echo "$repo" >id_output_expected &&
      echo "$config" >>id_output_expected &&
      test_cmp id_output id_output_expected
    '
  fi
}

test_plugin true "$IPFS_PATH" "<nil>"

test_expect_success "disable the plugin" '
  ipfs config --json Plugins.Plugins.test-plugin.Disabled true
'

test_plugin false

test_expect_success "re-enable the plugin" '
  ipfs config --json Plugins.Plugins.test-plugin.Disabled false
'

test_plugin true "$IPFS_PATH" "<nil>"

test_expect_success "configure the plugin" '
  ipfs config Plugins.Plugins.test-plugin.Config foobar
'

test_plugin true "$IPFS_PATH" "foobar"

test_done
