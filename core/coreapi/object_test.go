package coreapi_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"io/ioutil"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	emptyNode, err := api.Object().New(ctx)
	if err != nil {
		t.Fatal(err)
	}

	dirNode, err := api.Object().New(ctx, api.Object().WithType("unixfs-dir"))
	if err != nil {
		t.Fatal(err)
	}

	if emptyNode.String() != "QmdfTbBqBPQ7VNxZEYEj14VmRuZBkqFbiwReogJgS1zR1n" {
		t.Errorf("Unexpected emptyNode path: %s", emptyNode.String())
	}

	if dirNode.String() != "QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn" {
		t.Errorf("Unexpected dirNode path: %s", dirNode.String())
	}
}

func TestObjectPut(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	p1, err := api.Object().Put(ctx, strings.NewReader(`{"Data":"foo"}`))
	if err != nil {
		t.Fatal(err)
	}

	p2, err := api.Object().Put(ctx, strings.NewReader(`{"Data":"YmFy"}`), api.Object().WithDataType("base64")) //bar
	if err != nil {
		t.Fatal(err)
	}

	pbBytes, err := hex.DecodeString("0a0362617a")
	if err != nil {
		t.Fatal(err)
	}

	p3, err := api.Object().Put(ctx, bytes.NewReader(pbBytes), api.Object().WithInputEnc("protobuf"))
	if err != nil {
		t.Fatal(err)
	}

	if p1.String() != "/ipfs/QmQeGyS87nyijii7kFt1zbe4n2PsXTFimzsdxyE9qh9TST" {
		t.Errorf("unexpected path: %s", p1.String())
	}

	if p2.String() != "/ipfs/QmNeYRbCibmaMMK6Du6ChfServcLqFvLJF76PzzF76SPrZ" {
		t.Errorf("unexpected path: %s", p2.String())
	}

	if p3.String() != "/ipfs/QmZreR7M2t7bFXAdb1V5FtQhjk4t36GnrvueLJowJbQM9m" {
		t.Errorf("unexpected path: %s", p3.String())
	}
}

func TestObjectGet(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	p1, err := api.Object().Put(ctx, strings.NewReader(`{"Data":"foo"}`))
	if err != nil {
		t.Fatal(err)
	}

	nd, err := api.Object().Get(ctx, p1)
	if err != nil {
		t.Fatal(err)
	}

	if string(nd.RawData()[len(nd.RawData())-3:]) != "foo" {
		t.Fatal("got non-matching data")
	}
}

func TestObjectData(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	p1, err := api.Object().Put(ctx, strings.NewReader(`{"Data":"foo"}`))
	if err != nil {
		t.Fatal(err)
	}

	r, err := api.Object().Data(ctx, p1)
	if err != nil {
		t.Fatal(err)
	}

	data, err := ioutil.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != "foo" {
		t.Fatal("got non-matching data")
	}
}

func TestObjectLinks(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	p1, err := api.Object().Put(ctx, strings.NewReader(`{"Data":"foo"}`))
	if err != nil {
		t.Fatal(err)
	}

	p2, err := api.Object().Put(ctx, strings.NewReader(`{"Links":[{"Name":"bar", "Hash":"`+p1.Cid().String()+`"}]}`))
	if err != nil {
		t.Fatal(err)
	}

	links, err := api.Object().Links(ctx, p2)
	if err != nil {
		t.Fatal(err)
	}

	if len(links) != 1 {
		t.Errorf("unexpected number of links: %d", len(links))
	}

	if links[0].Cid.String() != p1.Cid().String() {
		t.Fatal("cids didn't batch")
	}

	if links[0].Name != "bar" {
		t.Fatal("unexpected link name")
	}
}

func TestObjectStat(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	p1, err := api.Object().Put(ctx, strings.NewReader(`{"Data":"foo"}`))
	if err != nil {
		t.Fatal(err)
	}

	p2, err := api.Object().Put(ctx, strings.NewReader(`{"Data":"bazz", "Links":[{"Name":"bar", "Hash":"`+p1.Cid().String()+`", "Size":3}]}`))
	if err != nil {
		t.Fatal(err)
	}

	stat, err := api.Object().Stat(ctx, p2)
	if err != nil {
		t.Fatal(err)
	}

	if stat.Cid.String() != p2.Cid().String() {
		t.Error("unexpected stat.Cid")
	}

	if stat.NumLinks != 1 {
		t.Errorf("unexpected stat.NumLinks")
	}

	if stat.BlockSize != 51 {
		t.Error("unexpected stat.BlockSize")
	}

	if stat.LinksSize != 47 {
		t.Errorf("unexpected stat.LinksSize: %d", stat.LinksSize)
	}

	if stat.DataSize != 4 {
		t.Error("unexpected stat.DataSize")
	}

	if stat.CumulativeSize != 54 {
		t.Error("unexpected stat.DataSize")
	}
}

func TestObjectAddLink(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	p1, err := api.Object().Put(ctx, strings.NewReader(`{"Data":"foo"}`))
	if err != nil {
		t.Fatal(err)
	}

	p2, err := api.Object().Put(ctx, strings.NewReader(`{"Data":"bazz", "Links":[{"Name":"bar", "Hash":"`+p1.Cid().String()+`", "Size":3}]}`))
	if err != nil {
		t.Fatal(err)
	}

	p3, err := api.Object().AddLink(ctx, p2, "abc", p2)
	if err != nil {
		t.Fatal(err)
	}

	links, err := api.Object().Links(ctx, p3)
	if err != nil {
		t.Fatal(err)
	}

	if len(links) != 2 {
		t.Errorf("unexpected number of links: %d", len(links))
	}

	if links[0].Name != "abc" {
		t.Errorf("unexpected link 0 name: %s", links[0].Name)
	}

	if links[1].Name != "bar" {
		t.Errorf("unexpected link 1 name: %s", links[1].Name)
	}
}

func TestObjectAddLinkCreate(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	p1, err := api.Object().Put(ctx, strings.NewReader(`{"Data":"foo"}`))
	if err != nil {
		t.Fatal(err)
	}

	p2, err := api.Object().Put(ctx, strings.NewReader(`{"Data":"bazz", "Links":[{"Name":"bar", "Hash":"`+p1.Cid().String()+`", "Size":3}]}`))
	if err != nil {
		t.Fatal(err)
	}

	p3, err := api.Object().AddLink(ctx, p2, "abc/d", p2)
	if err == nil {
		t.Fatal("expected an error")
	}
	if err.Error() != "no link by that name" {
		t.Fatalf("unexpected error: %s", err.Error())
	}

	p3, err = api.Object().AddLink(ctx, p2, "abc/d", p2, api.Object().WithCreate(true))
	if err != nil {
		t.Fatal(err)
	}

	links, err := api.Object().Links(ctx, p3)
	if err != nil {
		t.Fatal(err)
	}

	if len(links) != 2 {
		t.Errorf("unexpected number of links: %d", len(links))
	}

	if links[0].Name != "abc" {
		t.Errorf("unexpected link 0 name: %s", links[0].Name)
	}

	if links[1].Name != "bar" {
		t.Errorf("unexpected link 1 name: %s", links[1].Name)
	}
}

func TestObjectRmLink(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	p1, err := api.Object().Put(ctx, strings.NewReader(`{"Data":"foo"}`))
	if err != nil {
		t.Fatal(err)
	}

	p2, err := api.Object().Put(ctx, strings.NewReader(`{"Data":"bazz", "Links":[{"Name":"bar", "Hash":"`+p1.Cid().String()+`", "Size":3}]}`))
	if err != nil {
		t.Fatal(err)
	}

	p3, err := api.Object().RmLink(ctx, p2, "bar")
	if err != nil {
		t.Fatal(err)
	}

	links, err := api.Object().Links(ctx, p3)
	if err != nil {
		t.Fatal(err)
	}

	if len(links) != 0 {
		t.Errorf("unexpected number of links: %d", len(links))
	}
}

func TestObjectAddData(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	p1, err := api.Object().Put(ctx, strings.NewReader(`{"Data":"foo"}`))
	if err != nil {
		t.Fatal(err)
	}

	p2, err := api.Object().AppendData(ctx, p1, strings.NewReader("bar"))
	if err != nil {
		t.Fatal(err)
	}

	r, err := api.Object().Data(ctx, p2)
	if err != nil {
		t.Fatal(err)
	}

	data, err := ioutil.ReadAll(r)

	if string(data) != "foobar" {
		t.Error("unexpected data")
	}
}

func TestObjectSetData(t *testing.T) {
	ctx := context.Background()
	_, api, err := makeAPI(ctx)
	if err != nil {
		t.Fatal(err)
	}

	p1, err := api.Object().Put(ctx, strings.NewReader(`{"Data":"foo"}`))
	if err != nil {
		t.Fatal(err)
	}

	p2, err := api.Object().SetData(ctx, p1, strings.NewReader("bar"))
	if err != nil {
		t.Fatal(err)
	}

	r, err := api.Object().Data(ctx, p2)
	if err != nil {
		t.Fatal(err)
	}

	data, err := ioutil.ReadAll(r)

	if string(data) != "bar" {
		t.Error("unexpected data")
	}
}
