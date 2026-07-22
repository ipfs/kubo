#!/usr/bin/env bash
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test ls command"

. lib/test-lib.sh

test_init_ipfs

test_ls_cmd() {
  test_expect_success "'ipfs add -r testData' succeeds" '
    mkdir -p testData testData/d1 testData/d2 &&
    echo "test" >testData/f1 &&
    echo "data" >testData/f2 &&
    echo "hello" >testData/d1/a &&
    random-data -size=128 -seed=42 >testData/d1/128 &&
    echo "world" >testData/d2/a &&
    random-data -size=1024 -seed=42 >testData/d2/1024 &&
    echo "badname" >testData/d2/`echo -e "bad\x7fname.txt"` &&
    ipfs add -r testData >actual_add
  '

  test_expect_success "'ipfs add' output looks good" '
    cat <<-\EOF >expected_add &&
added QmYGeKXr7Jef8FgFbrb6X2cY45od7anc2QstAJbWT2E5ak testData/d1/128
added QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN testData/d1/a
added QmUc94gA5Vy3tp2gnBx4YnAQ3msnqDc4sdEUsZsrBV65Q3 testData/d2/1024
added QmaRGe7bVmVaLmxbrMiVNXqW4pRNNp3xq7hFtyRKA3mtJL testData/d2/a
added QmQSLRRd1Lxn6NMsWmmj2g9W3LtSRfmVAVqU3ShneLUrbn testData/d2/bad\x7fname.txt
added QmeomffUNfmQy76CQGy9NdmqEnnHU9soCexBnGU3ezPHVH testData/f1
added QmNtocSs7MoDkJMc1RkyisCSKvLadujPsfJfSdJ3e1eA1M testData/f2
added QmXQKyr8cT6erZHVqcBBASydmkSF9CnkC8AnTV8UhimExH testData/d1
added QmZX7bVCyEuxB6Mzm33yP77PQVuSFvveugDdenzH2yHnd6 testData/d2
added QmYC14BmpHUBMttbWaqbdR81RMcZfSLau6PSazhpRWi1My testData
EOF
    test_cmp expected_add actual_add
  '

  test_expect_success "'ipfs ls <three dir hashes>' succeeds" '
    ipfs ls QmYC14BmpHUBMttbWaqbdR81RMcZfSLau6PSazhpRWi1My QmZX7bVCyEuxB6Mzm33yP77PQVuSFvveugDdenzH2yHnd6 QmXQKyr8cT6erZHVqcBBASydmkSF9CnkC8AnTV8UhimExH >actual_ls
  '

  test_expect_success "'ipfs ls <three dir hashes>' output looks good" '
    cat <<-\EOF >expected_ls &&
QmYC14BmpHUBMttbWaqbdR81RMcZfSLau6PSazhpRWi1My:
QmXQKyr8cT6erZHVqcBBASydmkSF9CnkC8AnTV8UhimExH - d1/
QmZX7bVCyEuxB6Mzm33yP77PQVuSFvveugDdenzH2yHnd6 - d2/
QmeomffUNfmQy76CQGy9NdmqEnnHU9soCexBnGU3ezPHVH 5 f1
QmNtocSs7MoDkJMc1RkyisCSKvLadujPsfJfSdJ3e1eA1M 5 f2

QmZX7bVCyEuxB6Mzm33yP77PQVuSFvveugDdenzH2yHnd6:
QmUc94gA5Vy3tp2gnBx4YnAQ3msnqDc4sdEUsZsrBV65Q3 1024 1024
QmaRGe7bVmVaLmxbrMiVNXqW4pRNNp3xq7hFtyRKA3mtJL 6    a
QmQSLRRd1Lxn6NMsWmmj2g9W3LtSRfmVAVqU3ShneLUrbn 8    bad\x7fname.txt

QmXQKyr8cT6erZHVqcBBASydmkSF9CnkC8AnTV8UhimExH:
QmYGeKXr7Jef8FgFbrb6X2cY45od7anc2QstAJbWT2E5ak 128 128
QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN 6   a
EOF
    test_cmp expected_ls actual_ls
  '

  test_expect_success "'ipfs ls --size=false <three dir hashes>' succeeds" '
    ipfs ls --size=false QmYC14BmpHUBMttbWaqbdR81RMcZfSLau6PSazhpRWi1My QmZX7bVCyEuxB6Mzm33yP77PQVuSFvveugDdenzH2yHnd6 QmXQKyr8cT6erZHVqcBBASydmkSF9CnkC8AnTV8UhimExH >actual_ls
  '

  test_expect_success "'ipfs ls <three dir hashes>' output looks good" '
    cat <<-\EOF >expected_ls &&
QmYC14BmpHUBMttbWaqbdR81RMcZfSLau6PSazhpRWi1My:
QmXQKyr8cT6erZHVqcBBASydmkSF9CnkC8AnTV8UhimExH d1/
QmZX7bVCyEuxB6Mzm33yP77PQVuSFvveugDdenzH2yHnd6 d2/
QmeomffUNfmQy76CQGy9NdmqEnnHU9soCexBnGU3ezPHVH f1
QmNtocSs7MoDkJMc1RkyisCSKvLadujPsfJfSdJ3e1eA1M f2

QmZX7bVCyEuxB6Mzm33yP77PQVuSFvveugDdenzH2yHnd6:
QmUc94gA5Vy3tp2gnBx4YnAQ3msnqDc4sdEUsZsrBV65Q3 1024
QmaRGe7bVmVaLmxbrMiVNXqW4pRNNp3xq7hFtyRKA3mtJL a
QmQSLRRd1Lxn6NMsWmmj2g9W3LtSRfmVAVqU3ShneLUrbn bad\x7fname.txt

QmXQKyr8cT6erZHVqcBBASydmkSF9CnkC8AnTV8UhimExH:
QmYGeKXr7Jef8FgFbrb6X2cY45od7anc2QstAJbWT2E5ak 128
QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN a
EOF
    test_cmp expected_ls actual_ls
  '

  test_expect_success "'ipfs ls --headers <three dir hashes>' succeeds" '
    ipfs ls --headers QmYC14BmpHUBMttbWaqbdR81RMcZfSLau6PSazhpRWi1My QmZX7bVCyEuxB6Mzm33yP77PQVuSFvveugDdenzH2yHnd6 QmXQKyr8cT6erZHVqcBBASydmkSF9CnkC8AnTV8UhimExH >actual_ls_headers
  '

  test_expect_success "'ipfs ls --headers  <three dir hashes>' output looks good" '
    cat <<-\EOF >expected_ls_headers &&
QmYC14BmpHUBMttbWaqbdR81RMcZfSLau6PSazhpRWi1My:
Hash                                           Size Name
QmXQKyr8cT6erZHVqcBBASydmkSF9CnkC8AnTV8UhimExH -    d1/
QmZX7bVCyEuxB6Mzm33yP77PQVuSFvveugDdenzH2yHnd6 -    d2/
QmeomffUNfmQy76CQGy9NdmqEnnHU9soCexBnGU3ezPHVH 5    f1
QmNtocSs7MoDkJMc1RkyisCSKvLadujPsfJfSdJ3e1eA1M 5    f2

QmZX7bVCyEuxB6Mzm33yP77PQVuSFvveugDdenzH2yHnd6:
Hash                                           Size Name
QmUc94gA5Vy3tp2gnBx4YnAQ3msnqDc4sdEUsZsrBV65Q3 1024 1024
QmaRGe7bVmVaLmxbrMiVNXqW4pRNNp3xq7hFtyRKA3mtJL 6    a
QmQSLRRd1Lxn6NMsWmmj2g9W3LtSRfmVAVqU3ShneLUrbn 8    bad\x7fname.txt

QmXQKyr8cT6erZHVqcBBASydmkSF9CnkC8AnTV8UhimExH:
Hash                                           Size Name
QmYGeKXr7Jef8FgFbrb6X2cY45od7anc2QstAJbWT2E5ak 128  128
QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN 6    a
EOF
    test_cmp expected_ls_headers actual_ls_headers
  '

  test_expect_success "'ipfs ls --size=false --cid-base=base32 <three dir hashes>' succeeds" '
    ipfs ls --size=false --cid-base=base32 $(cid-fmt -v 1 -b base32 %s QmYC14BmpHUBMttbWaqbdR81RMcZfSLau6PSazhpRWi1My QmZX7bVCyEuxB6Mzm33yP77PQVuSFvveugDdenzH2yHnd6 QmXQKyr8cT6erZHVqcBBASydmkSF9CnkC8AnTV8UhimExH) >actual_ls_base32
  '

  test_expect_success "'ipfs ls --size=false --cid-base=base32 <three dir hashes>' output looks good" '
    cid-fmt -b base32 -v 1 --filter %s < expected_ls > expected_ls_base32
    test_cmp expected_ls_base32 actual_ls_base32
  '
}


test_ls_cmd_streaming() {

  test_expect_success "'ipfs add -r testData' succeeds" '
    mkdir -p testData testData/d1 testData/d2 &&
    echo "test" >testData/f1 &&
    echo "data" >testData/f2 &&
    echo "hello" >testData/d1/a &&
    random-data -size=128 -seed=42 >testData/d1/128 &&
    echo "world" >testData/d2/a &&
    random-data -size=1024 -seed=42 >testData/d2/1024 &&
    echo "badname" >testData/d2/`echo -e "bad\x7fname.txt"` &&
    ipfs add -r testData >actual_add
  '

  test_expect_success "'ipfs add' output looks good" '
    cat <<-\EOF >expected_add &&
added QmYGeKXr7Jef8FgFbrb6X2cY45od7anc2QstAJbWT2E5ak testData/d1/128
added QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN testData/d1/a
added QmUc94gA5Vy3tp2gnBx4YnAQ3msnqDc4sdEUsZsrBV65Q3 testData/d2/1024
added QmaRGe7bVmVaLmxbrMiVNXqW4pRNNp3xq7hFtyRKA3mtJL testData/d2/a
added QmQSLRRd1Lxn6NMsWmmj2g9W3LtSRfmVAVqU3ShneLUrbn testData/d2/bad\x7fname.txt
added QmeomffUNfmQy76CQGy9NdmqEnnHU9soCexBnGU3ezPHVH testData/f1
added QmNtocSs7MoDkJMc1RkyisCSKvLadujPsfJfSdJ3e1eA1M testData/f2
added QmXQKyr8cT6erZHVqcBBASydmkSF9CnkC8AnTV8UhimExH testData/d1
added QmZX7bVCyEuxB6Mzm33yP77PQVuSFvveugDdenzH2yHnd6 testData/d2
added QmYC14BmpHUBMttbWaqbdR81RMcZfSLau6PSazhpRWi1My testData
EOF
    test_cmp expected_add actual_add
  '

  test_expect_success "'ipfs ls --stream <three dir hashes>' succeeds" '
    ipfs ls --stream QmYC14BmpHUBMttbWaqbdR81RMcZfSLau6PSazhpRWi1My QmZX7bVCyEuxB6Mzm33yP77PQVuSFvveugDdenzH2yHnd6 QmXQKyr8cT6erZHVqcBBASydmkSF9CnkC8AnTV8UhimExH >actual_ls_stream
  '

  test_expect_success "'ipfs ls --stream <three dir hashes>' output looks good" '
    cat <<-\EOF >expected_ls_stream &&
QmYC14BmpHUBMttbWaqbdR81RMcZfSLau6PSazhpRWi1My:
QmXQKyr8cT6erZHVqcBBASydmkSF9CnkC8AnTV8UhimExH -         d1/
QmZX7bVCyEuxB6Mzm33yP77PQVuSFvveugDdenzH2yHnd6 -         d2/
QmeomffUNfmQy76CQGy9NdmqEnnHU9soCexBnGU3ezPHVH 5         f1
QmNtocSs7MoDkJMc1RkyisCSKvLadujPsfJfSdJ3e1eA1M 5         f2

QmZX7bVCyEuxB6Mzm33yP77PQVuSFvveugDdenzH2yHnd6:
QmUc94gA5Vy3tp2gnBx4YnAQ3msnqDc4sdEUsZsrBV65Q3 1024      1024
QmaRGe7bVmVaLmxbrMiVNXqW4pRNNp3xq7hFtyRKA3mtJL 6         a
QmQSLRRd1Lxn6NMsWmmj2g9W3LtSRfmVAVqU3ShneLUrbn 8         bad\x7fname.txt

QmXQKyr8cT6erZHVqcBBASydmkSF9CnkC8AnTV8UhimExH:
QmYGeKXr7Jef8FgFbrb6X2cY45od7anc2QstAJbWT2E5ak 128       128
QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN 6         a
EOF
    test_cmp expected_ls_stream actual_ls_stream
  '

  test_expect_success "'ipfs ls --size=false --stream <three dir hashes>' succeeds" '
    ipfs ls --size=false --stream QmYC14BmpHUBMttbWaqbdR81RMcZfSLau6PSazhpRWi1My QmZX7bVCyEuxB6Mzm33yP77PQVuSFvveugDdenzH2yHnd6 QmXQKyr8cT6erZHVqcBBASydmkSF9CnkC8AnTV8UhimExH >actual_ls_stream
  '

  test_expect_success "'ipfs ls --size=false --stream <three dir hashes>' output looks good" '
    cat <<-\EOF >expected_ls_stream &&
QmYC14BmpHUBMttbWaqbdR81RMcZfSLau6PSazhpRWi1My:
QmXQKyr8cT6erZHVqcBBASydmkSF9CnkC8AnTV8UhimExH d1/
QmZX7bVCyEuxB6Mzm33yP77PQVuSFvveugDdenzH2yHnd6 d2/
QmeomffUNfmQy76CQGy9NdmqEnnHU9soCexBnGU3ezPHVH f1
QmNtocSs7MoDkJMc1RkyisCSKvLadujPsfJfSdJ3e1eA1M f2

QmZX7bVCyEuxB6Mzm33yP77PQVuSFvveugDdenzH2yHnd6:
QmUc94gA5Vy3tp2gnBx4YnAQ3msnqDc4sdEUsZsrBV65Q3 1024
QmaRGe7bVmVaLmxbrMiVNXqW4pRNNp3xq7hFtyRKA3mtJL a
QmQSLRRd1Lxn6NMsWmmj2g9W3LtSRfmVAVqU3ShneLUrbn bad\x7fname.txt

QmXQKyr8cT6erZHVqcBBASydmkSF9CnkC8AnTV8UhimExH:
QmYGeKXr7Jef8FgFbrb6X2cY45od7anc2QstAJbWT2E5ak 128
QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN a
EOF
    test_cmp expected_ls_stream actual_ls_stream
  '

  test_expect_success "'ipfs ls --stream --headers <three dir hashes>' succeeds" '
    ipfs ls --stream --headers QmYC14BmpHUBMttbWaqbdR81RMcZfSLau6PSazhpRWi1My QmZX7bVCyEuxB6Mzm33yP77PQVuSFvveugDdenzH2yHnd6 QmXQKyr8cT6erZHVqcBBASydmkSF9CnkC8AnTV8UhimExH >actual_ls_stream_headers
  '

  test_expect_success "'ipfs ls --stream --headers  <three dir hashes>' output looks good" '
    cat <<-\EOF >expected_ls_stream_headers &&
QmYC14BmpHUBMttbWaqbdR81RMcZfSLau6PSazhpRWi1My:
Hash                                           Size      Name
QmXQKyr8cT6erZHVqcBBASydmkSF9CnkC8AnTV8UhimExH -         d1/
QmZX7bVCyEuxB6Mzm33yP77PQVuSFvveugDdenzH2yHnd6 -         d2/
QmeomffUNfmQy76CQGy9NdmqEnnHU9soCexBnGU3ezPHVH 5         f1
QmNtocSs7MoDkJMc1RkyisCSKvLadujPsfJfSdJ3e1eA1M 5         f2

QmZX7bVCyEuxB6Mzm33yP77PQVuSFvveugDdenzH2yHnd6:
Hash                                           Size      Name
QmUc94gA5Vy3tp2gnBx4YnAQ3msnqDc4sdEUsZsrBV65Q3 1024      1024
QmaRGe7bVmVaLmxbrMiVNXqW4pRNNp3xq7hFtyRKA3mtJL 6         a
QmQSLRRd1Lxn6NMsWmmj2g9W3LtSRfmVAVqU3ShneLUrbn 8         bad\x7fname.txt

QmXQKyr8cT6erZHVqcBBASydmkSF9CnkC8AnTV8UhimExH:
Hash                                           Size      Name
QmYGeKXr7Jef8FgFbrb6X2cY45od7anc2QstAJbWT2E5ak 128       128
QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN 6         a
EOF
    test_cmp expected_ls_stream_headers actual_ls_stream_headers
  '
}

test_ls_cmd_raw_leaves() {
  test_expect_success "'ipfs add -r --raw-leaves' then 'ipfs ls' works as expected" '
    mkdir -p somedir &&
    echo bar > somedir/foo &&
    ipfs add --raw-leaves -r somedir/ > /dev/null &&
    ipfs ls '$1' QmThNTdtKaVoCVrYmM5EBS6U3S5vfKFue2TxbxxAxRcKKE > ls-actual
    echo "bafkreid5qzpjlgzem2iyzgddv7fjilipxcoxzgwazgn27q3usucn5wlxga 4 foo" > ls-expect
    test_cmp ls-actual ls-expect
  '
}

test_ls_object() {
  test_expect_success "ipfs add medium size file then 'ipfs ls --size=false' works as expected" '
    random-data -size=500000 -seed=2 > somefile &&
    HASH=$(ipfs add somefile -q) &&
    echo "QmcNiEfXLxVV1pE3CBvDq4U9Bt23etJxxkeV84ZmLgdpFM " > ls-expect &&
    echo "QmUE5xKYYnTh5mBCu7vq53XyKMJoe5VHN3GfrQFs8NvNYV " >> ls-expect &&
    ipfs ls --size=false $HASH > ls-actual-1 &&
    test_cmp ls-actual-1 ls-expect
  '

  test_expect_success "ipfs add medium size file then 'ipfs ls' works as expected" '
    random-data -size=500000 -seed=2 > somefile &&
    HASH=$(ipfs add somefile -q) &&
    echo "QmcNiEfXLxVV1pE3CBvDq4U9Bt23etJxxkeV84ZmLgdpFM 262144 " > ls-expect &&
    echo "QmUE5xKYYnTh5mBCu7vq53XyKMJoe5VHN3GfrQFs8NvNYV 237856 " >> ls-expect &&
    ipfs ls $HASH > ls-actual-2 &&
    test_cmp ls-actual-2 ls-expect
  '
}

# should work offline
test_ls_cmd
test_ls_cmd_streaming
test_ls_cmd_raw_leaves
test_ls_cmd_raw_leaves --size
test_ls_object

# should work online
test_launch_ipfs_daemon
test_ls_cmd
test_ls_cmd_streaming
test_ls_cmd_raw_leaves
test_ls_cmd_raw_leaves --size
test_kill_ipfs_daemon
test_ls_object

#
# test for ls --resolve-type=false
#

test_expect_success "'ipfs add -r' succeeds" '
  mkdir adir &&
  # note: not using a seed as the files need to have truly random content
  random-data -size=1000 > adir/file1 &&
  random-data -size=1000 > adir/file2 &&
  ipfs add --pin=false -q -r adir > adir-hashes
'

test_expect_success "get hashes from add output" '
  FILE=`head -1 adir-hashes` &&
  DIR=`tail -1 adir-hashes` &&
  test "$FILE" -a "$DIR"
'

test_expect_success "remove a file in dir" '
  ipfs block rm $FILE
'

test_expect_success "'ipfs ls --resolve-type=false ' fails" '
  test_must_fail ipfs ls --resolve-type=false $DIR > /dev/null
'

test_expect_success "'ipfs ls' fails" '
  test_must_fail ipfs ls $DIR
'

test_expect_success "'ipfs ls --resolve-type=true --size=false' fails" '
  test_must_fail ipfs ls --resolve-type=true --size=false $DIR
'

test_launch_ipfs_daemon_without_network

test_expect_success "'ipfs ls --resolve-type=false --size=false' ok" '
  ipfs ls --resolve-type=false --size=false $DIR > /dev/null
'

test_expect_success "'ipfs ls' fails" '
  test_must_fail ipfs ls $DIR
'

test_expect_success "'ipfs ls --resolve-type=false --size=true' fails" '
  test_must_fail ipfs ls --resolve-type=false --size=true $DIR
'

test_kill_ipfs_daemon

test_launch_ipfs_daemon

# now we try `ipfs ls --resolve-type=false` with the daemon online It
# should not even attempt to retrieve the file from the network.  If
# it does it should eventually fail as the content is random and
# should not exist on the network, but we don't want to wait for a
# timeout so we will kill the request after a few seconds
test_expect_success "'ipfs ls --resolve-type=false --size=false' ok and does not hang" '
  go-timeout 2 ipfs ls --resolve-type=false --size=false $DIR
'

test_kill_ipfs_daemon

test_done
