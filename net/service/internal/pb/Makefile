
all: request.pb.go

request.pb.go: request.proto
	protoc --gogo_out=. --proto_path=../../../../../:/usr/local/opt/protobuf/include:. $<

clean:
	rm request.pb.go
