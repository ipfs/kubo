
all: node.pb.go

node.pb.go: node.proto
	protoc --gogo_out=. --proto_path=../../../../:/usr/local/opt/protobuf/include:. $<

clean:
	rm node.pb.go
