PB = $(wildcard *.proto)
GO = $(PB:.proto=.pb.go)
PROTOBUF_INCLUDE = /usr/local/opt/protobuf/include

all: $(GO)

%.pb.go: %.proto
		protoc --gogo_out=. --proto_path=../../../../../../:$(PROTOBUF_INCLUDE):. $<

clean:
		rm *.pb.go
