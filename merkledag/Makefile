
all: node.pb.go data.pb.go

node.pb.go: node.proto
	protoc --gogo_out=. --proto_path=../../../../:/usr/local/opt/protobuf/include:. $<

data.pb.go: data.proto
	protoc --go_out=. data.proto

clean:
	rm node.pb.go
