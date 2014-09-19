package message

import (
	"bytes"
	"testing"

	u "github.com/jbenet/go-ipfs/util"
	testutil "github.com/jbenet/go-ipfs/util/testutil"
)

func TestAppendWanted(t *testing.T) {
	const str = "foo"
	m := New()
	m.AppendWanted(u.Key(str))

	if !contains(m.ToProto().GetWantlist(), str) {
		t.Fail()
	}
}

func TestNewMessageFromProto(t *testing.T) {
	const str = "a_key"
	protoMessage := new(PBMessage)
	protoMessage.Wantlist = []string{string(str)}
	if !contains(protoMessage.Wantlist, str) {
		t.Fail()
	}
	m, err := newMessageFromProto(*protoMessage)
	if err != nil {
		t.Fatal(err)
	}
	if !contains(m.ToProto().GetWantlist(), str) {
		t.Fail()
	}
}

func TestAppendBlock(t *testing.T) {

	strs := make([]string, 2)
	strs = append(strs, "Celeritas")
	strs = append(strs, "Incendia")

	m := New()
	for _, str := range strs {
		block := testutil.NewBlockOrFail(t, str)
		m.AppendBlock(block)
	}

	// assert strings are in proto message
	for _, blockbytes := range m.ToProto().GetBlocks() {
		s := bytes.NewBuffer(blockbytes).String()
		if !contains(strs, s) {
			t.Fail()
		}
	}
}

func TestWantlist(t *testing.T) {
	keystrs := []string{"foo", "bar", "baz", "bat"}
	m := New()
	for _, s := range keystrs {
		m.AppendWanted(u.Key(s))
	}
	exported := m.Wantlist()

	for _, k := range exported {
		present := false
		for _, s := range keystrs {

			if s == string(k) {
				present = true
			}
		}
		if !present {
			t.Logf("%v isn't in original list", string(k))
			t.Fail()
		}
	}
}

func TestCopyProtoByValue(t *testing.T) {
	const str = "foo"
	m := New()
	protoBeforeAppend := m.ToProto()
	m.AppendWanted(u.Key(str))
	if contains(protoBeforeAppend.GetWantlist(), str) {
		t.Fail()
	}
}

func contains(s []string, x string) bool {
	for _, a := range s {
		if a == x {
			return true
		}
	}
	return false
}
