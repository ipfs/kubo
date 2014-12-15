ipfs bootstrap add /ip4/$BOOTSTRAP_PORT_4011_TCP_ADDR/tcp/$BOOTSTRAP_PORT_4011_TCP_PORT/QmNXuBh8HFsWq68Fid8dMbGNQTh7eG6hV9rr1fQyfmfomE

ipfs daemon &
sleep 3

while [ ! -f /data/id ]
do
    echo waiting for server to add the file...
    sleep 1
done
echo client found file with hash: $(cat /data/id)

ipfs cat $(cat /data/id) > file

cat file

if (($? > 0)); then
    printf '%s\n' 'ipfs cat failed.' >&2
    exit 1
fi

diff -u file /data/file

if (($? > 0)); then
    printf '%s\n' 'files did not match' >&2
    exit 1
fi

echo "success"
