package filestore

import (
	"testing"
)

func testParse(t *testing.T, str string, expect Key) {
	res,err := ParseKey(str)
	if err != nil {
		t.Errorf("%s", err)
	}
	if res.Key != expect {
		t.Errorf("parse failed on: %s: %#v != %#v", str, expect, res.Key)
	}
	if str != res.Format() {
		t.Errorf("Format() format failed %s != %s", str, res.Format())
	}
}

func testDsParse(t *testing.T, str string, expect Key) {
	res := ParseDsKey(str)
	if res != expect {
		t.Errorf("parse failed on: %s", str)
	}
	if str != res.String() {
		t.Errorf("String() format failed %s != %s", str, res.String())
	}
	if str != string(res.Bytes()) {
		t.Errorf("Bytes() format failed %s != %s", str, res.String())
	}
}

func TestKey(t *testing.T) {
	qmHash := "/CIQPJLLZXHBPDKSP325GP7BLB6J3WNGKMDZJWZRGANTAN22QKXDNY6Y"
	zdHash := "/AFZBEIGD4KVH2JPQABBLQGN44DZVK5F3WWBEFEUDWFZ2ANB3PLOXSHWTDY"
	testParse(t, "QmeomcMd37LRxkYn69XKiTpGEiJWRgUNEaxADx6ssfUJhp", Key{qmHash, "", -1})
	testParse(t, "zdvgqEbdrK4PzARFB7twNKangqFF3mgWeuJJAtMUwdDwFq7Pj", Key{zdHash, "", -1})	
	testParse(t, "QmeomcMd37LRxkYn69XKiTpGEiJWRgUNEaxADx6ssfUJhp/dir/file", Key{qmHash, "dir/file", -1})
	testParse(t, "QmeomcMd37LRxkYn69XKiTpGEiJWRgUNEaxADx6ssfUJhp//dir/file", Key{qmHash, "/dir/file", -1})
	testParse(t, "QmeomcMd37LRxkYn69XKiTpGEiJWRgUNEaxADx6ssfUJhp//dir/file//23", Key{qmHash, "/dir/file", 23})
	testParse(t, "//just/a/file", Key{"", "/just/a/file", -1})
	testParse(t, "/just/a/file", Key{"", "just/a/file", -1})

	testDsParse(t, "/ED65SD", Key{"/ED65SD", "", -1})
	testDsParse(t, "/ED65SD//some/file", Key{"/ED65SD", "/some/file", -1})
	testDsParse(t, "/ED65SD//some/file//34", Key{"/ED65SD", "/some/file", 34})
	testDsParse(t, "/ED65SD/c:/some/file//34", Key{"/ED65SD", "c:/some/file", 34})
}
