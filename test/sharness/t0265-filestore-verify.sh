#!/bin/sh
#
# Copyright (c) 2016 Kevin Atkinson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test filestore"

. lib/test-filestore-lib.sh
. lib/test-lib.sh

test_init_ipfs

test_enable_filestore

test_verify_cmp() {
    LC_ALL=C sort $1 | grep '[^[:blank:]]' > expect-sorted
    LC_ALL=C sort $2 | grep '[^[:blank:]]' > actual-sorted
    test_cmp expect-sorted actual-sorted
}

#########
#
# Check append
#

test_expect_success "create a file" '
  random 1000000 12 > afile &&
  HASH=$(ipfs filestore add -q --logical afile)
'

test_expect_success "run ipfs verify" '
  ipfs filestore verify > verify-out &&
  fgrep -q "ok       $HASH" verify-out
'

test_expect_success "append to the file" '
  echo "more content" >> afile
'

test_expect_success "test ipfs verify output" '
  ipfs filestore verify > verify-out &&
  fgrep -q "appended $HASH" verify-out
'

test_expect_success "test ipfs verify -l0 output" '
  ipfs filestore verify -l0 > verify-out &&
  fgrep -q "complete $HASH" verify-out
'

filestore_is_empty() {
  ipfs filestore ls -q -a > should-be-empty &&
  test_cmp /dev/null should-be-empty
}

#########
#
# Add a large enough file that the leaf node for the initial part of
# the file has a depth of at least two.  Then, change the contents of
# the initial part and make sure "filestore clean full" is correct.
#

reset_filestore

test_expect_success "generate 200MB file using go-random" '
    random 209715200 41 >mountdir/hugefile
'

test_expect_success "'ipfs add hugefile' succeeds" '
    HASH=$(ipfs filestore add -q --logical mountdir/hugefile)
'

test_expect_success "change first bit of file" '
    dd conv=notrunc if=/dev/zero of=mountdir/hugefile count=1
'

cat <<EOF > verify-expect
changed  QmeomcMd37LRxkYn69XKiTpGEiJWRgUNEaxADx6ssfUJhp
problem  QmRApadtoQSm9Bt3c2vVre7TKoQhh2LDFbaUV3So9yay9a
problem  QmVbVLFLbz72tRSw3HMBh6ABKbRVavMQLoh2BzQ4dUSAYL

EOF

test_expect_success "ipfs verify produces expected output" '
  ipfs filestore verify -q  > verify-actual || true &&
  test_verify_cmp verify-expect verify-actual
'

test_expect_success "'filestore clean full' is complete" '
    ipfs filestore clean full > clean-res &&
    filestore_is_empty
'

test_done

## FIXME NOW: Fix filestore clean and these test so they work

#########
#
# Create a filestore with various problems and then make sure
# "filestore clean" handles them correctly
#

cmp_verify() {
    ipfs filestore verify -q > verify-actual
    test_verify_cmp verify-now verify-actual
}

cat <<EOF > verify-initial
changed  QmWZsU9wAHbaJHgCqFsDPRKomEcKZHei4rGNDrbjzjbmiJ
problem  QmSLmxiETLJXJQxHBHwYd3BckDEqoZ3aZEnVGkb9EmbGcJ

no-file  QmXWr5Td85uXqKhyL17uAsZ7aJZSvtXs3aMGTZ4wHvwubP
problem  QmW6QuzoYEpwASzZktbc5G5Fkq3XeBbUfRCrrUEByYm6Pi

missing  QmQVwjbNQZRpNoeTYwDwtA3CEEXHBeuboPgShqspGn822N
incomplete QmWRhUdTruDSjcdeiFjkGFA6Qd2JXfmVLNMM4KKjJ84jSD

ok       QmaVeSKhGmPYxRyqA236Y4N5e4Rn6LGZKdCgaYUarEo5Nu

ok       QmcAkMdfBPYVzDCM6Fkrz1h8WXcprH8BLF6DmjNUGhXAnm

orphan   QmVBGAJY8ghCXomPmttGs7oUZkQQUAKG3Db5StwneJtxwq
changed  QmPSxQ4mNyq2b1gGu7Crsf3sbdSnYnFB3spSVETSLhD5RW
orphan   QmPSxQ4mNyq2b1gGu7Crsf3sbdSnYnFB3spSVETSLhD5RW
orphan   Qmctvse35eQ8cEgUEsBxJYi7e4uNz3gnUqYYj8JTvZNY2A
orphan   QmWuBmMUbJBjfoG8BgPAdVLuvtk8ysZuMrAYEFk18M9gvR
orphan   QmeoJhPxZ5tQoCXR2aMno63L6kJDbCJ3fZH4gcqjD65aKR
EOF

interesting_prep() {
  test_expect_success "generate a bunch of file with some block sharing" '
    random 1000000 1 > a &&
    random 1000000 2 > b &&
    random 1000000 3 > c &&
    random 1000000 4 > d &&
    random 1000000 5 > e &&
    cat a b > ab &&
    cat b c > bc
  '

  test_expect_success "add files with overlapping blocks" '
    A_HASH=$(ipfs filestore add --logical -q a) &&
    AB_HASH=$(ipfs filestore add --logical -q ab) &&
    BC_HASH=$(ipfs filestore add --logical -q bc) &&
    B_HASH=$(ipfs filestore add --logical -q b) &&
    C_HASH=$(ipfs filestore add --logical -q c) && # note blocks of C not shared due to alignment
    D_HASH=$(ipfs filestore add --logical -q d) &&
    E_HASH=$(ipfs filestore add --logical -q e)
  '

  test_expect_success "create various problems" '
    # removing the backing file for a
    rm a &&
    # remove the root to b
    ipfs filestore rm $B_HASH &&
    # remove a block in c
    ipfs filestore rm QmQVwjbNQZRpNoeTYwDwtA3CEEXHBeuboPgShqspGn822N &&
    # modify d
    dd conv=notrunc if=/dev/zero of=d count=1 &&
    # modify e amd remove the root from the filestore creating a block
    # that is both an orphan and "changed"
    dd conv=notrunc if=/dev/zero of=e count=1 &&
    ipfs filestore rm $E_HASH
  '

  test_expect_success "'filestore verify' produces expected output" '
    cp verify-initial verify-now &&
    cmp_verify
  '
}

interesting_prep

cat <<EOF > verify-now
changed  QmWZsU9wAHbaJHgCqFsDPRKomEcKZHei4rGNDrbjzjbmiJ
problem  QmSLmxiETLJXJQxHBHwYd3BckDEqoZ3aZEnVGkb9EmbGcJ

no-file  QmXWr5Td85uXqKhyL17uAsZ7aJZSvtXs3aMGTZ4wHvwubP
problem  QmW6QuzoYEpwASzZktbc5G5Fkq3XeBbUfRCrrUEByYm6Pi

missing  QmQVwjbNQZRpNoeTYwDwtA3CEEXHBeuboPgShqspGn822N
incomplete QmWRhUdTruDSjcdeiFjkGFA6Qd2JXfmVLNMM4KKjJ84jSD

ok       QmaVeSKhGmPYxRyqA236Y4N5e4Rn6LGZKdCgaYUarEo5Nu

ok       QmcAkMdfBPYVzDCM6Fkrz1h8WXcprH8BLF6DmjNUGhXAnm
EOF
test_expect_success "'filestore clean orphan' (should remove 'changed' orphan)" '
  ipfs filestore clean orphan &&
  cmp_verify
'

cat <<EOF > verify-now
changed  QmWZsU9wAHbaJHgCqFsDPRKomEcKZHei4rGNDrbjzjbmiJ
problem  QmSLmxiETLJXJQxHBHwYd3BckDEqoZ3aZEnVGkb9EmbGcJ

no-file  QmXWr5Td85uXqKhyL17uAsZ7aJZSvtXs3aMGTZ4wHvwubP
problem  QmW6QuzoYEpwASzZktbc5G5Fkq3XeBbUfRCrrUEByYm6Pi

ok       QmaVeSKhGmPYxRyqA236Y4N5e4Rn6LGZKdCgaYUarEo5Nu

ok       QmcAkMdfBPYVzDCM6Fkrz1h8WXcprH8BLF6DmjNUGhXAnm

orphan   QmYswupx1AdGdTn6GeXVdaUBEe6rApd7GWSnobcuVZjeRV
orphan   QmfDSgGhGsEf7LHC6gc7FbBMhGuYzxTLnbKqFBkWhGt8Qp
orphan   QmSWnPbrLFmxfJ9vj2FvKKpVmu3SZprbt7KEbkUVjy7bMD
EOF
test_expect_success "'filestore clean incomplete' (will create more orphans)" '
  ipfs filestore clean incomplete &&
  cmp_verify
'

cat <<EOF > verify-now
changed  QmWZsU9wAHbaJHgCqFsDPRKomEcKZHei4rGNDrbjzjbmiJ
problem  QmSLmxiETLJXJQxHBHwYd3BckDEqoZ3aZEnVGkb9EmbGcJ

no-file  QmXWr5Td85uXqKhyL17uAsZ7aJZSvtXs3aMGTZ4wHvwubP
problem  QmW6QuzoYEpwASzZktbc5G5Fkq3XeBbUfRCrrUEByYm6Pi

ok       QmaVeSKhGmPYxRyqA236Y4N5e4Rn6LGZKdCgaYUarEo5Nu

ok       QmcAkMdfBPYVzDCM6Fkrz1h8WXcprH8BLF6DmjNUGhXAnm
EOF
test_expect_success "'filestore clean orphan'" '
  ipfs filestore clean orphan &&
  cmp_verify
'

cat <<EOF > verify-now
no-file  QmXWr5Td85uXqKhyL17uAsZ7aJZSvtXs3aMGTZ4wHvwubP
problem  QmW6QuzoYEpwASzZktbc5G5Fkq3XeBbUfRCrrUEByYm6Pi

ok       QmaVeSKhGmPYxRyqA236Y4N5e4Rn6LGZKdCgaYUarEo5Nu

ok       QmcAkMdfBPYVzDCM6Fkrz1h8WXcprH8BLF6DmjNUGhXAnm

orphan   QmbZr7Fs6AJf7HpnTxDiYJqLXWDqAy3fKFXYVDkgSsH7DH
orphan   QmToAcacDnpqm17jV7rRHmXcS9686Mk59KCEYGAMkh9qCX
orphan   QmYtLWUVmevucXFN9q59taRT95Gxj5eJuLUhXKtwNna25t
EOF
test_expect_success "'filestore clean changed incomplete' (will create more orphans)" '
  ipfs filestore clean changed incomplete &&
  cmp_verify
'

cat <<EOF > verify-now
missing  QmXWr5Td85uXqKhyL17uAsZ7aJZSvtXs3aMGTZ4wHvwubP
incomplete QmW6QuzoYEpwASzZktbc5G5Fkq3XeBbUfRCrrUEByYm6Pi

ok       QmaVeSKhGmPYxRyqA236Y4N5e4Rn6LGZKdCgaYUarEo5Nu

ok       QmcAkMdfBPYVzDCM6Fkrz1h8WXcprH8BLF6DmjNUGhXAnm

orphan   QmToAcacDnpqm17jV7rRHmXcS9686Mk59KCEYGAMkh9qCX
orphan   QmbZr7Fs6AJf7HpnTxDiYJqLXWDqAy3fKFXYVDkgSsH7DH
orphan   QmYtLWUVmevucXFN9q59taRT95Gxj5eJuLUhXKtwNna25t
EOF
test_expect_success "'filestore clean no-file' (will create an incomplete)" '
  ipfs filestore clean no-file &&
  cmp_verify
'

cat <<EOF > verify-final
ok       QmaVeSKhGmPYxRyqA236Y4N5e4Rn6LGZKdCgaYUarEo5Nu

ok       QmcAkMdfBPYVzDCM6Fkrz1h8WXcprH8BLF6DmjNUGhXAnm
EOF
test_expect_success "'filestore clean incomplete orphan' (cleanup)" '
  cp verify-final verify-now &&
  ipfs filestore clean incomplete orphan &&
  cmp_verify
'

#
# Now reset and redo with a full clean and should get the same results
#

interesting_prep

test_expect_success "'filestore clean full'" '
  cp verify-final verify-now &&
  ipfs filestore clean full &&
  cmp_verify
'

test_expect_success "make sure clean does not remove shared and valid blocks" '
  ipfs cat $AB_HASH > /dev/null
  ipfs cat $BC_HASH > /dev/null
'



test_done
