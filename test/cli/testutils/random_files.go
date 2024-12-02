package testutils

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"path"
	"time"
)

var (
	AlphabetEasy = []rune("abcdefghijklmnopqrstuvwxyz01234567890-_")
	AlphabetHard = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ01234567890!@#$%^&*()-_+= ;.,<>'\"[]{}() ")
)

type RandFiles struct {
	Rand         *rand.Rand
	FileSize     int // the size per file.
	FilenameSize int
	Alphabet     []rune // for filenames

	FanoutDepth int // how deep the hierarchy goes
	FanoutFiles int // how many files per dir
	FanoutDirs  int // how many dirs per dir

	RandomSize   bool // randomize file sizes
	RandomFanout bool // randomize fanout numbers
}

func NewRandFiles() *RandFiles {
	return &RandFiles{
		Rand:         rand.New(rand.NewSource(time.Now().UnixNano())),
		FileSize:     4096,
		FilenameSize: 16,
		Alphabet:     AlphabetEasy,
		FanoutDepth:  2,
		FanoutDirs:   5,
		FanoutFiles:  10,
		RandomSize:   true,
	}
}

func (r *RandFiles) WriteRandomFiles(root string, depth int) error {
	numfiles := r.FanoutFiles
	if r.RandomFanout {
		numfiles = rand.Intn(r.FanoutFiles) + 1
	}

	for i := 0; i < numfiles; i++ {
		if err := r.WriteRandomFile(root); err != nil {
			return err
		}
	}

	if depth+1 <= r.FanoutDepth {
		numdirs := r.FanoutDirs
		if r.RandomFanout {
			numdirs = r.Rand.Intn(numdirs) + 1
		}

		for i := 0; i < numdirs; i++ {
			if err := r.WriteRandomDir(root, depth+1); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *RandFiles) RandomFilename(length int) string {
	b := make([]rune, length)
	for i := range b {
		b[i] = r.Alphabet[r.Rand.Intn(len(r.Alphabet))]
	}
	return string(b)
}

func (r *RandFiles) WriteRandomFile(root string) error {
	filesize := int64(r.FileSize)
	if r.RandomSize {
		filesize = r.Rand.Int63n(filesize) + 1
	}

	n := rand.Intn(r.FilenameSize-4) + 4
	name := r.RandomFilename(n)
	filepath := path.Join(root, name)
	f, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("creating random file: %w", err)
	}

	if _, err := io.CopyN(f, r.Rand, filesize); err != nil {
		return fmt.Errorf("copying random file: %w", err)
	}

	return f.Close()
}

func (r *RandFiles) WriteRandomDir(root string, depth int) error {
	if depth > r.FanoutDepth {
		return nil
	}

	n := rand.Intn(r.FilenameSize-4) + 4
	name := r.RandomFilename(n)
	root = path.Join(root, name)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("creating random dir: %w", err)
	}

	err := r.WriteRandomFiles(root, depth)
	if err != nil {
		return fmt.Errorf("writing random files in random dir: %w", err)
	}
	return nil
}
