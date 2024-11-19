package serialize

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ipfs/kubo/config"

	"github.com/facebookgo/atomicfile"
)

// ErrNotInitialized is returned when we fail to read the config because the
// repo doesn't exist.
var ErrNotInitialized = errors.New("ipfs not initialized, please run 'ipfs init'")

// removeCommentLines reads from the provided io.Reader, removes lines that
// start with "//", and writes the result to the provided io.Writer.
func removeCommentLines(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	writer := bufio.NewWriter(w)
	defer writer.Flush()

	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimLeft(line, " ")
		if !strings.HasPrefix(trimmed, "//") {
			if _, err := writer.WriteString(line + "\n"); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

// ReadConfigFile reads the config from `filename` into `cfg`.
func ReadConfigFile(filename string, cfg interface{}) error {
	f, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			err = ErrNotInitialized
		}
		return err
	}
	defer f.Close()

	// Remove line comments (any line that has `\s*//`)
	r, w := io.Pipe()
	go func() {
		if err := removeCommentLines(f, w); err != nil {
			w.CloseWithError(err)
			return
		}
		w.Close()
	}()
	if err := json.NewDecoder(r).Decode(cfg); err != nil {
		return fmt.Errorf("failure to decode config: %w", err)
	}
	return nil
}

// WriteConfigFile writes the config from `cfg` into `filename`.
func WriteConfigFile(filename string, cfg interface{}) error {
	err := os.MkdirAll(filepath.Dir(filename), 0o755)
	if err != nil {
		return err
	}

	f, err := atomicfile.New(filename, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	return encode(f, cfg)
}

// encode configuration with JSON.
func encode(w io.Writer, value interface{}) error {
	// need to prettyprint, hence MarshalIndent, instead of Encoder
	buf, err := config.Marshal(value)
	if err != nil {
		return err
	}
	_, err = w.Write(buf)
	return err
}

// Load reads given file and returns the read config, or error.
func Load(filename string) (*config.Config, error) {
	var cfg config.Config
	err := ReadConfigFile(filename, &cfg)
	if err != nil {
		return nil, err
	}

	return &cfg, err
}
