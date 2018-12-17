module github.com/ipfs/go-ipfs

require (
	bazil.org/fuse v0.0.0-20180421153158-65cc252bf669
	github.com/AndreasBriese/bbloom v0.0.0-20180913140656-343706a395b7 // indirect
	github.com/Kubuxu/go-os-helper v0.0.0-20161003143644-3d3fc2fb493d
	github.com/Stebalien/go-bitfield v0.0.0-20180330043415-076a62f9ce6e // indirect
	github.com/agl/ed25519 v0.0.0-20170116200512-5312a6153412 // indirect
	github.com/bifurcation/mint v0.0.0-20181105071958-a14404e9a861 // indirect
	github.com/blang/semver v3.5.1+incompatible
	github.com/bren2010/proquint v0.0.0-20160323162903-38337c27106d
	github.com/btcsuite/btcd v0.0.0-20181130015935-7d2daa5bfef2 // indirect
	github.com/cenkalti/backoff v2.1.0+incompatible
	github.com/cheekybits/genny v1.0.0 // indirect
	github.com/cheggaaa/pb v1.0.25
	github.com/coreos/go-semver v0.2.0 // indirect
	github.com/davidlazar/go-crypto v0.0.0-20170701192655-dcfb0a7ac018 // indirect
	github.com/dgraph-io/badger v1.5.5-0.20181215030126-2d7e01452d7c // indirect
	github.com/dgryski/go-farm v0.0.0-20180109070241-2de33835d102 // indirect
	github.com/dustin/go-humanize v1.0.0
	github.com/elgris/jsondiff v0.0.0-20160530203242-765b5c24c302
	github.com/facebookgo/atomicfile v0.0.0-20151019160806-2de1f203e7d5 // indirect
	github.com/fd/go-nat v1.0.0 // indirect
	github.com/fsnotify/fsnotify v1.4.7
	github.com/gogo/protobuf v1.2.0
	github.com/golang/snappy v0.0.0-20180518054509-2e65f85255db // indirect
	github.com/google/uuid v1.1.0 // indirect
	github.com/gorilla/websocket v1.4.0 // indirect
	github.com/gxed/client_golang v0.9.0-pre1 // indirect
	github.com/gxed/go-shellwords v1.0.3 // indirect
	github.com/gxed/hashland v0.0.0-20180221191214-d9f6b97f8db2 // indirect
	github.com/gxed/pubsub v0.0.0-20180201040156-26ebdf44f824 // indirect
	github.com/hashicorp/go-multierror v1.0.0 // indirect
	github.com/hashicorp/golang-lru v0.5.0
	github.com/hsanjuan/go-libp2p-gostream v0.0.0-20181207231917-ac5c3a126b8e // indirect
	github.com/hsanjuan/go-libp2p-http v0.0.0-20181207233728-1075f49fd2f3
	github.com/ipfs/bbloom v0.0.0-20180721065414-f6503c07a6c2 // indirect
	github.com/ipfs/dir-index-html v1.0.3
	github.com/ipfs/go-bitswap v1.1.13
	github.com/ipfs/go-block-format v0.2.0
	github.com/ipfs/go-blockservice v1.1.14-0.20181207231807-4255e937ad7f
	github.com/ipfs/go-cid v0.9.1-0.20181102235123-033594dcd620
	github.com/ipfs/go-cidutil v0.1.2-0.20181102235650-fe89746dc953
	github.com/ipfs/go-datastore v2.4.1-0.20181109225543-277eeb2fded2+incompatible
	github.com/ipfs/go-detect-race v1.0.1
	github.com/ipfs/go-ds-badger v1.4.11-0.20181109230345-4a093545f2f6
	github.com/ipfs/go-ds-flatfs v1.3.3
	github.com/ipfs/go-ds-leveldb v1.2.1
	github.com/ipfs/go-ds-measure v1.4.4-0.20181110192506-9139d950837a
	github.com/ipfs/go-fs-lock v0.1.8
	github.com/ipfs/go-ipfs-addr v0.1.25
	github.com/ipfs/go-ipfs-blockstore v0.1.2
	github.com/ipfs/go-ipfs-chunker v0.1.3
	github.com/ipfs/go-ipfs-cmdkit v1.1.3-0.20181016115945-0262a1200120
	github.com/ipfs/go-ipfs-cmds v2.0.2-0.20181213003447-9bdd0f2e161c+incompatible
	github.com/ipfs/go-ipfs-config v0.2.16-0.20181214000854-f1ee16fa41a9
	github.com/ipfs/go-ipfs-delay v0.0.1 // indirect
	github.com/ipfs/go-ipfs-ds-help v0.1.2
	github.com/ipfs/go-ipfs-exchange-interface v0.1.0
	github.com/ipfs/go-ipfs-exchange-offline v0.1.3
	github.com/ipfs/go-ipfs-files v0.0.0-20181017153249-bd692080a8c8
	github.com/ipfs/go-ipfs-flags v0.0.1 // indirect
	github.com/ipfs/go-ipfs-posinfo v0.1.0
	github.com/ipfs/go-ipfs-pq v0.0.1 // indirect
	github.com/ipfs/go-ipfs-routing v0.1.7
	github.com/ipfs/go-ipfs-util v1.2.8
	github.com/ipfs/go-ipld-cbor v1.5.1-0.20181203233501-cb3f4a55d301
	github.com/ipfs/go-ipld-format v0.6.1-0.20181109172815-a6db26417dc5
	github.com/ipfs/go-ipld-git v0.3.0
	github.com/ipfs/go-ipns v0.1.13
	github.com/ipfs/go-libp2p-peer v2.4.0+incompatible // indirect
	github.com/ipfs/go-libp2p-pubsub v0.0.0-20160916003311-ea0332454771 // indirect
	github.com/ipfs/go-log v1.5.7
	github.com/ipfs/go-merkledag v1.1.13
	github.com/ipfs/go-metrics-interface v0.2.0
	github.com/ipfs/go-metrics-prometheus v0.3.9
	github.com/ipfs/go-mfs v0.1.15
	github.com/ipfs/go-path v1.1.13
	github.com/ipfs/go-todocounter v1.0.1 // indirect
	github.com/ipfs/go-unixfs v1.1.16-0.20181213224303-cef0e21141db
	github.com/ipfs/go-verifcid v0.0.5-0.20181102235604-8909da14ac6d
	github.com/ipfs/iptb v1.3.8-0.20181112175249-5e6d8226eed0
	github.com/ipfs/iptb-plugins v0.0.0-20181214001730-b91ac6a447c7
	github.com/jbenet/go-context v0.0.0-20150711004518-d14ea06fba99 // indirect
	github.com/jbenet/go-fuse-version v0.0.0-20160322195114-6d4c97bcf253 // indirect
	github.com/jbenet/go-is-domain v0.0.0-20160119110217-ba9815c809e0
	github.com/jbenet/go-random v0.0.0-20150829044232-384f606e91f5
	github.com/jbenet/go-temp-err-catcher v0.0.0-20150120210811-aac704a3f4f2
	github.com/jbenet/goprocess v0.0.0-20160826012719-b497e2f366b8
	github.com/libp2p/go-addr-util v2.0.7+incompatible // indirect
	github.com/libp2p/go-buffer-pool v0.1.1 // indirect
	github.com/libp2p/go-conn-security v0.1.15 // indirect
	github.com/libp2p/go-conn-security-multistream v0.1.15 // indirect
	github.com/libp2p/go-flow-metrics v0.2.0 // indirect
	github.com/libp2p/go-libp2p v6.0.23+incompatible
	github.com/libp2p/go-libp2p-circuit v2.3.2+incompatible
	github.com/libp2p/go-libp2p-connmgr v0.3.23
	github.com/libp2p/go-libp2p-crypto v2.0.1+incompatible
	github.com/libp2p/go-libp2p-host v3.0.15+incompatible
	github.com/libp2p/go-libp2p-interface-connmgr v0.0.21
	github.com/libp2p/go-libp2p-interface-pnet v3.0.0+incompatible // indirect
	github.com/libp2p/go-libp2p-kad-dht v4.4.12+incompatible
	github.com/libp2p/go-libp2p-kbucket v2.2.12+incompatible
	github.com/libp2p/go-libp2p-loggables v1.1.24
	github.com/libp2p/go-libp2p-metrics v2.1.7+incompatible
	github.com/libp2p/go-libp2p-nat v0.8.8 // indirect
	github.com/libp2p/go-libp2p-net v3.0.15+incompatible
	github.com/libp2p/go-libp2p-netutil v0.4.12 // indirect
	github.com/libp2p/go-libp2p-peer v2.4.0+incompatible
	github.com/libp2p/go-libp2p-peerstore v2.0.6+incompatible
	github.com/libp2p/go-libp2p-pnet v3.0.4+incompatible
	github.com/libp2p/go-libp2p-protocol v1.0.0
	github.com/libp2p/go-libp2p-pubsub v0.10.3-0.20181214073107-6fc7deb28686
	github.com/libp2p/go-libp2p-pubsub-router v0.4.14
	github.com/libp2p/go-libp2p-quic-transport v0.2.9
	github.com/libp2p/go-libp2p-record v4.1.7+incompatible
	github.com/libp2p/go-libp2p-routing v2.7.1+incompatible
	github.com/libp2p/go-libp2p-routing-helpers v0.3.8
	github.com/libp2p/go-libp2p-secio v2.0.17+incompatible
	github.com/libp2p/go-libp2p-swarm v3.0.22+incompatible
	github.com/libp2p/go-libp2p-transport v3.0.15+incompatible // indirect
	github.com/libp2p/go-libp2p-transport-upgrader v0.1.16 // indirect
	github.com/libp2p/go-maddr-filter v1.1.10
	github.com/libp2p/go-mplex v0.2.30 // indirect
	github.com/libp2p/go-msgio v0.0.6 // indirect
	github.com/libp2p/go-reuseport v0.1.18 // indirect
	github.com/libp2p/go-reuseport-transport v0.1.11 // indirect
	github.com/libp2p/go-sockaddr v1.0.3 // indirect
	github.com/libp2p/go-stream-muxer v3.0.1+incompatible
	github.com/libp2p/go-tcp-transport v2.0.16+incompatible // indirect
	github.com/libp2p/go-testutil v1.2.10
	github.com/libp2p/go-ws-transport v2.0.15+incompatible // indirect
	github.com/lucas-clemente/aes12 v0.0.0-20171027163421-cd47fb39b79f // indirect
	github.com/lucas-clemente/quic-go v0.10.0 // indirect
	github.com/lucas-clemente/quic-go-certificates v0.0.0-20160823095156-d2f86524cced // indirect
	github.com/mattn/go-colorable v0.0.9 // indirect
	github.com/mattn/go-isatty v0.0.4 // indirect
	github.com/mattn/go-runewidth v0.0.4 // indirect
	github.com/mgutz/ansi v0.0.0-20170206155736-9520e82c474b // indirect
	github.com/miekg/dns v1.1.1 // indirect
	github.com/minio/blake2b-simd v0.0.0-20160723061019-3f5f724cb5b1 // indirect
	github.com/minio/sha256-simd v0.0.0-20181005183134-51976451ce19 // indirect
	github.com/mitchellh/go-homedir v1.0.0
	github.com/mr-tron/base58 v1.1.0
	github.com/multiformats/go-multiaddr v1.3.0
	github.com/multiformats/go-multiaddr-dns v0.2.5
	github.com/multiformats/go-multiaddr-net v1.6.3
	github.com/multiformats/go-multibase v0.3.0
	github.com/multiformats/go-multicodec v0.1.6 // indirect
	github.com/multiformats/go-multihash v1.0.8
	github.com/multiformats/go-multistream v0.3.9 // indirect
	github.com/opentracing/opentracing-go v1.0.2
	github.com/pkg/errors v0.8.0 // indirect
	github.com/polydawn/refmt v0.0.0-20181120165158-8b1e46de349b // indirect
	github.com/prometheus/client_golang v0.9.2
	github.com/rs/cors v1.6.0 // indirect
	github.com/spaolacci/murmur3 v0.0.0-20180118202830-f09979ecbc72 // indirect
	github.com/syndtr/goleveldb v0.0.0-20181128100959-b001fa50d6b2
	github.com/texttheater/golang-levenshtein v0.0.0-20180516184445-d188e65d659e // indirect
	github.com/urfave/cli v1.20.0 // indirect
	github.com/whyrusleeping/base32 v0.0.0-20170828182744-c30ac30633cc
	github.com/whyrusleeping/cbor v0.0.0-20171005072247-63513f603b11 // indirect
	github.com/whyrusleeping/chunker v0.0.0-20181014151217-fe64bd25879f // indirect
	github.com/whyrusleeping/go-ctrlnet v0.0.0-20180313164037-f564fbbdaa95 // indirect
	github.com/whyrusleeping/go-keyspace v0.0.0-20160322163242-5b898ac5add1 // indirect
	github.com/whyrusleeping/go-libp2p-pubsub v0.0.0-20160422011506-e44babf62e59 // indirect
	github.com/whyrusleeping/go-logging v0.0.0-20170515211332-0457bb6b88fc // indirect
	github.com/whyrusleeping/go-notifier v0.0.0-20170827234753-097c5d47330f // indirect
	github.com/whyrusleeping/go-smux-multiplex v3.0.16+incompatible
	github.com/whyrusleeping/go-smux-multistream v2.0.2+incompatible // indirect
	github.com/whyrusleeping/go-smux-yamux v2.0.8+incompatible
	github.com/whyrusleeping/go-sysinfo v0.0.0-20160322193845-dee6add16c7d
	github.com/whyrusleeping/mafmt v1.2.8 // indirect
	github.com/whyrusleeping/mdns v0.0.0-20180901202407-ef14215e6b30 // indirect
	github.com/whyrusleeping/multiaddr-filter v0.0.0-20160516205228-e903e4adabd7
	github.com/whyrusleeping/stump v0.0.0-20160611222256-206f8f13aae1 // indirect
	github.com/whyrusleeping/tar-utils v0.0.0-20180509141711-8c6c8ba81d5c
	github.com/whyrusleeping/timecache v0.0.0-20160911033111-cfcb2f1abfee // indirect
	github.com/whyrusleeping/yamux v1.1.2 // indirect
	go4.org v0.0.0-20181109185143-00e24f1b2599 // indirect
	golang.org/x/crypto v0.0.0-20181203042331-505ab145d0a9 // indirect
	golang.org/x/sys v0.0.0-20181213200352-4d1cda033e06
	gopkg.in/VividCortex/ewma.v1 v1.1.1 // indirect
	gopkg.in/cheggaaa/pb.v2 v2.0.6 // indirect
	gopkg.in/fatih/color.v1 v1.7.0 // indirect
	gopkg.in/mattn/go-colorable.v0 v0.0.9 // indirect
	gopkg.in/mattn/go-isatty.v0 v0.0.4 // indirect
	gopkg.in/mattn/go-runewidth.v0 v0.0.4 // indirect
)
