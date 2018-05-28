#!/bin/sh

set -e

if ! [ -f merkledag.proto ] ; then
	echo 1>&2 "You must run the regenerator while in the directory containing merkledag.proto"
	exit 1
fi

export GOGO_GX=gx/ipfs/QmZ4Qi3GaRbjcx28Sme5eMH7RQjGkt8wHxt2a65oLaeFEV/gogo-protobuf

if ! [ -d $GOPATH/src/$GOGO_GX ] ; then
	echo 1>&2 "\$GOPATH/src/$GOGO_GX does not seem to exist... unable to continue"
	exit 1
fi

# The version we have bundled in GX already contains the fix
# for https://github.com/gogo/protobuf/issues/42, need to undo it
#
patch -sf $GOPATH/src/$GOGO_GX/plugin/marshalto/marshalto.go <<EOP || /bin/true
--- marshalto.go.orig   2018-05-11 11:46:57.000000000 +0200
+++ marshalto.go        2018-05-11 11:49:04.265919111 +0200
@@ -132 +132 @@
-	"sort"
+//	"sort"
@@ -1147,2 +1147,2 @@
-		fields := orderFields(message.GetField())
-		sort.Sort(fields)
+//		fields := orderFields(message.GetField())
+//		sort.Sort(fields)
EOP

export PATH=".:$PATH"
protoc -I. -I$GOPATH/src/$GOGO_GX -I$GOPATH/src/$GOGO_GX/protobuf --gofast_gxpin_out=. merkledag.proto

perl -p -i -e "s{github.com/gogo/protobuf}{$GOGO_GX}g"              merkledag.pb.go merkledagpb_test.go
perl -p -i -e 's{^(?=import _ "gogoproto")}{\n// disable unused }'  merkledag.pb.go merkledagpb_test.go
