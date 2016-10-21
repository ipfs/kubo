package filestore

import (
	"testing"
)

func testParse(t *testing.T, str string, expect Key) {
	res := ParseKey(str)
	if res != expect {
		t.Errorf("parse failed on: %s %s", str)
	}
	if str != res.String() {
		t.Errorf("format failed %s != %s", str, res.String())
	}
}

func TestKey(t *testing.T) {
	testParse(t, "Qm45", Key{"Qm45", "", -1})
	testParse(t, "Qm45//dir/file", Key{"Qm45", "dir/file", -1})
	testParse(t, "Qm45//dir/file//", Key{"Qm45", "dir/file//", -1})
	testParse(t, "Qm45//dir/file//23", Key{"Qm45", "dir/file", 23})
	testParse(t, "/ED65SD", Key{"/ED65SD", "", -1})
	testParse(t, "/ED65SD///some/file", Key{"/ED65SD", "/some/file", -1})
	testParse(t, "/ED65SD///some/file//34", Key{"/ED65SD", "/some/file", 34})
	testParse(t, "///some/file", Key{"", "/some/file", -1})
}
