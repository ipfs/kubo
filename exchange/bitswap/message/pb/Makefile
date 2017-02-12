# TODO(brian): add proto tasks
all: message.pb.go

message.pb.go: message.proto
	protoc --gogo_out=. --proto_path=../../../../../:/usr/local/opt/protobuf/include:. $<

clean:
	rm message.pb.go
