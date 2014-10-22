
all: spipe.pb.go

spipe.pb.go: spipe.proto
	protoc --gogo_out=. --proto_path=../../../../../:/usr/local/opt/protobuf/include:. $<

clean:
	rm spipe.pb.go
