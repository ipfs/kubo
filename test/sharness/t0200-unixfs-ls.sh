#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test unixfs ls command"

. lib/test-lib.sh

test_init_ipfs

test_ls_cmd() {

	test_expect_success "'ipfs add -r testData' succeeds" '
		mkdir -p testData testData/d1 testData/d2 &&
		echo "test" >testData/f1 &&
		echo "data" >testData/f2 &&
		echo "hello" >testData/d1/a &&
		random 128 42 >testData/d1/128 &&
		echo "world" >testData/d2/a &&
		random 1024 42 >testData/d2/1024 &&
		ipfs add -r testData >actual_add
	'

	test_expect_success "'ipfs add' output looks good" '
		cat <<-\EOF >expected_add &&
			added QmQNd6ubRXaNG6Prov8o6vk3bn6eWsj9FxLGrAVDUAGkGe testData/d1/128
			added QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN testData/d1/a
			added QmSix55yz8CzWXf5ZVM9vgEvijnEeeXiTSarVtsqiiCJss testData/d1
			added QmbQBUSRL9raZtNXfpTDeaxQapibJEG6qEY8WqAN22aUzd testData/d2/1024
			added QmaRGe7bVmVaLmxbrMiVNXqW4pRNNp3xq7hFtyRKA3mtJL testData/d2/a
			added QmR3jhV4XpxxPjPT3Y8vNnWvWNvakdcT3H6vqpRBsX1MLy testData/d2
			added QmeomffUNfmQy76CQGy9NdmqEnnHU9soCexBnGU3ezPHVH testData/f1
			added QmNtocSs7MoDkJMc1RkyisCSKvLadujPsfJfSdJ3e1eA1M testData/f2
			added QmfNy183bXiRVyrhyWtq3TwHn79yHEkiAGFr18P7YNzESj testData
		EOF
		test_cmp expected_add actual_add
	'

	test_expect_success "'ipfs unixfs ls <three dir hashes>' succeeds" '
		ipfs unixfs ls QmfNy183bXiRVyrhyWtq3TwHn79yHEkiAGFr18P7YNzESj QmR3jhV4XpxxPjPT3Y8vNnWvWNvakdcT3H6vqpRBsX1MLy QmSix55yz8CzWXf5ZVM9vgEvijnEeeXiTSarVtsqiiCJss >actual_ls
	'

	test_expect_success "'ipfs unixfs ls <three dir hashes>' output looks good" '
		cat <<-\EOF >expected_ls &&
			QmfNy183bXiRVyrhyWtq3TwHn79yHEkiAGFr18P7YNzESj:
			d1
			d2
			f1
			f2

			QmR3jhV4XpxxPjPT3Y8vNnWvWNvakdcT3H6vqpRBsX1MLy:
			1024
			a

			QmSix55yz8CzWXf5ZVM9vgEvijnEeeXiTSarVtsqiiCJss:
			128
			a
		EOF
		test_cmp expected_ls actual_ls
	'

	test_expect_success "'ipfs unixfs ls <file hashes>' succeeds" '
		ipfs unixfs ls /ipfs/QmR3jhV4XpxxPjPT3Y8vNnWvWNvakdcT3H6vqpRBsX1MLy/1024 QmQNd6ubRXaNG6Prov8o6vk3bn6eWsj9FxLGrAVDUAGkGe >actual_ls_file
	'

	test_expect_success "'ipfs unixfs ls <file hashes>' output looks good" '
		cat <<-\EOF >expected_ls_file &&
			/ipfs/QmR3jhV4XpxxPjPT3Y8vNnWvWNvakdcT3H6vqpRBsX1MLy/1024
			QmQNd6ubRXaNG6Prov8o6vk3bn6eWsj9FxLGrAVDUAGkGe
		EOF
		test_cmp expected_ls_file actual_ls_file
	'
}


# should work offline
test_ls_cmd

# should work online
test_launch_ipfs_daemon
test_ls_cmd
test_kill_ipfs_daemon

test_done
