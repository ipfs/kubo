module github.com/ipfs/go-ipfs

require (
	bazil.org/fuse v0.0.0-20200117225306-7b5117fecadc
	contrib.go.opencensus.io/exporter/prometheus v0.4.0
	github.com/blang/semver/v4 v4.0.0
	github.com/cenkalti/backoff/v4 v4.1.2
	github.com/ceramicnetwork/go-dag-jose v0.1.0
	github.com/cespare/xxhash v1.1.0
	github.com/cheggaaa/pb v1.0.29
	github.com/coreos/go-systemd/v22 v22.3.2
	github.com/dustin/go-humanize v1.0.0
	github.com/elgris/jsondiff v0.0.0-20160530203242-765b5c24c302
	github.com/facebookgo/atomicfile v0.0.0-20151019160806-2de1f203e7d5
	github.com/fsnotify/fsnotify v1.5.1
	github.com/gabriel-vasile/mimetype v1.4.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/ipfs/go-bitswap v0.6.0
	github.com/ipfs/go-block-format v0.0.3
	github.com/ipfs/go-blockservice v0.3.0
	github.com/ipfs/go-cid v0.1.0
	github.com/ipfs/go-cidutil v0.0.2
	github.com/ipfs/go-datastore v0.5.1
	github.com/ipfs/go-detect-race v0.0.1
	github.com/ipfs/go-ds-badger v0.3.0
	github.com/ipfs/go-ds-flatfs v0.5.1
	github.com/ipfs/go-ds-leveldb v0.5.0
	github.com/ipfs/go-ds-measure v0.2.0
	github.com/ipfs/go-fetcher v1.6.1
	github.com/ipfs/go-filestore v1.2.0
	github.com/ipfs/go-fs-lock v0.0.7
	github.com/ipfs/go-graphsync v0.13.1
	github.com/ipfs/go-ipfs-blockstore v1.2.0
	github.com/ipfs/go-ipfs-chunker v0.0.5
	github.com/ipfs/go-ipfs-cmds v0.8.1
	github.com/ipfs/go-ipfs-exchange-interface v0.1.0
	github.com/ipfs/go-ipfs-exchange-offline v0.2.0
	github.com/ipfs/go-ipfs-files v0.1.1
	github.com/ipfs/go-ipfs-keystore v0.0.2
	github.com/ipfs/go-ipfs-pinner v0.2.1
	github.com/ipfs/go-ipfs-posinfo v0.0.1
	github.com/ipfs/go-ipfs-provider v0.7.1
	github.com/ipfs/go-ipfs-routing v0.2.1
	github.com/ipfs/go-ipfs-util v0.0.2
	github.com/ipfs/go-ipld-cbor v0.0.5
	github.com/ipfs/go-ipld-format v0.4.0
	github.com/ipfs/go-ipld-git v0.1.1
	github.com/ipfs/go-ipld-legacy v0.1.1
	github.com/ipfs/go-ipns v0.1.2
	github.com/ipfs/go-log v1.0.5
	github.com/ipfs/go-merkledag v0.6.0
	github.com/ipfs/go-metrics-interface v0.0.1
	github.com/ipfs/go-metrics-prometheus v0.0.2
	github.com/ipfs/go-mfs v0.2.1
	github.com/ipfs/go-namesys v0.5.0
	github.com/ipfs/go-path v0.3.0
	github.com/ipfs/go-pinning-service-http-client v0.1.1
	github.com/ipfs/go-unixfs v0.3.1
	github.com/ipfs/go-unixfsnode v1.4.0
	github.com/ipfs/go-verifcid v0.0.1
	github.com/ipfs/interface-go-ipfs-core v0.7.0
	github.com/ipfs/tar-utils v0.0.2
	github.com/ipld/go-car v0.3.2
	github.com/ipld/go-car/v2 v2.1.1
	github.com/ipld/go-codec-dagpb v1.4.0
	github.com/ipld/go-ipld-prime v0.16.0
	github.com/jbenet/go-random v0.0.0-20190219211222-123a90aedc0c
	github.com/jbenet/go-temp-err-catcher v0.1.0
	github.com/jbenet/goprocess v0.1.4
	github.com/libp2p/go-doh-resolver v0.4.0
	github.com/libp2p/go-libp2p v0.19.1
	github.com/libp2p/go-libp2p-core v0.15.1
	github.com/libp2p/go-libp2p-discovery v0.6.0
	github.com/libp2p/go-libp2p-http v0.2.1
	github.com/libp2p/go-libp2p-kad-dht v0.15.0
	github.com/libp2p/go-libp2p-kbucket v0.4.7
	github.com/libp2p/go-libp2p-loggables v0.1.0
	github.com/libp2p/go-libp2p-mplex v0.7.0
	github.com/libp2p/go-libp2p-noise v0.4.0
	github.com/libp2p/go-libp2p-peerstore v0.6.0
	github.com/libp2p/go-libp2p-pubsub v0.6.1
	github.com/libp2p/go-libp2p-pubsub-router v0.5.0
	github.com/libp2p/go-libp2p-quic-transport v0.17.0
	github.com/libp2p/go-libp2p-record v0.1.3
	github.com/libp2p/go-libp2p-resource-manager v0.3.0
	github.com/libp2p/go-libp2p-routing-helpers v0.2.3
	github.com/libp2p/go-libp2p-swarm v0.10.2
	github.com/libp2p/go-libp2p-testing v0.9.2
	github.com/libp2p/go-libp2p-tls v0.4.1
	github.com/libp2p/go-libp2p-yamux v0.9.1
	github.com/libp2p/go-socket-activation v0.1.0
	github.com/libp2p/go-tcp-transport v0.5.1
	github.com/libp2p/go-ws-transport v0.6.0
	github.com/miekg/dns v1.1.48
	github.com/mitchellh/go-homedir v1.1.0
	github.com/multiformats/go-multiaddr v0.5.0
	github.com/multiformats/go-multiaddr-dns v0.3.1
	github.com/multiformats/go-multibase v0.0.3
	github.com/multiformats/go-multicodec v0.4.1
	github.com/multiformats/go-multihash v0.1.0
	github.com/opentracing/opentracing-go v1.2.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.12.1
	github.com/prometheus/common v0.33.0 // indirect
	github.com/stretchr/testify v1.7.1
	github.com/syndtr/goleveldb v1.0.0
	github.com/wI2L/jsondiff v0.2.0
	github.com/whyrusleeping/go-sysinfo v0.0.0-20190219211824-4a357d4b90b1
	github.com/whyrusleeping/multiaddr-filter v0.0.0-20160516205228-e903e4adabd7
	go.opencensus.io v0.23.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.31.0
	go.opentelemetry.io/otel v1.6.3
	go.opentelemetry.io/otel/exporters/jaeger v1.6.3
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.6.3
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.6.3
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.6.3
	go.opentelemetry.io/otel/exporters/zipkin v1.6.3
	go.opentelemetry.io/otel/sdk v1.6.3
	go.opentelemetry.io/otel/trace v1.6.3
	go.uber.org/dig v1.14.0
	go.uber.org/fx v1.16.0
	go.uber.org/zap v1.21.0
	golang.org/x/crypto v0.0.0-20220411220226-7b82a4e95df4
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20220412211240-33da011f77ad
)

require (
	github.com/AndreasBriese/bbloom v0.0.0-20190825152654-46b345b51c96 // indirect
	github.com/Kubuxu/go-os-helper v0.0.1 // indirect
	github.com/Stebalien/go-bitfield v0.0.1 // indirect
	github.com/alecthomas/units v0.0.0-20210927113745-59d0afb8317a // indirect
	github.com/alexbrainman/goissue34681 v0.0.0-20191006012335-3fc7a47baff5 // indirect
	github.com/benbjohnson/clock v1.3.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/btcsuite/btcd v0.22.0-beta // indirect
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/cheekybits/genny v1.0.0 // indirect
	github.com/containerd/cgroups v1.0.3 // indirect
	github.com/crackcomm/go-gitignore v0.0.0-20170627025303-887ab5e44cc3 // indirect
	github.com/cskr/pubsub v1.0.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/davidlazar/go-crypto v0.0.0-20200604182044-b73af7476f6c // indirect
	github.com/dgraph-io/badger v1.6.2 // indirect
	github.com/dgraph-io/ristretto v0.0.2 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/elastic/gosigar v0.14.2 // indirect
	github.com/felixge/httpsnoop v1.0.2 // indirect
	github.com/flynn/noise v1.0.0 // indirect
	github.com/francoispqt/gojay v1.2.13 // indirect
	github.com/go-kit/log v0.2.0 // indirect
	github.com/go-logfmt/logfmt v0.5.1 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-task/slim-sprig v0.0.0-20210107165309-348f09dbbbc0 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/gopacket v1.1.19 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.7.0 // indirect
	github.com/hannahhoward/go-pubsub v0.0.0-20200423002714-8d62886cc36e // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/huin/goupnp v1.0.3 // indirect
	github.com/ipfs/bbloom v0.0.4 // indirect
	github.com/ipfs/go-bitfield v1.0.0 // indirect
	github.com/ipfs/go-ipfs-delay v0.0.1 // indirect
	github.com/ipfs/go-ipfs-ds-help v1.1.0 // indirect
	github.com/ipfs/go-ipfs-pq v0.0.2 // indirect
	github.com/ipfs/go-log/v2 v2.5.1 // indirect
	github.com/ipfs/go-peertaskqueue v0.7.1 // indirect
	github.com/jackpal/go-nat-pmp v1.0.2 // indirect
	github.com/klauspost/compress v1.15.1 // indirect
	github.com/klauspost/cpuid/v2 v2.0.12 // indirect
	github.com/koron/go-ssdp v0.0.2 // indirect
	github.com/libp2p/go-buffer-pool v0.0.2 // indirect
	github.com/libp2p/go-cidranger v1.1.0 // indirect
	github.com/libp2p/go-conn-security-multistream v0.3.0 // indirect
	github.com/libp2p/go-eventbus v0.2.1 // indirect
	github.com/libp2p/go-flow-metrics v0.0.3 // indirect
	github.com/libp2p/go-libp2p-asn-util v0.2.0 // indirect
	github.com/libp2p/go-libp2p-blankhost v0.3.0 // indirect
	github.com/libp2p/go-libp2p-gostream v0.3.0 // indirect
	github.com/libp2p/go-libp2p-nat v0.1.0 // indirect
	github.com/libp2p/go-libp2p-pnet v0.2.0 // indirect
	github.com/libp2p/go-libp2p-transport-upgrader v0.7.1 // indirect
	github.com/libp2p/go-libp2p-xor v0.0.0-20210714161855-5c005aca55db // indirect
	github.com/libp2p/go-mplex v0.7.0 // indirect
	github.com/libp2p/go-msgio v0.2.0 // indirect
	github.com/libp2p/go-nat v0.1.0 // indirect
	github.com/libp2p/go-netroute v0.2.0 // indirect
	github.com/libp2p/go-openssl v0.0.7 // indirect
	github.com/libp2p/go-reuseport v0.1.0 // indirect
	github.com/libp2p/go-reuseport-transport v0.1.0 // indirect
	github.com/libp2p/go-stream-muxer-multistream v0.4.0 // indirect
	github.com/libp2p/go-yamux/v3 v3.1.1 // indirect
	github.com/libp2p/zeroconf/v2 v2.1.1 // indirect
	github.com/lucas-clemente/quic-go v0.27.0 // indirect
	github.com/marten-seemann/qtls-go1-16 v0.1.5 // indirect
	github.com/marten-seemann/qtls-go1-17 v0.1.1 // indirect
	github.com/marten-seemann/qtls-go1-18 v0.1.1 // indirect
	github.com/marten-seemann/tcp v0.0.0-20210406111302-dfbc87cc63fd // indirect
	github.com/mattn/go-colorable v0.1.4 // indirect
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/mattn/go-runewidth v0.0.4 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.1 // indirect
	github.com/mgutz/ansi v0.0.0-20170206155736-9520e82c474b // indirect
	github.com/mikioh/tcpinfo v0.0.0-20190314235526-30a79bb1804b // indirect
	github.com/mikioh/tcpopt v0.0.0-20190314235656-172688c1accc // indirect
	github.com/minio/blake2b-simd v0.0.0-20160723061019-3f5f724cb5b1 // indirect
	github.com/minio/sha256-simd v1.0.0 // indirect
	github.com/mr-tron/base58 v1.2.0 // indirect
	github.com/multiformats/go-base32 v0.0.4 // indirect
	github.com/multiformats/go-base36 v0.1.0 // indirect
	github.com/multiformats/go-multiaddr-fmt v0.1.0 // indirect
	github.com/multiformats/go-multistream v0.3.0 // indirect
	github.com/multiformats/go-varint v0.0.6 // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/onsi/ginkgo v1.16.5 // indirect
	github.com/opencontainers/runtime-spec v1.0.2 // indirect
	github.com/openzipkin/zipkin-go v0.4.0 // indirect
	github.com/pbnjay/memory v0.0.0-20210728143218-7b4eea64cf58 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/polydawn/refmt v0.0.0-20201211092308-30ac6d18308e // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/procfs v0.7.3 // indirect
	github.com/prometheus/statsd_exporter v0.21.0 // indirect
	github.com/raulk/clock v1.1.0 // indirect
	github.com/raulk/go-watchdog v1.2.0 // indirect
	github.com/rs/cors v1.7.0 // indirect
	github.com/spacemonkeygo/spacelog v0.0.0-20180420211403-2296661a0572 // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/texttheater/golang-levenshtein v0.0.0-20180516184445-d188e65d659e // indirect
	github.com/tidwall/gjson v1.14.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	github.com/whyrusleeping/base32 v0.0.0-20170828182744-c30ac30633cc // indirect
	github.com/whyrusleeping/cbor-gen v0.0.0-20210219115102-f37d292932f2 // indirect
	github.com/whyrusleeping/chunker v0.0.0-20181014151217-fe64bd25879f // indirect
	github.com/whyrusleeping/go-keyspace v0.0.0-20160322163242-5b898ac5add1 // indirect
	github.com/whyrusleeping/mdns v0.0.0-20190826153040-b9b60ed33aa9 // indirect
	github.com/whyrusleeping/timecache v0.0.0-20160911033111-cfcb2f1abfee // indirect
	go.opentelemetry.io/otel/exporters/otlp/internal/retry v1.6.3 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.6.3 // indirect
	go.opentelemetry.io/otel/metric v0.28.0 // indirect
	go.opentelemetry.io/proto/otlp v0.15.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	go4.org v0.0.0-20200411211856-f5505b9728dd // indirect
	golang.org/x/exp v0.0.0-20210615023648-acb5c1269671 // indirect
	golang.org/x/mod v0.6.0-dev.0.20220106191415-9b9b3d81d5e3 // indirect
	golang.org/x/net v0.0.0-20220418201149-a630d4f3e7a2 // indirect
	golang.org/x/oauth2 v0.0.0-20220223155221-ee480838109b // indirect
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211 // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/tools v0.1.10 // indirect
	golang.org/x/xerrors v0.0.0-20220411194840-2f41105eb62f // indirect
	google.golang.org/appengine v1.6.6 // indirect
	google.golang.org/genproto v0.0.0-20211118181313-81c1377c94b1 // indirect
	google.golang.org/grpc v1.45.0 // indirect
	google.golang.org/protobuf v1.28.0 // indirect
	gopkg.in/square/go-jose.v2 v2.5.1 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	lukechampine.com/blake3 v1.1.7 // indirect
)

go 1.17
