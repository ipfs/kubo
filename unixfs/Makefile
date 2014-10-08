all: data.pb.go

data.pb.go: data.proto
	protoc --go_out=. data.proto

