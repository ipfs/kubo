
all: semver.pb.go

semver.pb.go: semver.proto
	protoc --gogo_out=. --proto_path=../../../../../:/usr/local/opt/protobuf/include:. $<

clean:
	rm semver.pb.go
