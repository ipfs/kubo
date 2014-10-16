
all: semver.pb.go

semver.pb.go: semver.proto
	protoc --gogo_out=. --proto_path=../../../../../:/usr/include/google:. $<

clean:
	rm semver.pb.go
