
all: mux.pb.go

mux.pb.go: mux.proto
	protoc --gogo_out=. --proto_path=../../../../../:/usr/local/opt/protobuf/include:. $<

clean:
	rm mux.pb.go
