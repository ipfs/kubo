package keystore

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"reflect"
	"sort"
	"strings"
	"testing"

	ci "gx/ipfs/QmfWDLQjGjVe4fr5CoztYW2DYYjRysMJrFe1RCsXLPTf46/go-libp2p-crypto"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/commands"
	"github.com/leanovate/gopter/gen"
)

type rr struct{}

func (rr rr) Read(b []byte) (int, error) {
	return rand.Read(b)
}

func privKeyOrFatal(t *testing.T) ci.PrivKey {
	priv, _, err := ci.GenerateEd25519Key(rr{})
	if err != nil {
		t.Fatal(err)
	}
	return priv
}

func TestKeystoreBasics(t *testing.T) {
	tdir, err := ioutil.TempDir("", "keystore-test")
	if err != nil {
		t.Fatal(err)
	}

	ks, err := NewFSKeystore(tdir)
	if err != nil {
		t.Fatal(err)
	}

	l, err := ks.List()
	if err != nil {
		t.Fatal(err)
	}

	if len(l) != 0 {
		t.Fatal("expected no keys")
	}

	k1 := privKeyOrFatal(t)
	k2 := privKeyOrFatal(t)
	k3 := privKeyOrFatal(t)
	k4 := privKeyOrFatal(t)

	err = ks.Put("foo", k1)
	if err != nil {
		t.Fatal(err)
	}

	err = ks.Put("bar", k2)
	if err != nil {
		t.Fatal(err)
	}

	l, err = ks.List()
	if err != nil {
		t.Fatal(err)
	}

	sort.Strings(l)
	if l[0] != "bar" || l[1] != "foo" {
		t.Fatal("wrong entries listed")
	}

	if err := assertDirContents(tdir, []string{"foo", "bar"}); err != nil {
		t.Fatal(err)
	}

	err = ks.Put("foo", k3)
	if err == nil {
		t.Fatal("should not be able to overwrite key")
	}

	if err := assertDirContents(tdir, []string{"foo", "bar"}); err != nil {
		t.Fatal(err)
	}

	if err := ks.Delete("bar"); err != nil {
		t.Fatal(err)
	}

	if err := assertDirContents(tdir, []string{"foo"}); err != nil {
		t.Fatal(err)
	}

	if err := ks.Put("beep", k3); err != nil {
		t.Fatal(err)
	}

	if err := ks.Put("boop", k4); err != nil {
		t.Fatal(err)
	}

	if err := assertDirContents(tdir, []string{"foo", "beep", "boop"}); err != nil {
		t.Fatal(err)
	}

	if err := assertGetKey(ks, "foo", k1); err != nil {
		t.Fatal(err)
	}

	if err := assertGetKey(ks, "beep", k3); err != nil {
		t.Fatal(err)
	}

	if err := assertGetKey(ks, "boop", k4); err != nil {
		t.Fatal(err)
	}

	if err := ks.Put("..///foo/", k1); err == nil {
		t.Fatal("shouldnt be able to put a poorly named key")
	}

	if err := ks.Put("", k1); err == nil {
		t.Fatal("shouldnt be able to put a key with no name")
	}

	if err := ks.Put(".foo", k1); err == nil {
		t.Fatal("shouldnt be able to put a key with a 'hidden' name")
	}
}

func TestNonExistingKey(t *testing.T) {
	tdir, err := ioutil.TempDir("", "keystore-test")
	if err != nil {
		t.Fatal(err)
	}

	ks, err := NewFSKeystore(tdir)
	if err != nil {
		t.Fatal(err)
	}

	k, err := ks.Get("does-it-exist")
	if err != ErrNoSuchKey {
		t.Fatalf("expected: %s, got %s", ErrNoSuchKey, err)
	}
	if k != nil {
		t.Fatalf("Get on nonexistant key should give nil")
	}
}

func TestMakeKeystoreNoDir(t *testing.T) {
	_, err := NewFSKeystore("/this/is/not/a/real/dir")
	if err == nil {
		t.Fatal("shouldnt be able to make a keystore in a nonexistant directory")
	}
}

func assertGetKey(ks Keystore, name string, exp ci.PrivKey) error {
	out_k, err := ks.Get(name)
	if err != nil {
		return err
	}

	if !out_k.Equals(exp) {
		return fmt.Errorf("key we got out didnt match expectation")
	}

	return nil
}

func assertDirContents(dir string, exp []string) error {
	finfos, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}

	if len(finfos) != len(exp) {
		return fmt.Errorf("Expected %d directory entries", len(exp))
	}

	var names []string
	for _, fi := range finfos {
		names = append(names, fi.Name())
	}

	sort.Strings(names)
	sort.Strings(exp)
	if len(names) != len(exp) {
		return fmt.Errorf("directory had wrong number of entries in it")
	}

	for i, v := range names {
		if v != exp[i] {
			return fmt.Errorf("had wrong entry in directory")
		}
	}
	return nil
}

type fsState struct {
	store map[string]ci.PrivKey
}

func (st *fsState) HasKey(key string) bool {
	_, e := st.store[key]
	return e
}

func (st *fsState) Put(key string, value ci.PrivKey) {
	st.store[key] = value
}

func (st *fsState) Get(key string) ci.PrivKey {
	return st.store[key]
}

func (st *fsState) Del(key string) {
	delete(st.store, key)
}

// Using []interface{} as return type due to gopter accepting interfaces in gen.OneOfConst(...)
func (st *fsState) Keys() []interface{} {
	keys := make([]interface{}, len(st.store))
	i := 0
	for k := range st.store {
		keys[i] = k
		i++
	}
	return keys
}

func ValidKey(key string) bool {
	// XXX: Only disallowing "/" might cause issues on Windows and with weird FSes
	return key != "" && !strings.Contains(key, "/") && !strings.HasPrefix(key, ".")
}

type getCommand string
func (cm *getCommand) Run(store commands.SystemUnderTest) commands.Result {
	val, err := store.(*FSKeystore).Get(string(*cm))
	if err != nil {
		return nil
	}
	hash, err := val.Hash()
	if err != nil {
		return nil
	}
	return hash
}
func (cm *getCommand) PreCondition(state commands.State) bool {
	return state.(*fsState).HasKey(string(*cm)) && ValidKey(string(*cm))
}
func (cm *getCommand) PostCondition(state commands.State, result commands.Result) *gopter.PropResult {
	expected, err := state.(*fsState).Get(string(*cm)).Hash()
	if result != nil && err == nil && reflect.DeepEqual(expected, result.([]byte)) {
		return &gopter.PropResult{Status: gopter.PropTrue}
	}
	return gopter.NewPropResult(false, fmt.Sprintf("result is nil: %v, err is nil: %v", result == nil, err == nil))
}
func (cm *getCommand) String() string {
	return fmt.Sprintf("Get(%q)", string(*cm))
}

func getGen(state commands.State) gopter.Gen {
	return gen.OneConstOf(state.(*fsState).Keys()...).Map(func(v string) getCommand {
		return getCommand(v)
	})
}

type delCommand string
func (cm *delCommand) Run(store commands.SystemUnderTest) commands.Result {
	err := store.(*FSKeystore).Delete(string(*cm))
	if err != nil {
		return false
	}
	return true
}
func (cm *delCommand) PreCondition(state commands.State) bool {
	return state.(*fsState).HasKey(string(*cm)) && ValidKey(string(*cm))
}
func (cm *delCommand) PostCondition(state commands.State, result commands.Result) *gopter.PropResult {
	if result.(bool) {
		return &gopter.PropResult{Status: gopter.PropTrue}
	}
	return &gopter.PropResult{Status: gopter.PropFalse}
}
func (cm *delCommand) String() string {
	return fmt.Sprintf("Del(%q)", string(*cm))
}
func (cm *delCommand) NextState(st commands.State) commands.State {
	st.(*fsState).Del(string(*cm))
	return st
}

func delGen(state commands.State) gopter.Gen {
	return gen.OneConstOf(state.(*fsState).Keys()...).Map(func(v string) delCommand {
		return delCommand(v)
	})
}

type putCommand struct {
	key string
	val ci.PrivKey
}

func (cm *putCommand) Run(store commands.SystemUnderTest) commands.Result {
	err := store.(*FSKeystore).Put(cm.key, cm.val)
	if err != nil {
		return false
	}
	return true
}
func (cm *putCommand) PreCondition(state commands.State) bool {
	return state.(*fsState).HasKey(string(cm.key)) && ValidKey(string(cm.key))
}
func (cm *putCommand) PostCondition(state commands.State, result commands.Result) *gopter.PropResult {
	if result.(bool) {
		return &gopter.PropResult{Status: gopter.PropTrue}
	}
	return &gopter.PropResult{Status: gopter.PropFalse}
}
func (cm *putCommand) String() string {
	h, _ := cm.val.Hash()
	return fmt.Sprintf("Put(%q, %x...)", cm.key, h[:6])
}
func (cm *putCommand) NextState(st commands.State) commands.State {
	st.(*fsState).Put(cm.key, cm.val)
	return st
}

func runesToString(v []rune) string {
	return string(v)
}

func genString(runeGen gopter.Gen, runeSieve func(ch rune) bool) gopter.Gen {
	return gen.SliceOf(runeGen).Map(runesToString).SuchThat(func(v string) bool {
		for _, ch := range v {
			if !runeSieve(ch) {
				return false
			}
		}
		return true
	}).WithShrinker(gen.StringShrinker)
}

func putGen() gopter.Gen {
	return genString(gen.OneGenOf(gen.AlphaLowerChar(), gen.NumChar(), gen.OneConstOf('_', '-')),
		func(_ rune) bool {
			return true
		}).Map(func(v string) *putCommand {
		
		k, _, _ := ci.GenerateEd25519Key(rr{}) // Unfortunately, can't replicate privk related bugs with this
		return &putCommand{
			key: v,
			val: k,
		}
	})
}

var listCommand = &commands.ProtoCommand{
	Name: "List",
	RunFunc: func (store commands.SystemUnderTest) commands.Result {
		list, err := store.(*FSKeystore).List()
		if err != nil {
			return nil
		}
		return list
	},
	PostConditionFunc: func(state commands.State, res commands.Result) *gopter.PropResult {
		if res != nil && len(res.([]string)) == len(state.(*fsState).store) {
			stk := state.(*fsState).Keys()
			// Convert []interface{} to []string
			expected := make([]string, len(stk))
			i := 0
			for _, k := range stk {
				expected[i] = k.(string)
				i++
			}
			sort.Strings(expected)
			actual := res.([]string)
			sort.Strings(actual)
			if reflect.DeepEqual(expected, actual) {
				return &gopter.PropResult{Status: gopter.PropTrue}
			}
			return gopter.NewPropResult(false, "Failed at deep equal");
		}
		return gopter.NewPropResult(false, fmt.Sprintf("Failed at first if, is res nil?: %v", res == nil))
	},
}

var filestoreCommands = &commands.ProtoCommands{
	NewSystemUnderTestFunc: func(initialState commands.State) commands.SystemUnderTest {
		tmp, err := ioutil.TempDir("", "keystore-test")
		if err != nil {
			return nil
		}
		keystore, err := NewFSKeystore(tmp)
		if err != nil {
			return nil
		}
		return keystore
	},
	InitialStateGen: gen.Const(&fsState { store: map[string]ci.PrivKey{} }),
	GenCommandFunc: func(state commands.State) gopter.Gen {
		if len(state.(*fsState).Keys()) == 0 {
			return gen.OneGenOf(putGen(), gen.Const(listCommand))
		}
		return gen.OneGenOf(getGen(state), putGen(), delGen(state), gen.Const(listCommand))
	},
}

func TestFilestoreCommands(t *testing.T) {
	parameters := gopter.DefaultTestParameters()

	properties := gopter.NewProperties(parameters)

	properties.Property("filestore", commands.Prop(filestoreCommands))

	properties.TestingRun(t)
}
