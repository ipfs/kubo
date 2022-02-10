package main

import (
	"net"

	pb "github.com/guseggert/go-ds-grpc/proto"
	"github.com/guseggert/go-ds-grpc/server"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/sync"
	"google.golang.org/grpc"
)

func main() {
	ds := sync.MutexWrap(datastore.NewMapDatastore())
	s := grpc.NewServer()
	pb.RegisterDatastoreServer(s, server.New(ds))

	l, err := net.Listen("tcp", "127.0.0.1:8990")
	if err != nil {
		panic(err)
	}
	err = s.Serve(l)
	if err != nil {
		panic(err)
	}
}
