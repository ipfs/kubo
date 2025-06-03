package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"testing"

	"github.com/ipfs/kubo/thirdparty/unit"

	random "github.com/ipfs/go-test/random"
	config "github.com/ipfs/kubo/config"
)

func main() {
	if err := compareResults(); err != nil {
		log.Fatal(err)
	}
}

func compareResults() error {
	var amount unit.Information
	for amount = 10 * unit.MB; amount > 0; amount = amount * 2 {
		if results, err := benchmarkAdd(int64(amount)); err != nil { // TODO compare
			return err
		} else {
			log.Println(amount, "\t", results)
		}
	}
	return nil
}

func benchmarkAdd(amount int64) (*testing.BenchmarkResult, error) {
	results := testing.Benchmark(func(b *testing.B) {
		b.SetBytes(amount)
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			tmpDir := b.TempDir()

			env := append(os.Environ(), fmt.Sprintf("%s=%s", config.EnvDir, path.Join(tmpDir, config.DefaultPathName)))
			setupCmd := func(cmd *exec.Cmd) {
				cmd.Env = env
			}

			cmd := exec.Command("ipfs", "init")
			setupCmd(cmd)
			if err := cmd.Run(); err != nil {
				b.Fatal(err)
			}

			const seed = 1
			f, err := os.CreateTemp("", "")
			if err != nil {
				b.Fatal(err)
			}
			defer os.Remove(f.Name())

			randReader := &io.LimitedReader{
				R: random.NewSeededRand(seed),
				N: amount,
			}
			_, err = io.Copy(f, randReader)
			if err != nil {
				b.Fatal(err)
			}
			if err := f.Close(); err != nil {
				b.Fatal(err)
			}

			b.StartTimer()
			cmd = exec.Command("ipfs", "add", f.Name())
			setupCmd(cmd)
			if err := cmd.Run(); err != nil {
				b.Fatal(err)
			}
			b.StopTimer()
		}
	})
	return &results, nil
}
