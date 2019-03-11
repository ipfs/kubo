# IPFS Implementation Status

> Legend: :green_apple: Done &nbsp; :lemon: In Progress &nbsp; :tomato: Missing &nbsp; :chestnut: Not planned

| Command                                      |                   status_test_2019_03_04.txt |                   status_test_2019_03_05.txt |
| -------------------------------------------- | :------------------------------------------: | :------------------------------------------: |
|                                     ipfs add |                     BAD: 5 GOOD: 1 XXX: 232  |                           GOOD: 17 XXX: 221  |
|                       ipfs add --cid-version |                                     XXX: 15  |                                     XXX: 15  |
|                  ipfs add --dereference-args |                                      XXX: 1  |                                      XXX: 1  |
|                           ipfs add --fscache |                                      XXX: 1  |                                      XXX: 1  |
|                              ipfs add --hash |                              GOOD: 1 XXX: 2  |                                      XXX: 3  |
|                            ipfs add --inline |                                      XXX: 3  |                                      XXX: 3  |
|                      ipfs add --inline-limit |                                          ??? |                                          ??? |
|                            ipfs add --nocopy |                                      XXX: 3  |                                      XXX: 3  |
|                               ipfs add --pin |                                     XXX: 26  |                                     XXX: 26  |
|                        ipfs add --raw-leaves |                                     XXX: 32  |                             GOOD: 1 XXX: 31  |
|                            ipfs add --silent |                                          ??? |                                          ??? |
|                        ipfs add --stdin-name |                                      XXX: 3  |                                      XXX: 3  |
|                         ipfs add -H/--hidden |                                      XXX: 1  |                                      XXX: 1  |
|                        ipfs add -Q/--quieter |                                      XXX: 3  |                                      XXX: 3  |
|                      ipfs add -n/--only-hash |                              BAD: 1 XXX: 42  |                             GOOD: 1 XXX: 42  |
|                       ipfs add -p/--progress |                                     XXX: 28  |                                     XXX: 28  |
|                          ipfs add -q/--quiet |                             BAD: 4 XXX: 112  |                            GOOD: 5 XXX: 111  |
|                      ipfs add -r/--recursive |                             BAD: 1 XXX: 102  |                            GOOD: 2 XXX: 101  |
|                        ipfs add -s/--chunker |                                      XXX: 6  |                                      XXX: 6  |
|                        ipfs add -t/--trickle |                                     XXX: 15  |                                     XXX: 15  |
|            ipfs add -w/--wrap-with-directory |                                     XXX: 12  |                                     XXX: 12  |
|                                 ipfs bitswap |                                      XXX: 5  |                                      XXX: 5  |
|                          ipfs bitswap ledger |                                          ??? |                                          ??? |
|                       ipfs bitswap reprovide |                                          ??? |                                          ??? |
|                            ipfs bitswap stat |                                      XXX: 2  |                                      XXX: 2  |
|                        ipfs bitswap wantlist |                                      XXX: 3  |                                      XXX: 3  |
|              ipfs bitswap wantlist -p/--peer |                                      XXX: 2  |                                      XXX: 2  |
|                                   ipfs block |                             GOOD: 1 XXX: 41  |                                     XXX: 42  |
|                               ipfs block get |                                      XXX: 5  |                                      XXX: 5  |
|                               ipfs block put |                             GOOD: 1 XXX: 13  |                                     XXX: 14  |
|                       ipfs block put --mhlen |                              GOOD: 1 XXX: 3  |                                      XXX: 4  |
|                      ipfs block put --mhtype |                                      XXX: 3  |                                      XXX: 3  |
|                   ipfs block put -f/--format |                              GOOD: 1 XXX: 6  |                                      XXX: 7  |
|                                ipfs block rm |                                     XXX: 14  |                                     XXX: 14  |
|                     ipfs block rm -f/--force |                                      XXX: 1  |                                      XXX: 1  |
|                     ipfs block rm -q/--quiet |                                      XXX: 1  |                                      XXX: 1  |
|                              ipfs block stat |                                      XXX: 8  |                                      XXX: 8  |
|                               ipfs bootstrap |                                     XXX: 10  |                              GOOD: 9 XXX: 1  |
|                           ipfs bootstrap add |                                      XXX: 3  |                                     GOOD: 3  |
|                 ipfs bootstrap add --default |                                      XXX: 1  |                                     GOOD: 1  |
|                   ipfs bootstrap add default |                                      XXX: 1  |                                     GOOD: 1  |
|                          ipfs bootstrap list |                                      XXX: 2  |                                     GOOD: 2  |
|                            ipfs bootstrap rm |                                      XXX: 5  |                              GOOD: 4 XXX: 1  |
|                      ipfs bootstrap rm --all |                                      XXX: 3  |                              GOOD: 2 XXX: 1  |
|                        ipfs bootstrap rm all |                                      XXX: 3  |                              GOOD: 2 XXX: 1  |
|                                     ipfs cat |                    BAD: 40 GOOD: 1 XXX: 104  |                            GOOD: 4 XXX: 141  |
|                         ipfs cat -l/--length |                                     XXX: 10  |                                     XXX: 10  |
|                         ipfs cat -o/--offset |                                      XXX: 8  |                                      XXX: 8  |
|                                     ipfs cid |                              BAD: 4 XXX: 56  |                                     XXX: 60  |
|                              ipfs cid base32 |                              BAD: 2 XXX: 31  |                                     XXX: 33  |
|                               ipfs cid bases |                                      XXX: 3  |                                      XXX: 3  |
|                     ipfs cid bases --numeric |                                      XXX: 1  |                                      XXX: 1  |
|                      ipfs cid bases --prefix |                                      XXX: 2  |                                      XXX: 2  |
|                              ipfs cid codecs |                                      XXX: 2  |                                      XXX: 2  |
|                    ipfs cid codecs --numeric |                                      XXX: 1  |                                      XXX: 1  |
|                              ipfs cid format |                                      XXX: 3  |                                      XXX: 3  |
|                           ipfs cid format -b |                                      XXX: 3  |                                      XXX: 3  |
|                           ipfs cid format -f |                                          ??? |                                          ??? |
|                           ipfs cid format -v |                                      XXX: 3  |                                      XXX: 3  |
|                              ipfs cid hashes |                                      XXX: 2  |                                      XXX: 2  |
|                    ipfs cid hashes --numeric |                                      XXX: 1  |                                      XXX: 1  |
|                                ipfs commands |                                      XXX: 2  |                                      XXX: 2  |
|                     ipfs commands -f/--flags |                                      XXX: 2  |                                      XXX: 2  |
|                                  ipfs config |                      BAD: 1 GOOD: 2 XXX: 63  |                             GOOD: 3 XXX: 63  |
|                           ipfs config --bool |                                          ??? |                                          ??? |
|                           ipfs config --json |                              BAD: 1 XXX: 20  |                             GOOD: 3 XXX: 18  |
|                             ipfs config edit |                                          ??? |                                          ??? |
|                          ipfs config profile |                                     XXX: 13  |                                     XXX: 13  |
|                    ipfs config profile apply |                                     XXX: 13  |                                     XXX: 13  |
|          ipfs config profile apply --dry-run |                                      XXX: 4  |                                      XXX: 4  |
|                          ipfs config replace |                                      XXX: 2  |                                      XXX: 2  |
|                             ipfs config show |                                      XXX: 8  |                                      XXX: 8  |
|                                  ipfs daemon |                                      XXX: 7  |                              GOOD: 1 XXX: 6  |
|   ipfs daemon --disable-transport-encryption |                                          ??? |                                          ??? |
|                      ipfs daemon --enable-gc |                                          ??? |                                          ??? |
|        ipfs daemon --enable-mplex-experiment |                                          ??? |                                          ??? |
|          ipfs daemon --enable-namesys-pubsub |                                          ??? |                                          ??? |
|       ipfs daemon --enable-pubsub-experiment |                                          ??? |                                          ??? |
|                           ipfs daemon --init |                                      XXX: 1  |                                     GOOD: 1  |
|                   ipfs daemon --init-profile |                                      XXX: 1  |                                     GOOD: 1  |
|                 ipfs daemon --manage-fdlimit |                                          ??? |                                          ??? |
|                        ipfs daemon --migrate |                                          ??? |                                          ??? |
|                          ipfs daemon --mount |                                          ??? |                                          ??? |
|                     ipfs daemon --mount-ipfs |                                          ??? |                                          ??? |
|                     ipfs daemon --mount-ipns |                                          ??? |                                          ??? |
|                        ipfs daemon --routing |                                      XXX: 2  |                                      XXX: 2  |
|               ipfs daemon --unrestricted-api |                                          ??? |                                          ??? |
|                       ipfs daemon --writable |                                          ??? |                                          ??? |
|                                     ipfs dag |                                     XXX: 40  |                             GOOD: 2 XXX: 38  |
|                                 ipfs dag get |                                     XXX: 11  |                                     XXX: 11  |
|                                 ipfs dag put |                                     XXX: 20  |                             GOOD: 2 XXX: 18  |
|                          ipfs dag put --hash |                                      XXX: 1  |                                      XXX: 1  |
|                     ipfs dag put --input-enc |                                      XXX: 6  |                                      XXX: 6  |
|                           ipfs dag put --pin |                                      XXX: 1  |                                      XXX: 1  |
|                     ipfs dag put -f/--format |                                     XXX: 14  |                                     XXX: 14  |
|                             ipfs dag resolve |                                      XXX: 9  |                                      XXX: 9  |
|                                     ipfs dht |                                          ??? |                                          ??? |
|                            ipfs dht findpeer |                                          ??? |                                          ??? |
|               ipfs dht findpeer -v/--verbose |                                          ??? |                                          ??? |
|                           ipfs dht findprovs |                                          ??? |                                          ??? |
|        ipfs dht findprovs -n/--num-providers |                                          ??? |                                          ??? |
|              ipfs dht findprovs -v/--verbose |                                          ??? |                                          ??? |
|                                 ipfs dht get |                                          ??? |                                          ??? |
|                    ipfs dht get -v/--verbose |                                          ??? |                                          ??? |
|                             ipfs dht provide |                                          ??? |                                          ??? |
|              ipfs dht provide -r/--recursive |                                          ??? |                                          ??? |
|                ipfs dht provide -v/--verbose |                                          ??? |                                          ??? |
|                                 ipfs dht put |                                          ??? |                                          ??? |
|                    ipfs dht put -v/--verbose |                                          ??? |                                          ??? |
|                               ipfs dht query |                                          ??? |                                          ??? |
|                  ipfs dht query -v/--verbose |                                          ??? |                                          ??? |
|                                    ipfs diag |                                      XXX: 4  |                                      XXX: 4  |
|                               ipfs diag cmds |                                      XXX: 3  |                                      XXX: 3  |
|                  ipfs diag cmds -v/--verbose |                                          ??? |                                          ??? |
|                         ipfs diag cmds clear |                                          ??? |                                          ??? |
|                      ipfs diag cmds set-time |                                          ??? |                                          ??? |
|                                ipfs diag sys |                                      XXX: 1  |                                      XXX: 1  |
|                                     ipfs dns |                                      XXX: 1  |                                      XXX: 1  |
|                      ipfs dns -r/--recursive |                                          ??? |                                          ??? |
|                                    ipfs file |                            BAD: 103 XXX: 59  |                                    XXX: 162  |
|                                 ipfs file ls |                                      XXX: 6  |                                      XXX: 6  |
|                                   ipfs files |                            BAD: 102 XXX: 22  |                                    XXX: 124  |
|                        ipfs files -f/--flush |                                      BAD: 9  |                                      XXX: 9  |
|                             ipfs files chcid |                                      BAD: 3  |                                      XXX: 3  |
|                      ipfs files chcid --hash |                                      BAD: 1  |                                      XXX: 1  |
|      ipfs files chcid -cid-ver/--cid-version |                                      BAD: 4  |                                      XXX: 4  |
|                                ipfs files cp |                               BAD: 5 XXX: 3  |                                      XXX: 8  |
|                             ipfs files flush |                                      BAD: 4  |                                      XXX: 4  |
|                                ipfs files ls |                              BAD: 11 XXX: 2  |                                     XXX: 13  |
|                             ipfs files ls -U |                                      BAD: 1  |                                      XXX: 1  |
|                             ipfs files ls -l |                                      BAD: 4  |                                      XXX: 4  |
|                             ipfs files mkdir |                               BAD: 8 XXX: 1  |                                      XXX: 9  |
|                      ipfs files mkdir --hash |                                          ??? |                                          ??? |
|      ipfs files mkdir -cid-ver/--cid-version |                                          ??? |                                          ??? |
|                ipfs files mkdir -p/--parents |                                      BAD: 5  |                                      XXX: 5  |
|                                ipfs files mv |                                      BAD: 1  |                                      XXX: 1  |
|                              ipfs files read |                              BAD: 14 XXX: 1  |                                     XXX: 15  |
|                   ipfs files read -n/--count |                                      BAD: 3  |                                      XXX: 3  |
|                  ipfs files read -o/--offset |                                      BAD: 5  |                                      XXX: 5  |
|                                ipfs files rm |                              BAD: 16 XXX: 1  |                                     XXX: 17  |
|                        ipfs files rm --force |                                      BAD: 2  |                                      XXX: 2  |
|                 ipfs files rm -r/--recursive |                              BAD: 10 XXX: 1  |                                     XXX: 11  |
|                              ipfs files stat |                                     BAD: 32  |                                     XXX: 32  |
|                     ipfs files stat --format |                                      BAD: 2  |                                      XXX: 2  |
|                       ipfs files stat --hash |                                     BAD: 20  |                                     XXX: 20  |
|                       ipfs files stat --size |                                      BAD: 2  |                                      XXX: 2  |
|                 ipfs files stat --with-local |                                      BAD: 1  |                                      XXX: 1  |
|                             ipfs files write |                              BAD: 11 XXX: 3  |                                     XXX: 14  |
|                      ipfs files write --hash |                                          ??? |                                          ??? |
|                ipfs files write --raw-leaves |                                          ??? |                                          ??? |
|      ipfs files write -cid-ver/--cid-version |                                          ??? |                                          ??? |
|                 ipfs files write -e/--create |                               BAD: 8 XXX: 3  |                                     XXX: 11  |
|                  ipfs files write -n/--count |                                          ??? |                                          ??? |
|                 ipfs files write -o/--offset |                                      BAD: 3  |                                      XXX: 3  |
|                ipfs files write -p/--parents |                                          ??? |                                          ??? |
|               ipfs files write -t/--truncate |                                      BAD: 2  |                                      XXX: 2  |
|                               ipfs filestore |                                      XXX: 3  |                                      XXX: 3  |
|                          ipfs filestore dups |                                          ??? |                                          ??? |
|                            ipfs filestore ls |                                      XXX: 1  |                                      XXX: 1  |
|               ipfs filestore ls --file-order |                                          ??? |                                          ??? |
|                        ipfs filestore verify |                                      XXX: 2  |                                      XXX: 2  |
|           ipfs filestore verify --file-order |                                          ??? |                                          ??? |
|                                     ipfs get |                             GOOD: 1 XXX: 44  |                            GOOD: 11 XXX: 34  |
|                       ipfs get -C/--compress |                                      XXX: 1  |                                     GOOD: 1  |
|                        ipfs get -a/--archive |                                      XXX: 1  |                                     GOOD: 1  |
|              ipfs get -l/--compression-level |                                          ??? |                                          ??? |
|                         ipfs get -o/--output |                                      XXX: 8  |                              GOOD: 2 XXX: 6  |
|                                      ipfs id |                                     XXX: 22  |                             GOOD: 13 XXX: 9  |
|                          ipfs id -f/--format |                                     XXX: 25  |                             GOOD: 17 XXX: 8  |
|                                    ipfs init |                             GOOD: 2 XXX: 11  |                             GOOD: 1 XXX: 12  |
|                          ipfs init -b/--bits |                             GOOD: 2 XXX: 14  |                                     XXX: 16  |
|                    ipfs init -e/--empty-repo |                                      XXX: 2  |                                      XXX: 2  |
|                       ipfs init -p/--profile |                                     XXX: 10  |                                     XXX: 10  |
|                                     ipfs key |                                     XXX: 17  |                             GOOD: 2 XXX: 15  |
|                                 ipfs key gen |                                      XXX: 4  |                              GOOD: 1 XXX: 3  |
|                       ipfs key gen -s/--size |                                      XXX: 6  |                              GOOD: 2 XXX: 4  |
|                       ipfs key gen -t/--type |                                      XXX: 8  |                              GOOD: 2 XXX: 6  |
|                                ipfs key list |                                      XXX: 6  |                                      XXX: 6  |
|                             ipfs key list -l |                                      XXX: 3  |                                      XXX: 3  |
|                              ipfs key rename |                                      XXX: 4  |                                      XXX: 4  |
|                   ipfs key rename -f/--force |                                      XXX: 1  |                                      XXX: 1  |
|                                  ipfs key rm |                                      XXX: 2  |                                      XXX: 2  |
|                               ipfs key rm -l |                                          ??? |                                          ??? |
|                                     ipfs log |                                      XXX: 1  |                                      XXX: 1  |
|                               ipfs log level |                                          ??? |                                          ??? |
|                                  ipfs log ls |                                          ??? |                                          ??? |
|                                ipfs log tail |                                      XXX: 1  |                                      XXX: 1  |
|                                      ipfs ls |                             BAD: 12 XXX: 57  |                                     XXX: 69  |
|                       ipfs ls --resolve-type |                                      XXX: 6  |                                      XXX: 6  |
|                               ipfs ls --size |                                     XXX: 10  |                                     XXX: 10  |
|                          ipfs ls -s/--stream |                                     XXX: 15  |                                     XXX: 15  |
|                         ipfs ls -v/--headers |                                      XXX: 3  |                                      XXX: 3  |
|                                   ipfs mount |                                     XXX: 21  |                                     XXX: 21  |
|                    ipfs mount -f/--ipfs-path |                                          ??? |                                          ??? |
|                    ipfs mount -n/--ipns-path |                                          ??? |                                          ??? |
|                                    ipfs name |                                     XXX: 27  |                            GOOD: 15 XXX: 12  |
|                            ipfs name publish |                                     XXX: 11  |                              GOOD: 7 XXX: 4  |
|            ipfs name publish --allow-offline |                                     XXX: 10  |                              GOOD: 6 XXX: 4  |
|                  ipfs name publish --resolve |                                          ??? |                                          ??? |
|                      ipfs name publish --ttl |                                          ??? |                                          ??? |
|               ipfs name publish -Q/--quieter |                                      XXX: 2  |                              GOOD: 1 XXX: 1  |
|                   ipfs name publish -k/--key |                                      XXX: 2  |                                     GOOD: 2  |
|              ipfs name publish -t/--lifetime |                                          ??? |                                          ??? |
|                             ipfs name pubsub |                                          ??? |                                          ??? |
|                      ipfs name pubsub cancel |                                          ??? |                                          ??? |
|                       ipfs name pubsub state |                                          ??? |                                          ??? |
|                        ipfs name pubsub subs |                                          ??? |                                          ??? |
|                            ipfs name resolve |                                     XXX: 11  |                              GOOD: 8 XXX: 3  |
|  ipfs name resolve -dhtrc/--dht-record-count |                                          ??? |                                          ??? |
|        ipfs name resolve -dhtt/--dht-timeout |                                          ??? |                                          ??? |
|               ipfs name resolve -n/--nocache |                                      XXX: 2  |                                     GOOD: 2  |
|             ipfs name resolve -r/--recursive |                                          ??? |                                          ??? |
|                ipfs name resolve -s/--stream |                                          ??? |                                          ??? |
|                                  ipfs object |                                     XXX: 83  |                             GOOD: 1 XXX: 82  |
|             ipfs object add-link -p/--create |                                      XXX: 2  |                                      XXX: 2  |
|                             ipfs object data |                                     XXX: 14  |                                     XXX: 14  |
|                             ipfs object diff |                                     XXX: 16  |                                     XXX: 16  |
|                ipfs object diff -v/--verbose |                                      XXX: 8  |                                      XXX: 8  |
|                              ipfs object get |                                     XXX: 11  |                                     XXX: 11  |
|              ipfs object get --data-encoding |                                      XXX: 3  |                                      XXX: 3  |
|                            ipfs object links |                                      XXX: 4  |                                      XXX: 4  |
|               ipfs object links -v/--headers |                                          ??? |                                          ??? |
|                              ipfs object new |                                      XXX: 7  |                                      XXX: 7  |
|                            ipfs object patch |                                     XXX: 15  |                                     XXX: 15  |
|                   ipfs object patch add-link |                                     XXX: 11  |                                     XXX: 11  |
|                ipfs object patch append-data |                                      XXX: 1  |                                      XXX: 1  |
|                    ipfs object patch rm-link |                                      XXX: 2  |                                      XXX: 2  |
|                   ipfs object patch set-data |                                      XXX: 1  |                                      XXX: 1  |
|                              ipfs object put |                                     XXX: 16  |                             GOOD: 1 XXX: 15  |
|               ipfs object put --datafieldenc |                                          ??? |                                          ??? |
|                   ipfs object put --inputenc |                                      XXX: 6  |                                      XXX: 6  |
|                        ipfs object put --pin |                                      XXX: 1  |                                      XXX: 1  |
|                   ipfs object put -q/--quiet |                                      XXX: 4  |                                      XXX: 4  |
|                             ipfs object stat |                                     XXX: 10  |                                     XXX: 10  |
|                                     ipfs p2p |                                          ??? |                                          ??? |
|                               ipfs p2p close |                                          ??? |                                          ??? |
|                      ipfs p2p close -a/--all |                                          ??? |                                          ??? |
|           ipfs p2p close -l/--listen-address |                                          ??? |                                          ??? |
|                 ipfs p2p close -p/--protocol |                                          ??? |                                          ??? |
|           ipfs p2p close -t/--target-address |                                          ??? |                                          ??? |
|                             ipfs p2p forward |                                          ??? |                                          ??? |
|     ipfs p2p forward --allow-custom-protocol |                                          ??? |                                          ??? |
|                              ipfs p2p listen |                                          ??? |                                          ??? |
|      ipfs p2p listen --allow-custom-protocol |                                          ??? |                                          ??? |
|          ipfs p2p listen -r/--report-peer-id |                                          ??? |                                          ??? |
|                                  ipfs p2p ls |                                          ??? |                                          ??? |
|                     ipfs p2p ls -v/--headers |                                          ??? |                                          ??? |
|                              ipfs p2p stream |                                          ??? |                                          ??? |
|                        ipfs p2p stream close |                                          ??? |                                          ??? |
|                           ipfs p2p stream ls |                                          ??? |                                          ??? |
|                                     ipfs pin |                              BAD: 3 XXX: 90  |                                     XXX: 93  |
|                                 ipfs pin add |                              BAD: 1 XXX: 22  |                                     XXX: 23  |
|                      ipfs pin add --progress |                                      XXX: 1  |                                      XXX: 1  |
|                  ipfs pin add -r/--recursive |                                     XXX: 18  |                                     XXX: 18  |
|                                  ipfs pin ls |                                     XXX: 14  |                                     XXX: 14  |
|                       ipfs pin ls -q/--quiet |                                      XXX: 4  |                                      XXX: 4  |
|                        ipfs pin ls -t/--type |                                     XXX: 16  |                                     XXX: 16  |
|                                  ipfs pin rm |                              BAD: 1 XXX: 23  |                                     XXX: 24  |
|                   ipfs pin rm -r/--recursive |                                      XXX: 9  |                                      XXX: 9  |
|                              ipfs pin update |                                      XXX: 1  |                                      XXX: 1  |
|                      ipfs pin update --unpin |                                      XXX: 1  |                                      XXX: 1  |
|                              ipfs pin verify |                                      XXX: 2  |                                      XXX: 2  |
|                    ipfs pin verify --verbose |                                      XXX: 1  |                                      XXX: 1  |
|                   ipfs pin verify -q/--quiet |                                          ??? |                                          ??? |
|                                    ipfs ping |                                          ??? |                                          ??? |
|                         ipfs ping -n/--count |                                          ??? |                                          ??? |
|                                  ipfs pubsub |                                          ??? |                                          ??? |
|                               ipfs pubsub ls |                                          ??? |                                          ??? |
|                            ipfs pubsub peers |                                          ??? |                                          ??? |
|                              ipfs pubsub pub |                                          ??? |                                          ??? |
|                              ipfs pubsub sub |                                          ??? |                                          ??? |
|                   ipfs pubsub sub --discover |                                          ??? |                                          ??? |
|                                    ipfs refs |                              BAD: 1 XXX: 30  |                                     XXX: 31  |
|                           ipfs refs --format |                                          ??? |                                          ??? |
|                        ipfs refs --max-depth |                                      XXX: 4  |                                      XXX: 4  |
|                         ipfs refs -e/--edges |                                      XXX: 1  |                                      XXX: 1  |
|                     ipfs refs -r/--recursive |                                     XXX: 16  |                                     XXX: 16  |
|                        ipfs refs -u/--unique |                                      XXX: 9  |                                      XXX: 9  |
|                              ipfs refs local |                               BAD: 1 XXX: 9  |                                     XXX: 10  |
|                                    ipfs repo |                              BAD: 1 XXX: 50  |                                     XXX: 51  |
|                               ipfs repo fsck |                                      XXX: 9  |                                      XXX: 9  |
|                                 ipfs repo gc |                              BAD: 1 XXX: 31  |                                     XXX: 32  |
|                 ipfs repo gc --stream-errors |                                      XXX: 1  |                                      XXX: 1  |
|                      ipfs repo gc -q/--quiet |                                          ??? |                                          ??? |
|                               ipfs repo stat |                                      XXX: 6  |                                      XXX: 6  |
|                       ipfs repo stat --human |                                          ??? |                                          ??? |
|                   ipfs repo stat --size-only |                                      XXX: 1  |                                      XXX: 1  |
|                             ipfs repo verify |                                      XXX: 1  |                                      XXX: 1  |
|                            ipfs repo version |                                      XXX: 2  |                                      XXX: 2  |
|                 ipfs repo version -q/--quiet |                                      XXX: 1  |                                      XXX: 1  |
|                                 ipfs resolve |                                     XXX: 30  |                             GOOD: 8 XXX: 22  |
|       ipfs resolve -dhtrc/--dht-record-count |                                          ??? |                                          ??? |
|             ipfs resolve -dhtt/--dht-timeout |                                          ??? |                                          ??? |
|                  ipfs resolve -r/--recursive |                                      XXX: 1  |                                      XXX: 1  |
|                                ipfs shutdown |                                      XXX: 2  |                                      XXX: 2  |
|                                   ipfs stats |                                      XXX: 5  |                                      XXX: 5  |
|                           ipfs stats bitswap |                                          ??? |                                          ??? |
|                                ipfs stats bw |                                          ??? |                                          ??? |
|                         ipfs stats bw --poll |                                          ??? |                                          ??? |
|                  ipfs stats bw -i/--interval |                                          ??? |                                          ??? |
|                      ipfs stats bw -p/--peer |                                          ??? |                                          ??? |
|                     ipfs stats bw -t/--proto |                                          ??? |                                          ??? |
|                              ipfs stats repo |                                          ??? |                                          ??? |
|                      ipfs stats repo --human |                                          ??? |                                          ??? |
|                  ipfs stats repo --size-only |                                          ??? |                                          ??? |
|                                   ipfs swarm |                                     XXX: 16  |                             GOOD: 6 XXX: 10  |
|                             ipfs swarm addrs |                                      XXX: 7  |                              GOOD: 5 XXX: 2  |
|                      ipfs swarm addrs listen |                                      XXX: 1  |                                      XXX: 1  |
|                       ipfs swarm addrs local |                                      XXX: 6  |                              GOOD: 5 XXX: 1  |
|                  ipfs swarm addrs local --id |                                      XXX: 1  |                                     GOOD: 1  |
|                           ipfs swarm connect |                                          ??? |                                          ??? |
|                        ipfs swarm disconnect |                                          ??? |                                          ??? |
|                           ipfs swarm filters |                                      XXX: 6  |                                      XXX: 6  |
|                       ipfs swarm filters add |                                      XXX: 2  |                                      XXX: 2  |
|                        ipfs swarm filters rm |                                      XXX: 3  |                                      XXX: 3  |
|                             ipfs swarm peers |                                      XXX: 3  |                              GOOD: 1 XXX: 2  |
|                 ipfs swarm peers --direction |                                          ??? |                                          ??? |
|                   ipfs swarm peers --latency |                                          ??? |                                          ??? |
|                   ipfs swarm peers --streams |                                          ??? |                                          ??? |
|                ipfs swarm peers -v/--verbose |                                          ??? |                                          ??? |
|                                     ipfs tar |                                      XXX: 5  |                                      XXX: 5  |
|                                 ipfs tar add |                                      XXX: 2  |                                      XXX: 2  |
|                                 ipfs tar cat |                                      XXX: 1  |                                      XXX: 1  |
|                                  ipfs update |                                      XXX: 5  |                              GOOD: 4 XXX: 1  |
|                                ipfs urlstore |                                      XXX: 8  |                                      XXX: 8  |
|                            ipfs urlstore add |                                      XXX: 8  |                                      XXX: 8  |
|                      ipfs urlstore add --pin |                                      XXX: 1  |                                      XXX: 1  |
|               ipfs urlstore add -t/--trickle |                                      XXX: 2  |                                      XXX: 2  |
|                                 ipfs version |                              BAD: 2 XXX: 29  |                                     XXX: 31  |
|                           ipfs version --all |                                      XXX: 2  |                                      XXX: 2  |
|                        ipfs version --commit |                                          ??? |                                          ??? |
|                          ipfs version --repo |                                          ??? |                                          ??? |
|                     ipfs version -n/--number |                                      XXX: 5  |                                      XXX: 5  |
