package fs_test

import (
	"bytes"
	"errors"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/bazil.org/fuse"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/bazil.org/fuse/fs"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/bazil.org/fuse/fs/fstestutil"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/bazil.org/fuse/fs/fstestutil/record"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/bazil.org/fuse/fuseutil"
	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/bazil.org/fuse/syscallx"
)

// TO TEST:
//	Lookup(*LookupRequest, *LookupResponse)
//	Getattr(*GetattrRequest, *GetattrResponse)
//	Attr with explicit inode
//	Setattr(*SetattrRequest, *SetattrResponse)
//	Access(*AccessRequest)
//	Open(*OpenRequest, *OpenResponse)
//	Write(*WriteRequest, *WriteResponse)
//	Flush(*FlushRequest, *FlushResponse)

func init() {
	fstestutil.DebugByDefault()
}

var childMode bool

func init() {
	flag.BoolVar(&childMode, "fuse.internal.childmode", false, "internal use only")
}

// childCmd prepares a test function to be run in a subprocess, with
// childMode set to true. Caller must still call Run or Start.
//
// Re-using the test executable as the subprocess is useful because
// now test executables can e.g. be cross-compiled, transferred
// between hosts, and run in settings where the whole Go development
// environment is not installed.
func childCmd(testName string) (*exec.Cmd, error) {
	// caller may set cwd, so we can't rely on relative paths
	executable, err := filepath.Abs(os.Args[0])
	if err != nil {
		return nil, err
	}
	testName = regexp.QuoteMeta(testName)
	cmd := exec.Command(executable, "-test.run=^"+testName+"$", "-fuse.internal.childmode")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd, nil
}

// childMapFS is an FS with one fixed child named "child".
type childMapFS map[string]fs.Node

var _ = fs.FS(childMapFS{})
var _ = fs.Node(childMapFS{})
var _ = fs.NodeStringLookuper(childMapFS{})

func (f childMapFS) Attr() fuse.Attr {
	return fuse.Attr{Inode: 1, Mode: os.ModeDir | 0777}
}

func (f childMapFS) Root() (fs.Node, fuse.Error) {
	return f, nil
}

func (f childMapFS) Lookup(name string, intr fs.Intr) (fs.Node, fuse.Error) {
	child, ok := f[name]
	if !ok {
		return nil, fuse.ENOENT
	}
	return child, nil
}

// simpleFS is a trivial FS that just implements the Root method.
type simpleFS struct {
	node fs.Node
}

var _ = fs.FS(simpleFS{})

func (f simpleFS) Root() (fs.Node, fuse.Error) {
	return f.node, nil
}

// file can be embedded in a struct to make it look like a file.
type file struct{}

func (f file) Attr() fuse.Attr { return fuse.Attr{Mode: 0666} }

// dir can be embedded in a struct to make it look like a directory.
type dir struct{}

func (f dir) Attr() fuse.Attr { return fuse.Attr{Mode: os.ModeDir | 0777} }

// symlink can be embedded in a struct to make it look like a symlink.
type symlink struct {
	target string
}

func (f symlink) Attr() fuse.Attr { return fuse.Attr{Mode: os.ModeSymlink | 0666} }

// fifo can be embedded in a struct to make it look like a named pipe.
type fifo struct{}

func (f fifo) Attr() fuse.Attr { return fuse.Attr{Mode: os.ModeNamedPipe | 0666} }

type badRootFS struct{}

func (badRootFS) Root() (fs.Node, fuse.Error) {
	// pick a really distinct error, to identify it later
	return nil, fuse.Errno(syscall.ENAMETOOLONG)
}

func TestRootErr(t *testing.T) {
	t.Parallel()
	mnt, err := fstestutil.MountedT(t, badRootFS{})
	if err == nil {
		// path for synchronous mounts (linux): started out fine, now
		// wait for Serve to cycle through
		err = <-mnt.Error
		// without this, unmount will keep failing with EBUSY; nudge
		// kernel into realizing InitResponse will not happen
		mnt.Conn.Close()
		mnt.Close()
	}

	if err == nil {
		t.Fatal("expected an error")
	}
	// TODO this should not be a textual comparison, Serve hides
	// details
	if err.Error() != "cannot obtain root node: file name too long" {
		t.Errorf("Unexpected error: %v", err)
	}
}

type testStatFS struct{}

func (f testStatFS) Root() (fs.Node, fuse.Error) {
	return f, nil
}

func (f testStatFS) Attr() fuse.Attr {
	return fuse.Attr{Inode: 1, Mode: os.ModeDir | 0777}
}

func (f testStatFS) Statfs(req *fuse.StatfsRequest, resp *fuse.StatfsResponse, int fs.Intr) fuse.Error {
	resp.Blocks = 42
	resp.Files = 13
	return nil
}

func TestStatfs(t *testing.T) {
	t.Parallel()
	mnt, err := fstestutil.MountedT(t, testStatFS{})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	{
		var st syscall.Statfs_t
		err = syscall.Statfs(mnt.Dir, &st)
		if err != nil {
			t.Errorf("Statfs failed: %v", err)
		}
		t.Logf("Statfs got: %#v", st)
		if g, e := st.Blocks, uint64(42); g != e {
			t.Errorf("got Blocks = %d; want %d", g, e)
		}
		if g, e := st.Files, uint64(13); g != e {
			t.Errorf("got Files = %d; want %d", g, e)
		}
	}

	{
		var st syscall.Statfs_t
		f, err := os.Open(mnt.Dir)
		if err != nil {
			t.Errorf("Open for fstatfs failed: %v", err)
		}
		defer f.Close()
		err = syscall.Fstatfs(int(f.Fd()), &st)
		if err != nil {
			t.Errorf("Fstatfs failed: %v", err)
		}
		t.Logf("Fstatfs got: %#v", st)
		if g, e := st.Blocks, uint64(42); g != e {
			t.Errorf("got Blocks = %d; want %d", g, e)
		}
		if g, e := st.Files, uint64(13); g != e {
			t.Errorf("got Files = %d; want %d", g, e)
		}
	}

}

// Test Stat of root.

type root struct{}

func (f root) Root() (fs.Node, fuse.Error) {
	return f, nil
}

func (root) Attr() fuse.Attr {
	return fuse.Attr{Inode: 1, Mode: os.ModeDir | 0555}
}

func TestStatRoot(t *testing.T) {
	t.Parallel()
	mnt, err := fstestutil.MountedT(t, root{})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	fi, err := os.Stat(mnt.Dir)
	if err != nil {
		t.Fatalf("root getattr failed with %v", err)
	}
	mode := fi.Mode()
	if (mode & os.ModeType) != os.ModeDir {
		t.Errorf("root is not a directory: %#v", fi)
	}
	if mode.Perm() != 0555 {
		t.Errorf("root has weird access mode: %v", mode.Perm())
	}
	switch stat := fi.Sys().(type) {
	case *syscall.Stat_t:
		if stat.Ino != 1 {
			t.Errorf("root has wrong inode: %v", stat.Ino)
		}
		if stat.Nlink != 1 {
			t.Errorf("root has wrong link count: %v", stat.Nlink)
		}
		if stat.Uid != 0 {
			t.Errorf("root has wrong uid: %d", stat.Uid)
		}
		if stat.Gid != 0 {
			t.Errorf("root has wrong gid: %d", stat.Gid)
		}
	}
}

// Test Read calling ReadAll.

type readAll struct{ file }

const hi = "hello, world"

func (readAll) Attr() fuse.Attr {
	return fuse.Attr{
		Mode: 0666,
		Size: uint64(len(hi)),
	}
}

func (readAll) ReadAll(intr fs.Intr) ([]byte, fuse.Error) {
	return []byte(hi), nil
}

func testReadAll(t *testing.T, path string) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("readAll: %v", err)
	}
	if string(data) != hi {
		t.Errorf("readAll = %q, want %q", data, hi)
	}
}

func TestReadAll(t *testing.T) {
	t.Parallel()
	mnt, err := fstestutil.MountedT(t, childMapFS{"child": readAll{}})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	testReadAll(t, mnt.Dir+"/child")
}

// Test Read.

type readWithHandleRead struct{ file }

func (readWithHandleRead) Attr() fuse.Attr {
	return fuse.Attr{
		Mode: 0666,
		Size: uint64(len(hi)),
	}
}

func (readWithHandleRead) Read(req *fuse.ReadRequest, resp *fuse.ReadResponse, intr fs.Intr) fuse.Error {
	fuseutil.HandleRead(req, resp, []byte(hi))
	return nil
}

func TestReadAllWithHandleRead(t *testing.T) {
	t.Parallel()
	mnt, err := fstestutil.MountedT(t, childMapFS{"child": readWithHandleRead{}})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	testReadAll(t, mnt.Dir+"/child")
}

// Test Release.

type release struct {
	file
	record.ReleaseWaiter
}

func TestRelease(t *testing.T) {
	t.Parallel()
	r := &release{}
	mnt, err := fstestutil.MountedT(t, childMapFS{"child": r})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	f, err := os.Open(mnt.Dir + "/child")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if !r.WaitForRelease(1 * time.Second) {
		t.Error("Close did not Release in time")
	}
}

// Test Write calling basic Write, with an fsync thrown in too.

type write struct {
	file
	record.Writes
	record.Fsyncs
}

func TestWrite(t *testing.T) {
	t.Parallel()
	w := &write{}
	mnt, err := fstestutil.MountedT(t, childMapFS{"child": w})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	f, err := os.Create(mnt.Dir + "/child")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer f.Close()
	n, err := f.Write([]byte(hi))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(hi) {
		t.Fatalf("short write; n=%d; hi=%d", n, len(hi))
	}

	err = syscall.Fsync(int(f.Fd()))
	if err != nil {
		t.Fatalf("Fsync = %v", err)
	}
	if w.RecordedFsync() == (fuse.FsyncRequest{}) {
		t.Errorf("never received expected fsync call")
	}

	err = f.Close()
	if err != nil {
		t.Fatalf("Close: %v", err)
	}

	if got := string(w.RecordedWriteData()); got != hi {
		t.Errorf("write = %q, want %q", got, hi)
	}
}

// Test Write of a larger buffer.

type writeLarge struct {
	file
	record.Writes
}

func TestWriteLarge(t *testing.T) {
	t.Parallel()
	w := &write{}
	mnt, err := fstestutil.MountedT(t, childMapFS{"child": w})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	f, err := os.Create(mnt.Dir + "/child")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	defer f.Close()
	const one = "xyzzyfoo"
	large := bytes.Repeat([]byte(one), 8192)
	n, err := f.Write(large)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if g, e := n, len(large); g != e {
		t.Fatalf("short write: %d != %d", g, e)
	}

	err = f.Close()
	if err != nil {
		t.Fatalf("Close: %v", err)
	}

	got := w.RecordedWriteData()
	if g, e := len(got), len(large); g != e {
		t.Errorf("write wrong length: %d != %d", g, e)
	}
	if g := strings.Replace(string(got), one, "", -1); g != "" {
		t.Errorf("write wrong data: expected repeats of %q, also got %q", one, g)
	}
}

// Test Write calling Setattr+Write+Flush.

type writeTruncateFlush struct {
	file
	record.Writes
	record.Setattrs
	record.Flushes
}

func TestWriteTruncateFlush(t *testing.T) {
	t.Parallel()
	w := &writeTruncateFlush{}
	mnt, err := fstestutil.MountedT(t, childMapFS{"child": w})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	err = ioutil.WriteFile(mnt.Dir+"/child", []byte(hi), 0666)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if w.RecordedSetattr() == (fuse.SetattrRequest{}) {
		t.Errorf("writeTruncateFlush expected Setattr")
	}
	if !w.RecordedFlush() {
		t.Errorf("writeTruncateFlush expected Setattr")
	}
	if got := string(w.RecordedWriteData()); got != hi {
		t.Errorf("writeTruncateFlush = %q, want %q", got, hi)
	}
}

// Test Mkdir.

type mkdir1 struct {
	dir
	record.Mkdirs
}

func (f *mkdir1) Mkdir(req *fuse.MkdirRequest, intr fs.Intr) (fs.Node, fuse.Error) {
	f.Mkdirs.Mkdir(req, intr)
	return &mkdir1{}, nil
}

func TestMkdir(t *testing.T) {
	t.Parallel()
	f := &mkdir1{}
	mnt, err := fstestutil.MountedT(t, simpleFS{f})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	// uniform umask needed to make os.Mkdir's mode into something
	// reproducible
	defer syscall.Umask(syscall.Umask(0022))
	err = os.Mkdir(mnt.Dir+"/foo", 0771)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	want := fuse.MkdirRequest{Name: "foo", Mode: os.ModeDir | 0751}
	if g, e := f.RecordedMkdir(), want; g != e {
		t.Errorf("mkdir saw %+v, want %+v", g, e)
	}
}

// Test Create (and fsync)

type create1file struct {
	file
	record.Fsyncs
}

type create1 struct {
	dir
	f create1file
}

func (f *create1) Create(req *fuse.CreateRequest, resp *fuse.CreateResponse, intr fs.Intr) (fs.Node, fs.Handle, fuse.Error) {
	if req.Name != "foo" {
		log.Printf("ERROR create1.Create unexpected name: %q\n", req.Name)
		return nil, nil, fuse.EPERM
	}
	flags := req.Flags

	// OS X does not pass O_TRUNC here, Linux does; as this is a
	// Create, that's acceptable
	flags &^= fuse.OpenTruncate

	if runtime.GOOS == "linux" {
		// Linux <3.7 accidentally leaks O_CLOEXEC through to FUSE;
		// avoid spurious test failures
		flags &^= fuse.OpenFlags(syscall.O_CLOEXEC)
	}

	if g, e := flags, fuse.OpenReadWrite|fuse.OpenCreate; g != e {
		log.Printf("ERROR create1.Create unexpected flags: %v != %v\n", g, e)
		return nil, nil, fuse.EPERM
	}
	if g, e := req.Mode, os.FileMode(0644); g != e {
		log.Printf("ERROR create1.Create unexpected mode: %v != %v\n", g, e)
		return nil, nil, fuse.EPERM
	}
	return &f.f, &f.f, nil
}

func TestCreate(t *testing.T) {
	t.Parallel()
	f := &create1{}
	mnt, err := fstestutil.MountedT(t, simpleFS{f})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	// uniform umask needed to make os.Create's 0666 into something
	// reproducible
	defer syscall.Umask(syscall.Umask(0022))
	ff, err := os.Create(mnt.Dir + "/foo")
	if err != nil {
		t.Fatalf("create1 WriteFile: %v", err)
	}
	defer ff.Close()

	err = syscall.Fsync(int(ff.Fd()))
	if err != nil {
		t.Fatalf("Fsync = %v", err)
	}

	if f.f.RecordedFsync() == (fuse.FsyncRequest{}) {
		t.Errorf("never received expected fsync call")
	}

	ff.Close()
}

// Test Create + Write + Remove

type create3file struct {
	file
	record.Writes
}

type create3 struct {
	dir
	f          create3file
	fooCreated record.MarkRecorder
	fooRemoved record.MarkRecorder
}

func (f *create3) Create(req *fuse.CreateRequest, resp *fuse.CreateResponse, intr fs.Intr) (fs.Node, fs.Handle, fuse.Error) {
	if req.Name != "foo" {
		log.Printf("ERROR create3.Create unexpected name: %q\n", req.Name)
		return nil, nil, fuse.EPERM
	}
	f.fooCreated.Mark()
	return &f.f, &f.f, nil
}

func (f *create3) Lookup(name string, intr fs.Intr) (fs.Node, fuse.Error) {
	if f.fooCreated.Recorded() && !f.fooRemoved.Recorded() && name == "foo" {
		return &f.f, nil
	}
	return nil, fuse.ENOENT
}

func (f *create3) Remove(r *fuse.RemoveRequest, intr fs.Intr) fuse.Error {
	if f.fooCreated.Recorded() && !f.fooRemoved.Recorded() &&
		r.Name == "foo" && !r.Dir {
		f.fooRemoved.Mark()
		return nil
	}
	return fuse.ENOENT
}

func TestCreateWriteRemove(t *testing.T) {
	t.Parallel()
	f := &create3{}
	mnt, err := fstestutil.MountedT(t, simpleFS{f})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	err = ioutil.WriteFile(mnt.Dir+"/foo", []byte(hi), 0666)
	if err != nil {
		t.Fatalf("create3 WriteFile: %v", err)
	}
	if got := string(f.f.RecordedWriteData()); got != hi {
		t.Fatalf("create3 write = %q, want %q", got, hi)
	}

	err = os.Remove(mnt.Dir + "/foo")
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	err = os.Remove(mnt.Dir + "/foo")
	if err == nil {
		t.Fatalf("second Remove = nil; want some error")
	}
}

// Test symlink + readlink

// is a Node that is a symlink to target
type symlink1link struct {
	symlink
	target string
}

func (f symlink1link) Readlink(*fuse.ReadlinkRequest, fs.Intr) (string, fuse.Error) {
	return f.target, nil
}

type symlink1 struct {
	dir
	record.Symlinks
}

func (f *symlink1) Symlink(req *fuse.SymlinkRequest, intr fs.Intr) (fs.Node, fuse.Error) {
	f.Symlinks.Symlink(req, intr)
	return symlink1link{target: req.Target}, nil
}

func TestSymlink(t *testing.T) {
	t.Parallel()
	f := &symlink1{}
	mnt, err := fstestutil.MountedT(t, simpleFS{f})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	const target = "/some-target"

	err = os.Symlink(target, mnt.Dir+"/symlink.file")
	if err != nil {
		t.Fatalf("os.Symlink: %v", err)
	}

	want := fuse.SymlinkRequest{NewName: "symlink.file", Target: target}
	if g, e := f.RecordedSymlink(), want; g != e {
		t.Errorf("symlink saw %+v, want %+v", g, e)
	}

	gotName, err := os.Readlink(mnt.Dir + "/symlink.file")
	if err != nil {
		t.Fatalf("os.Readlink: %v", err)
	}
	if gotName != target {
		t.Errorf("os.Readlink = %q; want %q", gotName, target)
	}
}

// Test link

type link1 struct {
	dir
	record.Links
}

func (f *link1) Lookup(name string, intr fs.Intr) (fs.Node, fuse.Error) {
	if name == "old" {
		return file{}, nil
	}
	return nil, fuse.ENOENT
}

func (f *link1) Link(r *fuse.LinkRequest, old fs.Node, intr fs.Intr) (fs.Node, fuse.Error) {
	f.Links.Link(r, old, intr)
	return file{}, nil
}

func TestLink(t *testing.T) {
	t.Parallel()
	f := &link1{}
	mnt, err := fstestutil.MountedT(t, simpleFS{f})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	err = os.Link(mnt.Dir+"/old", mnt.Dir+"/new")
	if err != nil {
		t.Fatalf("Link: %v", err)
	}

	got := f.RecordedLink()
	want := fuse.LinkRequest{
		NewName: "new",
		// unpredictable
		OldNode: got.OldNode,
	}
	if g, e := got, want; g != e {
		t.Fatalf("link saw %+v, want %+v", g, e)
	}
}

// Test Rename

type rename1 struct {
	dir
	renamed record.Counter
}

func (f *rename1) Lookup(name string, intr fs.Intr) (fs.Node, fuse.Error) {
	if name == "old" {
		return file{}, nil
	}
	return nil, fuse.ENOENT
}

func (f *rename1) Rename(r *fuse.RenameRequest, newDir fs.Node, intr fs.Intr) fuse.Error {
	if r.OldName == "old" && r.NewName == "new" && newDir == f {
		f.renamed.Inc()
		return nil
	}
	return fuse.EIO
}

func TestRename(t *testing.T) {
	t.Parallel()
	f := &rename1{}
	mnt, err := fstestutil.MountedT(t, simpleFS{f})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	err = os.Rename(mnt.Dir+"/old", mnt.Dir+"/new")
	if err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if g, e := f.renamed.Count(), uint32(1); g != e {
		t.Fatalf("expected rename didn't happen: %d != %d", g, e)
	}
	err = os.Rename(mnt.Dir+"/old2", mnt.Dir+"/new2")
	if err == nil {
		t.Fatal("expected error on second Rename; got nil")
	}
}

// Test mknod

type mknod1 struct {
	dir
	record.Mknods
}

func (f *mknod1) Mknod(r *fuse.MknodRequest, intr fs.Intr) (fs.Node, fuse.Error) {
	f.Mknods.Mknod(r, intr)
	return fifo{}, nil
}

func TestMknod(t *testing.T) {
	t.Parallel()
	if os.Getuid() != 0 {
		t.Skip("skipping unless root")
	}

	f := &mknod1{}
	mnt, err := fstestutil.MountedT(t, simpleFS{f})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	defer syscall.Umask(syscall.Umask(0))
	err = syscall.Mknod(mnt.Dir+"/node", syscall.S_IFIFO|0666, 123)
	if err != nil {
		t.Fatalf("Mknod: %v", err)
	}

	want := fuse.MknodRequest{
		Name: "node",
		Mode: os.FileMode(os.ModeNamedPipe | 0666),
		Rdev: uint32(123),
	}
	if runtime.GOOS == "linux" {
		// Linux fuse doesn't echo back the rdev if the node
		// isn't a device (we're using a FIFO here, as that
		// bit is portable.)
		want.Rdev = 0
	}
	if g, e := f.RecordedMknod(), want; g != e {
		t.Fatalf("mknod saw %+v, want %+v", g, e)
	}
}

// Test Read served with DataHandle.

type dataHandleTest struct {
	file
}

func (dataHandleTest) Attr() fuse.Attr {
	return fuse.Attr{
		Mode: 0666,
		Size: uint64(len(hi)),
	}
}

func (dataHandleTest) Open(*fuse.OpenRequest, *fuse.OpenResponse, fs.Intr) (fs.Handle, fuse.Error) {
	return fs.DataHandle([]byte(hi)), nil
}

func TestDataHandle(t *testing.T) {
	t.Parallel()
	f := &dataHandleTest{}
	mnt, err := fstestutil.MountedT(t, childMapFS{"child": f})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	data, err := ioutil.ReadFile(mnt.Dir + "/child")
	if err != nil {
		t.Errorf("readAll: %v", err)
		return
	}
	if string(data) != hi {
		t.Errorf("readAll = %q, want %q", data, hi)
	}
}

// Test interrupt

type interrupt struct {
	file

	// strobes to signal we have a read hanging
	hanging chan struct{}
}

func (interrupt) Attr() fuse.Attr {
	return fuse.Attr{
		Mode: 0666,
		Size: 1,
	}
}

func (it *interrupt) Read(req *fuse.ReadRequest, resp *fuse.ReadResponse, intr fs.Intr) fuse.Error {
	select {
	case it.hanging <- struct{}{}:
	default:
	}
	<-intr
	return fuse.EINTR
}

func TestInterrupt(t *testing.T) {
	t.Parallel()
	f := &interrupt{}
	f.hanging = make(chan struct{}, 1)
	mnt, err := fstestutil.MountedT(t, childMapFS{"child": f})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	// start a subprocess that can hang until signaled
	cmd := exec.Command("cat", mnt.Dir+"/child")

	err = cmd.Start()
	if err != nil {
		t.Errorf("interrupt: cannot start cat: %v", err)
		return
	}

	// try to clean up if child is still alive when returning
	defer cmd.Process.Kill()

	// wait till we're sure it's hanging in read
	<-f.hanging

	err = cmd.Process.Signal(os.Interrupt)
	if err != nil {
		t.Errorf("interrupt: cannot interrupt cat: %v", err)
		return
	}

	p, err := cmd.Process.Wait()
	if err != nil {
		t.Errorf("interrupt: cat bork: %v", err)
		return
	}
	switch ws := p.Sys().(type) {
	case syscall.WaitStatus:
		if ws.CoreDump() {
			t.Errorf("interrupt: didn't expect cat to dump core: %v", ws)
		}

		if ws.Exited() {
			t.Errorf("interrupt: didn't expect cat to exit normally: %v", ws)
		}

		if !ws.Signaled() {
			t.Errorf("interrupt: expected cat to get a signal: %v", ws)
		} else {
			if ws.Signal() != os.Interrupt {
				t.Errorf("interrupt: cat got wrong signal: %v", ws)
			}
		}
	default:
		t.Logf("interrupt: this platform has no test coverage")
	}
}

// Test truncate

type truncate struct {
	file
	record.Setattrs
}

func testTruncate(t *testing.T, toSize int64) {
	t.Parallel()
	f := &truncate{}
	mnt, err := fstestutil.MountedT(t, childMapFS{"child": f})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	err = os.Truncate(mnt.Dir+"/child", toSize)
	if err != nil {
		t.Fatalf("Truncate: %v", err)
	}
	gotr := f.RecordedSetattr()
	if gotr == (fuse.SetattrRequest{}) {
		t.Fatalf("no recorded SetattrRequest")
	}
	if g, e := gotr.Size, uint64(toSize); g != e {
		t.Errorf("got Size = %q; want %q", g, e)
	}
	if g, e := gotr.Valid&^fuse.SetattrLockOwner, fuse.SetattrSize; g != e {
		t.Errorf("got Valid = %q; want %q", g, e)
	}
	t.Logf("Got request: %#v", gotr)
}

func TestTruncate42(t *testing.T) {
	testTruncate(t, 42)
}

func TestTruncate0(t *testing.T) {
	testTruncate(t, 0)
}

// Test ftruncate

type ftruncate struct {
	file
	record.Setattrs
}

func testFtruncate(t *testing.T, toSize int64) {
	t.Parallel()
	f := &ftruncate{}
	mnt, err := fstestutil.MountedT(t, childMapFS{"child": f})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	{
		fil, err := os.OpenFile(mnt.Dir+"/child", os.O_WRONLY, 0666)
		if err != nil {
			t.Error(err)
			return
		}
		defer fil.Close()

		err = fil.Truncate(toSize)
		if err != nil {
			t.Fatalf("Ftruncate: %v", err)
		}
	}
	gotr := f.RecordedSetattr()
	if gotr == (fuse.SetattrRequest{}) {
		t.Fatalf("no recorded SetattrRequest")
	}
	if g, e := gotr.Size, uint64(toSize); g != e {
		t.Errorf("got Size = %q; want %q", g, e)
	}
	if g, e := gotr.Valid&^fuse.SetattrLockOwner, fuse.SetattrHandle|fuse.SetattrSize; g != e {
		t.Errorf("got Valid = %q; want %q", g, e)
	}
	t.Logf("Got request: %#v", gotr)
}

func TestFtruncate42(t *testing.T) {
	testFtruncate(t, 42)
}

func TestFtruncate0(t *testing.T) {
	testFtruncate(t, 0)
}

// Test opening existing file truncates

type truncateWithOpen struct {
	file
	record.Setattrs
}

func TestTruncateWithOpen(t *testing.T) {
	t.Parallel()
	f := &truncateWithOpen{}
	mnt, err := fstestutil.MountedT(t, childMapFS{"child": f})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	fil, err := os.OpenFile(mnt.Dir+"/child", os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		t.Error(err)
		return
	}
	fil.Close()

	gotr := f.RecordedSetattr()
	if gotr == (fuse.SetattrRequest{}) {
		t.Fatalf("no recorded SetattrRequest")
	}
	if g, e := gotr.Size, uint64(0); g != e {
		t.Errorf("got Size = %q; want %q", g, e)
	}
	// osxfuse sets SetattrHandle here, linux does not
	if g, e := gotr.Valid&^(fuse.SetattrLockOwner|fuse.SetattrHandle), fuse.SetattrSize; g != e {
		t.Errorf("got Valid = %q; want %q", g, e)
	}
	t.Logf("Got request: %#v", gotr)
}

// Test readdir

type readdir struct {
	dir
}

func (d *readdir) ReadDir(intr fs.Intr) ([]fuse.Dirent, fuse.Error) {
	return []fuse.Dirent{
		{Name: "one", Inode: 11, Type: fuse.DT_Dir},
		{Name: "three", Inode: 13},
		{Name: "two", Inode: 12, Type: fuse.DT_File},
	}, nil
}

func TestReadDir(t *testing.T) {
	t.Parallel()
	f := &readdir{}
	mnt, err := fstestutil.MountedT(t, simpleFS{f})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	fil, err := os.Open(mnt.Dir)
	if err != nil {
		t.Error(err)
		return
	}
	defer fil.Close()

	// go Readdir is just Readdirnames + Lstat, there's no point in
	// testing that here; we have no consumption API for the real
	// dirent data
	names, err := fil.Readdirnames(100)
	if err != nil {
		t.Error(err)
		return
	}

	t.Logf("Got readdir: %q", names)

	if len(names) != 3 ||
		names[0] != "one" ||
		names[1] != "three" ||
		names[2] != "two" {
		t.Errorf(`expected 3 entries of "one", "three", "two", got: %q`, names)
		return
	}
}

// Test Chmod.

type chmod struct {
	file
	record.Setattrs
}

func (f *chmod) Setattr(req *fuse.SetattrRequest, resp *fuse.SetattrResponse, intr fs.Intr) fuse.Error {
	if !req.Valid.Mode() {
		log.Printf("setattr not a chmod: %v", req.Valid)
		return fuse.EIO
	}
	f.Setattrs.Setattr(req, resp, intr)
	return nil
}

func TestChmod(t *testing.T) {
	t.Parallel()
	f := &chmod{}
	mnt, err := fstestutil.MountedT(t, childMapFS{"child": f})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	err = os.Chmod(mnt.Dir+"/child", 0764)
	if err != nil {
		t.Errorf("chmod: %v", err)
		return
	}
	got := f.RecordedSetattr()
	if g, e := got.Mode, os.FileMode(0764); g != e {
		t.Errorf("wrong mode: %v != %v", g, e)
	}
}

// Test open

type open struct {
	file
	record.Opens
}

func (f *open) Open(req *fuse.OpenRequest, resp *fuse.OpenResponse, intr fs.Intr) (fs.Handle, fuse.Error) {
	f.Opens.Open(req, resp, intr)
	// pick a really distinct error, to identify it later
	return nil, fuse.Errno(syscall.ENAMETOOLONG)

}

func TestOpen(t *testing.T) {
	t.Parallel()
	f := &open{}
	mnt, err := fstestutil.MountedT(t, childMapFS{"child": f})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	// node: mode only matters with O_CREATE
	fil, err := os.OpenFile(mnt.Dir+"/child", os.O_WRONLY|os.O_APPEND, 0)
	if err == nil {
		t.Error("Open err == nil, expected ENAMETOOLONG")
		fil.Close()
		return
	}

	switch err2 := err.(type) {
	case *os.PathError:
		if err2.Err == syscall.ENAMETOOLONG {
			break
		}
		t.Errorf("unexpected inner error: %#v", err2)
	default:
		t.Errorf("unexpected error: %v", err)
	}

	want := fuse.OpenRequest{Dir: false, Flags: fuse.OpenWriteOnly | fuse.OpenAppend}
	if runtime.GOOS == "darwin" {
		// osxfuse does not let O_APPEND through at all
		//
		// https://code.google.com/p/macfuse/issues/detail?id=233
		// https://code.google.com/p/macfuse/issues/detail?id=132
		// https://code.google.com/p/macfuse/issues/detail?id=133
		want.Flags &^= fuse.OpenAppend
	}
	got := f.RecordedOpen()

	if runtime.GOOS == "linux" {
		// Linux <3.7 accidentally leaks O_CLOEXEC through to FUSE;
		// avoid spurious test failures
		got.Flags &^= fuse.OpenFlags(syscall.O_CLOEXEC)
	}

	if g, e := got, want; g != e {
		t.Errorf("open saw %v, want %v", g, e)
		return
	}
}

// Test Fsync on a dir

type fsyncDir struct {
	dir
	record.Fsyncs
}

func TestFsyncDir(t *testing.T) {
	t.Parallel()
	f := &fsyncDir{}
	mnt, err := fstestutil.MountedT(t, simpleFS{f})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	fil, err := os.Open(mnt.Dir)
	if err != nil {
		t.Errorf("fsyncDir open: %v", err)
		return
	}
	defer fil.Close()
	err = fil.Sync()
	if err != nil {
		t.Errorf("fsyncDir sync: %v", err)
		return
	}

	got := f.RecordedFsync()
	want := fuse.FsyncRequest{
		Flags: 0,
		Dir:   true,
		// unpredictable
		Handle: got.Handle,
	}
	if runtime.GOOS == "darwin" {
		// TODO document the meaning of these flags, figure out why
		// they differ
		want.Flags = 1
	}
	if g, e := got, want; g != e {
		t.Fatalf("fsyncDir saw %+v, want %+v", g, e)
	}
}

// Test Getxattr

type getxattr struct {
	file
	record.Getxattrs
}

func (f *getxattr) Getxattr(req *fuse.GetxattrRequest, resp *fuse.GetxattrResponse, intr fs.Intr) fuse.Error {
	f.Getxattrs.Getxattr(req, resp, intr)
	resp.Xattr = []byte("hello, world")
	return nil
}

func TestGetxattr(t *testing.T) {
	t.Parallel()
	f := &getxattr{}
	mnt, err := fstestutil.MountedT(t, childMapFS{"child": f})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	buf := make([]byte, 8192)
	n, err := syscallx.Getxattr(mnt.Dir+"/child", "not-there", buf)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	buf = buf[:n]
	if g, e := string(buf), "hello, world"; g != e {
		t.Errorf("wrong getxattr content: %#v != %#v", g, e)
	}
	seen := f.RecordedGetxattr()
	if g, e := seen.Name, "not-there"; g != e {
		t.Errorf("wrong getxattr name: %#v != %#v", g, e)
	}
}

// Test Getxattr that has no space to return value

type getxattrTooSmall struct {
	file
}

func (f *getxattrTooSmall) Getxattr(req *fuse.GetxattrRequest, resp *fuse.GetxattrResponse, intr fs.Intr) fuse.Error {
	resp.Xattr = []byte("hello, world")
	return nil
}

func TestGetxattrTooSmall(t *testing.T) {
	t.Parallel()
	f := &getxattrTooSmall{}
	mnt, err := fstestutil.MountedT(t, childMapFS{"child": f})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	buf := make([]byte, 3)
	_, err = syscallx.Getxattr(mnt.Dir+"/child", "whatever", buf)
	if err == nil {
		t.Error("Getxattr = nil; want some error")
	}
	if err != syscall.ERANGE {
		t.Errorf("unexpected error: %v", err)
		return
	}
}

// Test Getxattr used to probe result size

type getxattrSize struct {
	file
}

func (f *getxattrSize) Getxattr(req *fuse.GetxattrRequest, resp *fuse.GetxattrResponse, intr fs.Intr) fuse.Error {
	resp.Xattr = []byte("hello, world")
	return nil
}

func TestGetxattrSize(t *testing.T) {
	t.Parallel()
	f := &getxattrSize{}
	mnt, err := fstestutil.MountedT(t, childMapFS{"child": f})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	n, err := syscallx.Getxattr(mnt.Dir+"/child", "whatever", nil)
	if err != nil {
		t.Errorf("Getxattr unexpected error: %v", err)
		return
	}
	if g, e := n, len("hello, world"); g != e {
		t.Errorf("Getxattr incorrect size: %d != %d", g, e)
	}
}

// Test Listxattr

type listxattr struct {
	file
	record.Listxattrs
}

func (f *listxattr) Listxattr(req *fuse.ListxattrRequest, resp *fuse.ListxattrResponse, intr fs.Intr) fuse.Error {
	f.Listxattrs.Listxattr(req, resp, intr)
	resp.Append("one", "two")
	return nil
}

func TestListxattr(t *testing.T) {
	t.Parallel()
	f := &listxattr{}
	mnt, err := fstestutil.MountedT(t, childMapFS{"child": f})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	buf := make([]byte, 8192)
	n, err := syscallx.Listxattr(mnt.Dir+"/child", buf)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}
	buf = buf[:n]
	if g, e := string(buf), "one\x00two\x00"; g != e {
		t.Errorf("wrong listxattr content: %#v != %#v", g, e)
	}

	want := fuse.ListxattrRequest{
		Size: 8192,
	}
	if g, e := f.RecordedListxattr(), want; g != e {
		t.Fatalf("listxattr saw %+v, want %+v", g, e)
	}
}

// Test Listxattr that has no space to return value

type listxattrTooSmall struct {
	file
}

func (f *listxattrTooSmall) Listxattr(req *fuse.ListxattrRequest, resp *fuse.ListxattrResponse, intr fs.Intr) fuse.Error {
	resp.Xattr = []byte("one\x00two\x00")
	return nil
}

func TestListxattrTooSmall(t *testing.T) {
	t.Parallel()
	f := &listxattrTooSmall{}
	mnt, err := fstestutil.MountedT(t, childMapFS{"child": f})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	buf := make([]byte, 3)
	_, err = syscallx.Listxattr(mnt.Dir+"/child", buf)
	if err == nil {
		t.Error("Listxattr = nil; want some error")
	}
	if err != syscall.ERANGE {
		t.Errorf("unexpected error: %v", err)
		return
	}
}

// Test Listxattr used to probe result size

type listxattrSize struct {
	file
}

func (f *listxattrSize) Listxattr(req *fuse.ListxattrRequest, resp *fuse.ListxattrResponse, intr fs.Intr) fuse.Error {
	resp.Xattr = []byte("one\x00two\x00")
	return nil
}

func TestListxattrSize(t *testing.T) {
	t.Parallel()
	f := &listxattrSize{}
	mnt, err := fstestutil.MountedT(t, childMapFS{"child": f})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	n, err := syscallx.Listxattr(mnt.Dir+"/child", nil)
	if err != nil {
		t.Errorf("Listxattr unexpected error: %v", err)
		return
	}
	if g, e := n, len("one\x00two\x00"); g != e {
		t.Errorf("Getxattr incorrect size: %d != %d", g, e)
	}
}

// Test Setxattr

type setxattr struct {
	file
	record.Setxattrs
}

func TestSetxattr(t *testing.T) {
	t.Parallel()
	f := &setxattr{}
	mnt, err := fstestutil.MountedT(t, childMapFS{"child": f})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	err = syscallx.Setxattr(mnt.Dir+"/child", "greeting", []byte("hello, world"), 0)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	// fuse.SetxattrRequest contains a byte slice and thus cannot be
	// directly compared
	got := f.RecordedSetxattr()

	if g, e := got.Name, "greeting"; g != e {
		t.Errorf("Setxattr incorrect name: %q != %q", g, e)
	}

	if g, e := got.Flags, uint32(0); g != e {
		t.Errorf("Setxattr incorrect flags: %d != %d", g, e)
	}

	if g, e := string(got.Xattr), "hello, world"; g != e {
		t.Errorf("Setxattr incorrect data: %q != %q", g, e)
	}
}

// Test Removexattr

type removexattr struct {
	file
	record.Removexattrs
}

func TestRemovexattr(t *testing.T) {
	t.Parallel()
	f := &removexattr{}
	mnt, err := fstestutil.MountedT(t, childMapFS{"child": f})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	err = syscallx.Removexattr(mnt.Dir+"/child", "greeting")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	want := fuse.RemovexattrRequest{Name: "greeting"}
	if g, e := f.RecordedRemovexattr(), want; g != e {
		t.Errorf("removexattr saw %v, want %v", g, e)
	}
}

// Test default error.

type defaultErrno struct {
	dir
}

func (f defaultErrno) Lookup(name string, intr fs.Intr) (fs.Node, fuse.Error) {
	return nil, errors.New("bork")
}

func TestDefaultErrno(t *testing.T) {
	t.Parallel()
	mnt, err := fstestutil.MountedT(t, simpleFS{defaultErrno{}})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	_, err = os.Stat(mnt.Dir + "/trigger")
	if err == nil {
		t.Fatalf("expected error")
	}

	switch err2 := err.(type) {
	case *os.PathError:
		if err2.Err == syscall.EIO {
			break
		}
		t.Errorf("unexpected inner error: Err=%v %#v", err2.Err, err2)
	default:
		t.Errorf("unexpected error: %v", err)
	}
}

// Test custom error.

type customErrNode struct {
	dir
}

type myCustomError struct {
	fuse.ErrorNumber
}

var _ = fuse.ErrorNumber(myCustomError{})

func (myCustomError) Error() string {
	return "bork"
}

func (f customErrNode) Lookup(name string, intr fs.Intr) (fs.Node, fuse.Error) {
	return nil, myCustomError{
		ErrorNumber: fuse.Errno(syscall.ENAMETOOLONG),
	}
}

func TestCustomErrno(t *testing.T) {
	t.Parallel()
	mnt, err := fstestutil.MountedT(t, simpleFS{customErrNode{}})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	_, err = os.Stat(mnt.Dir + "/trigger")
	if err == nil {
		t.Fatalf("expected error")
	}

	switch err2 := err.(type) {
	case *os.PathError:
		if err2.Err == syscall.ENAMETOOLONG {
			break
		}
		t.Errorf("unexpected inner error: %#v", err2)
	default:
		t.Errorf("unexpected error: %v", err)
	}
}

// Test Mmap writing

type inMemoryFile struct {
	data []byte
}

func (f *inMemoryFile) Attr() fuse.Attr {
	return fuse.Attr{
		Mode: 0666,
		Size: uint64(len(f.data)),
	}
}

func (f *inMemoryFile) Read(req *fuse.ReadRequest, resp *fuse.ReadResponse, intr fs.Intr) fuse.Error {
	fuseutil.HandleRead(req, resp, f.data)
	return nil
}

func (f *inMemoryFile) Write(req *fuse.WriteRequest, resp *fuse.WriteResponse, intr fs.Intr) fuse.Error {
	resp.Size = copy(f.data[req.Offset:], req.Data)
	return nil
}

type mmap struct {
	inMemoryFile
	// We don't actually care about whether the fsync happened or not;
	// this just lets us force the page cache to send the writes to
	// FUSE, so we can reliably verify they came through.
	record.Fsyncs
}

func TestMmap(t *testing.T) {
	const size = 16 * 4096
	writes := map[int]byte{
		10:          'a',
		4096:        'b',
		4097:        'c',
		size - 4096: 'd',
		size - 1:    'z',
	}

	// Run the mmap-using parts of the test in a subprocess, to avoid
	// an intentional page fault hanging the whole process (because it
	// would need to be served by the same process, and there might
	// not be a thread free to do that). Merely bumping GOMAXPROCS is
	// not enough to prevent the hangs reliably.
	if childMode {
		f, err := os.Create("child")
		if err != nil {
			t.Fatalf("Create: %v", err)
		}
		defer f.Close()

		data, err := syscall.Mmap(int(f.Fd()), 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
		if err != nil {
			t.Fatalf("Mmap: %v", err)
		}

		for i, b := range writes {
			data[i] = b
		}

		if err := syscallx.Msync(data, syscall.MS_SYNC); err != nil {
			t.Fatalf("Msync: %v", err)
		}

		if err := syscall.Munmap(data); err != nil {
			t.Fatalf("Munmap: %v", err)
		}

		if err := f.Sync(); err != nil {
			t.Fatalf("Fsync = %v", err)
		}

		err = f.Close()
		if err != nil {
			t.Fatalf("Close: %v", err)
		}

		return
	}

	w := &mmap{}
	w.data = make([]byte, size)
	mnt, err := fstestutil.MountedT(t, childMapFS{"child": w})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	child, err := childCmd("TestMmap")
	if err != nil {
		t.Fatal(err)
	}
	child.Dir = mnt.Dir
	if err := child.Run(); err != nil {
		t.Fatal(err)
	}

	got := w.data
	if g, e := len(got), size; g != e {
		t.Fatalf("bad write length: %d != %d", g, e)
	}
	for i, g := range got {
		// default '\x00' for writes[i] is good here
		if e := writes[i]; g != e {
			t.Errorf("wrong byte at offset %d: %q != %q", i, g, e)
		}
	}
}

// Test direct Read.

type directRead struct{ file }

// explicitly not defining Attr and setting Size

func (f directRead) Open(req *fuse.OpenRequest, resp *fuse.OpenResponse, intr fs.Intr) (fs.Handle, fuse.Error) {
	// do not allow the kernel to use page cache
	resp.Flags |= fuse.OpenDirectIO
	return f, nil
}

func (directRead) Read(req *fuse.ReadRequest, resp *fuse.ReadResponse, intr fs.Intr) fuse.Error {
	fuseutil.HandleRead(req, resp, []byte(hi))
	return nil
}

func TestDirectRead(t *testing.T) {
	t.Parallel()
	mnt, err := fstestutil.MountedT(t, childMapFS{"child": directRead{}})
	if err != nil {
		t.Fatal(err)
	}
	defer mnt.Close()

	testReadAll(t, mnt.Dir+"/child")
}
