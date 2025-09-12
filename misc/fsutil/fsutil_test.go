package fsutil_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/ipfs/kubo/misc/fsutil"
	"github.com/stretchr/testify/require"
)

func TestDirWritable(t *testing.T) {
	err := fsutil.DirWritable("")
	require.Error(t, err)

	err = fsutil.DirWritable("~nosuchuser/tmp")
	require.Error(t, err)

	tmpDir := t.TempDir()

	wrDir := filepath.Join(tmpDir, "readwrite")
	err = fsutil.DirWritable(wrDir)
	require.NoError(t, err)

	// Check that DirWritable created directory.
	fi, err := os.Stat(wrDir)
	require.NoError(t, err)
	require.True(t, fi.IsDir())

	err = fsutil.DirWritable(wrDir)
	require.NoError(t, err)

	// If running on Windows, skip read-only directory tests.
	if runtime.GOOS == "windows" {
		t.SkipNow()
	}

	roDir := filepath.Join(tmpDir, "readonly")
	require.NoError(t, os.Mkdir(roDir, 0500))
	err = fsutil.DirWritable(roDir)
	require.ErrorIs(t, err, fs.ErrPermission)

	roChild := filepath.Join(roDir, "child")
	err = fsutil.DirWritable(roChild)
	require.ErrorIs(t, err, fs.ErrPermission)
}

func TestFileExists(t *testing.T) {
	fileName := filepath.Join(t.TempDir(), "somefile")
	require.False(t, fsutil.FileExists(fileName))

	file, err := os.Create(fileName)
	require.NoError(t, err)
	file.Close()

	require.True(t, fsutil.FileExists(fileName))
}

func TestExpandHome(t *testing.T) {
	dir, err := fsutil.ExpandHome("")
	require.NoError(t, err)
	require.Equal(t, "", dir)

	origDir := filepath.Join("somedir", "somesub")
	dir, err = fsutil.ExpandHome(origDir)
	require.NoError(t, err)
	require.Equal(t, origDir, dir)

	_, err = fsutil.ExpandHome(filepath.FromSlash("~nosuchuser/somedir"))
	require.Error(t, err)

	homeEnv := "HOME"
	if runtime.GOOS == "windows" {
		homeEnv = "USERPROFILE"
	}
	origHome := os.Getenv(homeEnv)
	defer os.Setenv(homeEnv, origHome)
	homeDir := filepath.Join(t.TempDir(), "testhome")
	os.Setenv(homeEnv, homeDir)

	const subDir = "mytmp"
	origDir = filepath.Join("~", subDir)
	dir, err = fsutil.ExpandHome(origDir)
	require.NoError(t, err)
	require.Equal(t, filepath.Join(homeDir, subDir), dir)

	os.Unsetenv(homeEnv)
	_, err = fsutil.ExpandHome(origDir)
	require.Error(t, err)
}
