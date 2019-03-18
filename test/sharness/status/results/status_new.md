# IPFS Implementation Status

> Legend: :green_apple: Passed &nbsp; :lemon: Didn't run &nbsp; :tomato: Failed &nbsp; :chestnut: No test implemented

| Command                                      |                   status_test_2019_03_04.txt |                   status_test_2019_03_05.txt |
| -------------------------------------------- | :------------------------------------------: | :------------------------------------------: |
|                                     ipfs add |                     :tomato:: 5 :green_apple:: 1 :lemon:: 232  |                           :green_apple:: 17 :lemon:: 221  |
|                       ipfs add --cid-version |                                     :lemon:: 15  |                                     :lemon:: 15  |
|                  ipfs add --dereference-args |                                      :lemon:: 1  |                                      :lemon:: 1  |
|                           ipfs add --fscache |                                      :lemon:: 1  |                                      :lemon:: 1  |
|                              ipfs add --hash |                              :green_apple:: 1 :lemon:: 2  |                                      :lemon:: 3  |
|                            ipfs add --inline |                                      :lemon:: 3  |                                      :lemon:: 3  |
|                      ipfs add --inline-limit |                                          :chestnut: |                                          :chestnut: |
|                            ipfs add --nocopy |                                      :lemon:: 3  |                                      :lemon:: 3  |
|                               ipfs add --pin |                                     :lemon:: 26  |                                     :lemon:: 26  |
|                        ipfs add --raw-leaves |                                     :lemon:: 32  |                             :green_apple:: 1 :lemon:: 31  |
|                            ipfs add --silent |                                          :chestnut: |                                          :chestnut: |
|                        ipfs add --stdin-name |                                      :lemon:: 3  |                                      :lemon:: 3  |
|                         ipfs add -H/--hidden |                                      :lemon:: 1  |                                      :lemon:: 1  |
|                        ipfs add -Q/--quieter |                                      :lemon:: 3  |                                      :lemon:: 3  |
|                      ipfs add -n/--only-hash |                              :tomato:: 1 :lemon:: 42  |                             :green_apple:: 1 :lemon:: 42  |
|                       ipfs add -p/--progress |                                     :lemon:: 28  |                                     :lemon:: 28  |
|                          ipfs add -q/--quiet |                             :tomato:: 4 :lemon:: 112  |                            :green_apple:: 5 :lemon:: 111  |
|                      ipfs add -r/--recursive |                             :tomato:: 1 :lemon:: 102  |                            :green_apple:: 2 :lemon:: 101  |
|                        ipfs add -s/--chunker |                                      :lemon:: 6  |                                      :lemon:: 6  |
|                        ipfs add -t/--trickle |                                     :lemon:: 15  |                                     :lemon:: 15  |
|            ipfs add -w/--wrap-with-directory |                                     :lemon:: 12  |                                     :lemon:: 12  |
|                                 ipfs bitswap |                                      :lemon:: 5  |                                      :lemon:: 5  |
|                          ipfs bitswap ledger |                                          :chestnut: |                                          :chestnut: |
|                       ipfs bitswap reprovide |                                          :chestnut: |                                          :chestnut: |
|                            ipfs bitswap stat |                                      :lemon:: 2  |                                      :lemon:: 2  |
|                        ipfs bitswap wantlist |                                      :lemon:: 3  |                                      :lemon:: 3  |
|              ipfs bitswap wantlist -p/--peer |                                      :lemon:: 2  |                                      :lemon:: 2  |
|                                   ipfs block |                             :green_apple:: 1 :lemon:: 41  |                                     :lemon:: 42  |
|                               ipfs block get |                                      :lemon:: 5  |                                      :lemon:: 5  |
|                               ipfs block put |                             :green_apple:: 1 :lemon:: 13  |                                     :lemon:: 14  |
|                       ipfs block put --mhlen |                              :green_apple:: 1 :lemon:: 3  |                                      :lemon:: 4  |
|                      ipfs block put --mhtype |                                      :lemon:: 3  |                                      :lemon:: 3  |
|                   ipfs block put -f/--format |                              :green_apple:: 1 :lemon:: 6  |                                      :lemon:: 7  |
|                                ipfs block rm |                                     :lemon:: 14  |                                     :lemon:: 14  |
|                     ipfs block rm -f/--force |                                      :lemon:: 1  |                                      :lemon:: 1  |
|                     ipfs block rm -q/--quiet |                                      :lemon:: 1  |                                      :lemon:: 1  |
|                              ipfs block stat |                                      :lemon:: 8  |                                      :lemon:: 8  |
|                               ipfs bootstrap |                                     :lemon:: 10  |                              :green_apple:: 9 :lemon:: 1  |
|                           ipfs bootstrap add |                                      :lemon:: 3  |                                     :green_apple:: 3  |
|                 ipfs bootstrap add --default |                                      :lemon:: 1  |                                     :green_apple:: 1  |
|                   ipfs bootstrap add default |                                      :lemon:: 1  |                                     :green_apple:: 1  |
|                          ipfs bootstrap list |                                      :lemon:: 2  |                                     :green_apple:: 2  |
|                            ipfs bootstrap rm |                                      :lemon:: 5  |                              :green_apple:: 4 :lemon:: 1  |
|                      ipfs bootstrap rm --all |                                      :lemon:: 3  |                              :green_apple:: 2 :lemon:: 1  |
|                        ipfs bootstrap rm all |                                      :lemon:: 3  |                              :green_apple:: 2 :lemon:: 1  |
|                                     ipfs cat |                    :tomato:: 40 :green_apple:: 1 :lemon:: 104  |                            :green_apple:: 4 :lemon:: 141  |
|                         ipfs cat -l/--length |                                     :lemon:: 10  |                                     :lemon:: 10  |
|                         ipfs cat -o/--offset |                                      :lemon:: 8  |                                      :lemon:: 8  |
|                                     ipfs cid |                              :tomato:: 4 :lemon:: 56  |                                     :lemon:: 60  |
|                              ipfs cid base32 |                              :tomato:: 2 :lemon:: 31  |                                     :lemon:: 33  |
|                               ipfs cid bases |                                      :lemon:: 3  |                                      :lemon:: 3  |
|                     ipfs cid bases --numeric |                                      :lemon:: 1  |                                      :lemon:: 1  |
|                      ipfs cid bases --prefix |                                      :lemon:: 2  |                                      :lemon:: 2  |
|                              ipfs cid codecs |                                      :lemon:: 2  |                                      :lemon:: 2  |
|                    ipfs cid codecs --numeric |                                      :lemon:: 1  |                                      :lemon:: 1  |
|                              ipfs cid format |                                      :lemon:: 3  |                                      :lemon:: 3  |
|                           ipfs cid format -b |                                      :lemon:: 3  |                                      :lemon:: 3  |
|                           ipfs cid format -f |                                          :chestnut: |                                          :chestnut: |
|                           ipfs cid format -v |                                      :lemon:: 3  |                                      :lemon:: 3  |
|                              ipfs cid hashes |                                      :lemon:: 2  |                                      :lemon:: 2  |
|                    ipfs cid hashes --numeric |                                      :lemon:: 1  |                                      :lemon:: 1  |
|                                ipfs commands |                                      :lemon:: 2  |                                      :lemon:: 2  |
|                     ipfs commands -f/--flags |                                      :lemon:: 2  |                                      :lemon:: 2  |
|                                  ipfs config |                      :tomato:: 1 :green_apple:: 2 :lemon:: 63  |                             :green_apple:: 3 :lemon:: 63  |
|                           ipfs config --bool |                                          :chestnut: |                                          :chestnut: |
|                           ipfs config --json |                              :tomato:: 1 :lemon:: 20  |                             :green_apple:: 3 :lemon:: 18  |
|                             ipfs config edit |                                          :chestnut: |                                          :chestnut: |
|                          ipfs config profile |                                     :lemon:: 13  |                                     :lemon:: 13  |
|                    ipfs config profile apply |                                     :lemon:: 13  |                                     :lemon:: 13  |
|          ipfs config profile apply --dry-run |                                      :lemon:: 4  |                                      :lemon:: 4  |
|                          ipfs config replace |                                      :lemon:: 2  |                                      :lemon:: 2  |
|                             ipfs config show |                                      :lemon:: 8  |                                      :lemon:: 8  |
|                                  ipfs daemon |                                      :lemon:: 7  |                              :green_apple:: 1 :lemon:: 6  |
|   ipfs daemon --disable-transport-encryption |                                          :chestnut: |                                          :chestnut: |
|                      ipfs daemon --enable-gc |                                          :chestnut: |                                          :chestnut: |
|        ipfs daemon --enable-mplex-experiment |                                          :chestnut: |                                          :chestnut: |
|          ipfs daemon --enable-namesys-pubsub |                                          :chestnut: |                                          :chestnut: |
|       ipfs daemon --enable-pubsub-experiment |                                          :chestnut: |                                          :chestnut: |
|                           ipfs daemon --init |                                      :lemon:: 1  |                                     :green_apple:: 1  |
|                   ipfs daemon --init-profile |                                      :lemon:: 1  |                                     :green_apple:: 1  |
|                 ipfs daemon --manage-fdlimit |                                          :chestnut: |                                          :chestnut: |
|                        ipfs daemon --migrate |                                          :chestnut: |                                          :chestnut: |
|                          ipfs daemon --mount |                                          :chestnut: |                                          :chestnut: |
|                     ipfs daemon --mount-ipfs |                                          :chestnut: |                                          :chestnut: |
|                     ipfs daemon --mount-ipns |                                          :chestnut: |                                          :chestnut: |
|                        ipfs daemon --routing |                                      :lemon:: 2  |                                      :lemon:: 2  |
|               ipfs daemon --unrestricted-api |                                          :chestnut: |                                          :chestnut: |
|                       ipfs daemon --writable |                                          :chestnut: |                                          :chestnut: |
|                                     ipfs dag |                                     :lemon:: 40  |                             :green_apple:: 2 :lemon:: 38  |
|                                 ipfs dag get |                                     :lemon:: 11  |                                     :lemon:: 11  |
|                                 ipfs dag put |                                     :lemon:: 20  |                             :green_apple:: 2 :lemon:: 18  |
|                          ipfs dag put --hash |                                      :lemon:: 1  |                                      :lemon:: 1  |
|                     ipfs dag put --input-enc |                                      :lemon:: 6  |                                      :lemon:: 6  |
|                           ipfs dag put --pin |                                      :lemon:: 1  |                                      :lemon:: 1  |
|                     ipfs dag put -f/--format |                                     :lemon:: 14  |                                     :lemon:: 14  |
|                             ipfs dag resolve |                                      :lemon:: 9  |                                      :lemon:: 9  |
|                                     ipfs dht |                                          :chestnut: |                                          :chestnut: |
|                            ipfs dht findpeer |                                          :chestnut: |                                          :chestnut: |
|               ipfs dht findpeer -v/--verbose |                                          :chestnut: |                                          :chestnut: |
|                           ipfs dht findprovs |                                          :chestnut: |                                          :chestnut: |
|        ipfs dht findprovs -n/--num-providers |                                          :chestnut: |                                          :chestnut: |
|              ipfs dht findprovs -v/--verbose |                                          :chestnut: |                                          :chestnut: |
|                                 ipfs dht get |                                          :chestnut: |                                          :chestnut: |
|                    ipfs dht get -v/--verbose |                                          :chestnut: |                                          :chestnut: |
|                             ipfs dht provide |                                          :chestnut: |                                          :chestnut: |
|              ipfs dht provide -r/--recursive |                                          :chestnut: |                                          :chestnut: |
|                ipfs dht provide -v/--verbose |                                          :chestnut: |                                          :chestnut: |
|                                 ipfs dht put |                                          :chestnut: |                                          :chestnut: |
|                    ipfs dht put -v/--verbose |                                          :chestnut: |                                          :chestnut: |
|                               ipfs dht query |                                          :chestnut: |                                          :chestnut: |
|                  ipfs dht query -v/--verbose |                                          :chestnut: |                                          :chestnut: |
|                                    ipfs diag |                                      :lemon:: 4  |                                      :lemon:: 4  |
|                               ipfs diag cmds |                                      :lemon:: 3  |                                      :lemon:: 3  |
|                  ipfs diag cmds -v/--verbose |                                          :chestnut: |                                          :chestnut: |
|                         ipfs diag cmds clear |                                          :chestnut: |                                          :chestnut: |
|                      ipfs diag cmds set-time |                                          :chestnut: |                                          :chestnut: |
|                                ipfs diag sys |                                      :lemon:: 1  |                                      :lemon:: 1  |
|                                     ipfs dns |                                      :lemon:: 1  |                                      :lemon:: 1  |
|                      ipfs dns -r/--recursive |                                          :chestnut: |                                          :chestnut: |
|                                    ipfs file |                            :tomato:: 103 :lemon:: 59  |                                    :lemon:: 162  |
|                                 ipfs file ls |                                      :lemon:: 6  |                                      :lemon:: 6  |
|                                   ipfs files |                            :tomato:: 102 :lemon:: 22  |                                    :lemon:: 124  |
|                        ipfs files -f/--flush |                                      :tomato:: 9  |                                      :lemon:: 9  |
|                             ipfs files chcid |                                      :tomato:: 3  |                                      :lemon:: 3  |
|                      ipfs files chcid --hash |                                      :tomato:: 1  |                                      :lemon:: 1  |
|      ipfs files chcid -cid-ver/--cid-version |                                      :tomato:: 4  |                                      :lemon:: 4  |
|                                ipfs files cp |                               :tomato:: 5 :lemon:: 3  |                                      :lemon:: 8  |
|                             ipfs files flush |                                      :tomato:: 4  |                                      :lemon:: 4  |
|                                ipfs files ls |                              :tomato:: 11 :lemon:: 2  |                                     :lemon:: 13  |
|                             ipfs files ls -U |                                      :tomato:: 1  |                                      :lemon:: 1  |
|                             ipfs files ls -l |                                      :tomato:: 4  |                                      :lemon:: 4  |
|                             ipfs files mkdir |                               :tomato:: 8 :lemon:: 1  |                                      :lemon:: 9  |
|                      ipfs files mkdir --hash |                                          :chestnut: |                                          :chestnut: |
|      ipfs files mkdir -cid-ver/--cid-version |                                          :chestnut: |                                          :chestnut: |
|                ipfs files mkdir -p/--parents |                                      :tomato:: 5  |                                      :lemon:: 5  |
|                                ipfs files mv |                                      :tomato:: 1  |                                      :lemon:: 1  |
|                              ipfs files read |                              :tomato:: 14 :lemon:: 1  |                                     :lemon:: 15  |
|                   ipfs files read -n/--count |                                      :tomato:: 3  |                                      :lemon:: 3  |
|                  ipfs files read -o/--offset |                                      :tomato:: 5  |                                      :lemon:: 5  |
|                                ipfs files rm |                              :tomato:: 16 :lemon:: 1  |                                     :lemon:: 17  |
|                        ipfs files rm --force |                                      :tomato:: 2  |                                      :lemon:: 2  |
|                 ipfs files rm -r/--recursive |                              :tomato:: 10 :lemon:: 1  |                                     :lemon:: 11  |
|                              ipfs files stat |                                     :tomato:: 32  |                                     :lemon:: 32  |
|                     ipfs files stat --format |                                      :tomato:: 2  |                                      :lemon:: 2  |
|                       ipfs files stat --hash |                                     :tomato:: 20  |                                     :lemon:: 20  |
|                       ipfs files stat --size |                                      :tomato:: 2  |                                      :lemon:: 2  |
|                 ipfs files stat --with-local |                                      :tomato:: 1  |                                      :lemon:: 1  |
|                             ipfs files write |                              :tomato:: 11 :lemon:: 3  |                                     :lemon:: 14  |
|                      ipfs files write --hash |                                          :chestnut: |                                          :chestnut: |
|                ipfs files write --raw-leaves |                                          :chestnut: |                                          :chestnut: |
|      ipfs files write -cid-ver/--cid-version |                                          :chestnut: |                                          :chestnut: |
|                 ipfs files write -e/--create |                               :tomato:: 8 :lemon:: 3  |                                     :lemon:: 11  |
|                  ipfs files write -n/--count |                                          :chestnut: |                                          :chestnut: |
|                 ipfs files write -o/--offset |                                      :tomato:: 3  |                                      :lemon:: 3  |
|                ipfs files write -p/--parents |                                          :chestnut: |                                          :chestnut: |
|               ipfs files write -t/--truncate |                                      :tomato:: 2  |                                      :lemon:: 2  |
|                               ipfs filestore |                                      :lemon:: 3  |                                      :lemon:: 3  |
|                          ipfs filestore dups |                                          :chestnut: |                                          :chestnut: |
|                            ipfs filestore ls |                                      :lemon:: 1  |                                      :lemon:: 1  |
|               ipfs filestore ls --file-order |                                          :chestnut: |                                          :chestnut: |
|                        ipfs filestore verify |                                      :lemon:: 2  |                                      :lemon:: 2  |
|           ipfs filestore verify --file-order |                                          :chestnut: |                                          :chestnut: |
|                                     ipfs get |                             :green_apple:: 1 :lemon:: 44  |                            :green_apple:: 11 :lemon:: 34  |
|                       ipfs get -C/--compress |                                      :lemon:: 1  |                                     :green_apple:: 1  |
|                        ipfs get -a/--archive |                                      :lemon:: 1  |                                     :green_apple:: 1  |
|              ipfs get -l/--compression-level |                                          :chestnut: |                                          :chestnut: |
|                         ipfs get -o/--output |                                      :lemon:: 8  |                              :green_apple:: 2 :lemon:: 6  |
|                                      ipfs id |                                     :lemon:: 22  |                             :green_apple:: 13 :lemon:: 9  |
|                          ipfs id -f/--format |                                     :lemon:: 25  |                             :green_apple:: 17 :lemon:: 8  |
|                                    ipfs init |                             :green_apple:: 2 :lemon:: 11  |                             :green_apple:: 1 :lemon:: 12  |
|                          ipfs init -b/--bits |                             :green_apple:: 2 :lemon:: 14  |                                     :lemon:: 16  |
|                    ipfs init -e/--empty-repo |                                      :lemon:: 2  |                                      :lemon:: 2  |
|                       ipfs init -p/--profile |                                     :lemon:: 10  |                                     :lemon:: 10  |
|                                     ipfs key |                                     :lemon:: 17  |                             :green_apple:: 2 :lemon:: 15  |
|                                 ipfs key gen |                                      :lemon:: 4  |                              :green_apple:: 1 :lemon:: 3  |
|                       ipfs key gen -s/--size |                                      :lemon:: 6  |                              :green_apple:: 2 :lemon:: 4  |
|                       ipfs key gen -t/--type |                                      :lemon:: 8  |                              :green_apple:: 2 :lemon:: 6  |
|                                ipfs key list |                                      :lemon:: 6  |                                      :lemon:: 6  |
|                             ipfs key list -l |                                      :lemon:: 3  |                                      :lemon:: 3  |
|                              ipfs key rename |                                      :lemon:: 4  |                                      :lemon:: 4  |
|                   ipfs key rename -f/--force |                                      :lemon:: 1  |                                      :lemon:: 1  |
|                                  ipfs key rm |                                      :lemon:: 2  |                                      :lemon:: 2  |
|                               ipfs key rm -l |                                          :chestnut: |                                          :chestnut: |
|                                     ipfs log |                                      :lemon:: 1  |                                      :lemon:: 1  |
|                               ipfs log level |                                          :chestnut: |                                          :chestnut: |
|                                  ipfs log ls |                                          :chestnut: |                                          :chestnut: |
|                                ipfs log tail |                                      :lemon:: 1  |                                      :lemon:: 1  |
|                                      ipfs ls |                             :tomato:: 12 :lemon:: 57  |                                     :lemon:: 69  |
|                       ipfs ls --resolve-type |                                      :lemon:: 6  |                                      :lemon:: 6  |
|                               ipfs ls --size |                                     :lemon:: 10  |                                     :lemon:: 10  |
|                          ipfs ls -s/--stream |                                     :lemon:: 15  |                                     :lemon:: 15  |
|                         ipfs ls -v/--headers |                                      :lemon:: 3  |                                      :lemon:: 3  |
|                                   ipfs mount |                                     :lemon:: 21  |                                     :lemon:: 21  |
|                    ipfs mount -f/--ipfs-path |                                          :chestnut: |                                          :chestnut: |
|                    ipfs mount -n/--ipns-path |                                          :chestnut: |                                          :chestnut: |
|                                    ipfs name |                                     :lemon:: 27  |                            :green_apple:: 15 :lemon:: 12  |
|                            ipfs name publish |                                     :lemon:: 11  |                              :green_apple:: 7 :lemon:: 4  |
|            ipfs name publish --allow-offline |                                     :lemon:: 10  |                              :green_apple:: 6 :lemon:: 4  |
|                  ipfs name publish --resolve |                                          :chestnut: |                                          :chestnut: |
|                      ipfs name publish --ttl |                                          :chestnut: |                                          :chestnut: |
|               ipfs name publish -Q/--quieter |                                      :lemon:: 2  |                              :green_apple:: 1 :lemon:: 1  |
|                   ipfs name publish -k/--key |                                      :lemon:: 2  |                                     :green_apple:: 2  |
|              ipfs name publish -t/--lifetime |                                          :chestnut: |                                          :chestnut: |
|                             ipfs name pubsub |                                          :chestnut: |                                          :chestnut: |
|                      ipfs name pubsub cancel |                                          :chestnut: |                                          :chestnut: |
|                       ipfs name pubsub state |                                          :chestnut: |                                          :chestnut: |
|                        ipfs name pubsub subs |                                          :chestnut: |                                          :chestnut: |
|                            ipfs name resolve |                                     :lemon:: 11  |                              :green_apple:: 8 :lemon:: 3  |
|  ipfs name resolve -dhtrc/--dht-record-count |                                          :chestnut: |                                          :chestnut: |
|        ipfs name resolve -dhtt/--dht-timeout |                                          :chestnut: |                                          :chestnut: |
|               ipfs name resolve -n/--nocache |                                      :lemon:: 2  |                                     :green_apple:: 2  |
|             ipfs name resolve -r/--recursive |                                          :chestnut: |                                          :chestnut: |
|                ipfs name resolve -s/--stream |                                          :chestnut: |                                          :chestnut: |
|                                  ipfs object |                                     :lemon:: 83  |                             :green_apple:: 1 :lemon:: 82  |
|             ipfs object add-link -p/--create |                                      :lemon:: 2  |                                      :lemon:: 2  |
|                             ipfs object data |                                     :lemon:: 14  |                                     :lemon:: 14  |
|                             ipfs object diff |                                     :lemon:: 16  |                                     :lemon:: 16  |
|                ipfs object diff -v/--verbose |                                      :lemon:: 8  |                                      :lemon:: 8  |
|                              ipfs object get |                                     :lemon:: 11  |                                     :lemon:: 11  |
|              ipfs object get --data-encoding |                                      :lemon:: 3  |                                      :lemon:: 3  |
|                            ipfs object links |                                      :lemon:: 4  |                                      :lemon:: 4  |
|               ipfs object links -v/--headers |                                          :chestnut: |                                          :chestnut: |
|                              ipfs object new |                                      :lemon:: 7  |                                      :lemon:: 7  |
|                            ipfs object patch |                                     :lemon:: 15  |                                     :lemon:: 15  |
|                   ipfs object patch add-link |                                     :lemon:: 11  |                                     :lemon:: 11  |
|                ipfs object patch append-data |                                      :lemon:: 1  |                                      :lemon:: 1  |
|                    ipfs object patch rm-link |                                      :lemon:: 2  |                                      :lemon:: 2  |
|                   ipfs object patch set-data |                                      :lemon:: 1  |                                      :lemon:: 1  |
|                              ipfs object put |                                     :lemon:: 16  |                             :green_apple:: 1 :lemon:: 15  |
|               ipfs object put --datafieldenc |                                          :chestnut: |                                          :chestnut: |
|                   ipfs object put --inputenc |                                      :lemon:: 6  |                                      :lemon:: 6  |
|                        ipfs object put --pin |                                      :lemon:: 1  |                                      :lemon:: 1  |
|                   ipfs object put -q/--quiet |                                      :lemon:: 4  |                                      :lemon:: 4  |
|                             ipfs object stat |                                     :lemon:: 10  |                                     :lemon:: 10  |
|                                     ipfs p2p |                                          :chestnut: |                                          :chestnut: |
|                               ipfs p2p close |                                          :chestnut: |                                          :chestnut: |
|                      ipfs p2p close -a/--all |                                          :chestnut: |                                          :chestnut: |
|           ipfs p2p close -l/--listen-address |                                          :chestnut: |                                          :chestnut: |
|                 ipfs p2p close -p/--protocol |                                          :chestnut: |                                          :chestnut: |
|           ipfs p2p close -t/--target-address |                                          :chestnut: |                                          :chestnut: |
|                             ipfs p2p forward |                                          :chestnut: |                                          :chestnut: |
|     ipfs p2p forward --allow-custom-protocol |                                          :chestnut: |                                          :chestnut: |
|                              ipfs p2p listen |                                          :chestnut: |                                          :chestnut: |
|      ipfs p2p listen --allow-custom-protocol |                                          :chestnut: |                                          :chestnut: |
|          ipfs p2p listen -r/--report-peer-id |                                          :chestnut: |                                          :chestnut: |
|                                  ipfs p2p ls |                                          :chestnut: |                                          :chestnut: |
|                     ipfs p2p ls -v/--headers |                                          :chestnut: |                                          :chestnut: |
|                              ipfs p2p stream |                                          :chestnut: |                                          :chestnut: |
|                        ipfs p2p stream close |                                          :chestnut: |                                          :chestnut: |
|                           ipfs p2p stream ls |                                          :chestnut: |                                          :chestnut: |
|                                     ipfs pin |                              :tomato:: 3 :lemon:: 90  |                                     :lemon:: 93  |
|                                 ipfs pin add |                              :tomato:: 1 :lemon:: 22  |                                     :lemon:: 23  |
|                      ipfs pin add --progress |                                      :lemon:: 1  |                                      :lemon:: 1  |
|                  ipfs pin add -r/--recursive |                                     :lemon:: 18  |                                     :lemon:: 18  |
|                                  ipfs pin ls |                                     :lemon:: 14  |                                     :lemon:: 14  |
|                       ipfs pin ls -q/--quiet |                                      :lemon:: 4  |                                      :lemon:: 4  |
|                        ipfs pin ls -t/--type |                                     :lemon:: 16  |                                     :lemon:: 16  |
|                                  ipfs pin rm |                              :tomato:: 1 :lemon:: 23  |                                     :lemon:: 24  |
|                   ipfs pin rm -r/--recursive |                                      :lemon:: 9  |                                      :lemon:: 9  |
|                              ipfs pin update |                                      :lemon:: 1  |                                      :lemon:: 1  |
|                      ipfs pin update --unpin |                                      :lemon:: 1  |                                      :lemon:: 1  |
|                              ipfs pin verify |                                      :lemon:: 2  |                                      :lemon:: 2  |
|                    ipfs pin verify --verbose |                                      :lemon:: 1  |                                      :lemon:: 1  |
|                   ipfs pin verify -q/--quiet |                                          :chestnut: |                                          :chestnut: |
|                                    ipfs ping |                                          :chestnut: |                                          :chestnut: |
|                         ipfs ping -n/--count |                                          :chestnut: |                                          :chestnut: |
|                                  ipfs pubsub |                                          :chestnut: |                                          :chestnut: |
|                               ipfs pubsub ls |                                          :chestnut: |                                          :chestnut: |
|                            ipfs pubsub peers |                                          :chestnut: |                                          :chestnut: |
|                              ipfs pubsub pub |                                          :chestnut: |                                          :chestnut: |
|                              ipfs pubsub sub |                                          :chestnut: |                                          :chestnut: |
|                   ipfs pubsub sub --discover |                                          :chestnut: |                                          :chestnut: |
|                                    ipfs refs |                              :tomato:: 1 :lemon:: 30  |                                     :lemon:: 31  |
|                           ipfs refs --format |                                          :chestnut: |                                          :chestnut: |
|                        ipfs refs --max-depth |                                      :lemon:: 4  |                                      :lemon:: 4  |
|                         ipfs refs -e/--edges |                                      :lemon:: 1  |                                      :lemon:: 1  |
|                     ipfs refs -r/--recursive |                                     :lemon:: 16  |                                     :lemon:: 16  |
|                        ipfs refs -u/--unique |                                      :lemon:: 9  |                                      :lemon:: 9  |
|                              ipfs refs local |                               :tomato:: 1 :lemon:: 9  |                                     :lemon:: 10  |
|                                    ipfs repo |                              :tomato:: 1 :lemon:: 50  |                                     :lemon:: 51  |
|                               ipfs repo fsck |                                      :lemon:: 9  |                                      :lemon:: 9  |
|                                 ipfs repo gc |                              :tomato:: 1 :lemon:: 31  |                                     :lemon:: 32  |
|                 ipfs repo gc --stream-errors |                                      :lemon:: 1  |                                      :lemon:: 1  |
|                      ipfs repo gc -q/--quiet |                                          :chestnut: |                                          :chestnut: |
|                               ipfs repo stat |                                      :lemon:: 6  |                                      :lemon:: 6  |
|                       ipfs repo stat --human |                                          :chestnut: |                                          :chestnut: |
|                   ipfs repo stat --size-only |                                      :lemon:: 1  |                                      :lemon:: 1  |
|                             ipfs repo verify |                                      :lemon:: 1  |                                      :lemon:: 1  |
|                            ipfs repo version |                                      :lemon:: 2  |                                      :lemon:: 2  |
|                 ipfs repo version -q/--quiet |                                      :lemon:: 1  |                                      :lemon:: 1  |
|                                 ipfs resolve |                                     :lemon:: 30  |                             :green_apple:: 8 :lemon:: 22  |
|       ipfs resolve -dhtrc/--dht-record-count |                                          :chestnut: |                                          :chestnut: |
|             ipfs resolve -dhtt/--dht-timeout |                                          :chestnut: |                                          :chestnut: |
|                  ipfs resolve -r/--recursive |                                      :lemon:: 1  |                                      :lemon:: 1  |
|                                ipfs shutdown |                                      :lemon:: 2  |                                      :lemon:: 2  |
|                                   ipfs stats |                                      :lemon:: 5  |                                      :lemon:: 5  |
|                           ipfs stats bitswap |                                          :chestnut: |                                          :chestnut: |
|                                ipfs stats bw |                                          :chestnut: |                                          :chestnut: |
|                         ipfs stats bw --poll |                                          :chestnut: |                                          :chestnut: |
|                  ipfs stats bw -i/--interval |                                          :chestnut: |                                          :chestnut: |
|                      ipfs stats bw -p/--peer |                                          :chestnut: |                                          :chestnut: |
|                     ipfs stats bw -t/--proto |                                          :chestnut: |                                          :chestnut: |
|                              ipfs stats repo |                                          :chestnut: |                                          :chestnut: |
|                      ipfs stats repo --human |                                          :chestnut: |                                          :chestnut: |
|                  ipfs stats repo --size-only |                                          :chestnut: |                                          :chestnut: |
|                                   ipfs swarm |                                     :lemon:: 16  |                             :green_apple:: 6 :lemon:: 10  |
|                             ipfs swarm addrs |                                      :lemon:: 7  |                              :green_apple:: 5 :lemon:: 2  |
|                      ipfs swarm addrs listen |                                      :lemon:: 1  |                                      :lemon:: 1  |
|                       ipfs swarm addrs local |                                      :lemon:: 6  |                              :green_apple:: 5 :lemon:: 1  |
|                  ipfs swarm addrs local --id |                                      :lemon:: 1  |                                     :green_apple:: 1  |
|                           ipfs swarm connect |                                          :chestnut: |                                          :chestnut: |
|                        ipfs swarm disconnect |                                          :chestnut: |                                          :chestnut: |
|                           ipfs swarm filters |                                      :lemon:: 6  |                                      :lemon:: 6  |
|                       ipfs swarm filters add |                                      :lemon:: 2  |                                      :lemon:: 2  |
|                        ipfs swarm filters rm |                                      :lemon:: 3  |                                      :lemon:: 3  |
|                             ipfs swarm peers |                                      :lemon:: 3  |                              :green_apple:: 1 :lemon:: 2  |
|                 ipfs swarm peers --direction |                                          :chestnut: |                                          :chestnut: |
|                   ipfs swarm peers --latency |                                          :chestnut: |                                          :chestnut: |
|                   ipfs swarm peers --streams |                                          :chestnut: |                                          :chestnut: |
|                ipfs swarm peers -v/--verbose |                                          :chestnut: |                                          :chestnut: |
|                                     ipfs tar |                                      :lemon:: 5  |                                      :lemon:: 5  |
|                                 ipfs tar add |                                      :lemon:: 2  |                                      :lemon:: 2  |
|                                 ipfs tar cat |                                      :lemon:: 1  |                                      :lemon:: 1  |
|                                  ipfs update |                                      :lemon:: 5  |                              :green_apple:: 4 :lemon:: 1  |
|                                ipfs urlstore |                                      :lemon:: 8  |                                      :lemon:: 8  |
|                            ipfs urlstore add |                                      :lemon:: 8  |                                      :lemon:: 8  |
|                      ipfs urlstore add --pin |                                      :lemon:: 1  |                                      :lemon:: 1  |
|               ipfs urlstore add -t/--trickle |                                      :lemon:: 2  |                                      :lemon:: 2  |
|                                 ipfs version |                              :tomato:: 2 :lemon:: 29  |                                     :lemon:: 31  |
|                           ipfs version --all |                                      :lemon:: 2  |                                      :lemon:: 2  |
|                        ipfs version --commit |                                          :chestnut: |                                          :chestnut: |
|                          ipfs version --repo |                                          :chestnut: |                                          :chestnut: |
|                     ipfs version -n/--number |                                      :lemon:: 5  |                                      :lemon:: 5  |
