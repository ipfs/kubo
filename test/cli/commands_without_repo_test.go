package cli

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestCommandsWithoutRepo(t *testing.T) {
	t.Run("cid", func(t *testing.T) {
		t.Run("base32", func(t *testing.T) {
			cmd := exec.Command("ipfs", "cid", "base32", "QmS4ustL54uo8FzR9455qaxZwuMiUhyvMcX9Ba8nUH4uVv")
			cmd.Env = append(os.Environ(), "IPFS_PATH="+t.TempDir())
			stdout, err := cmd.Output()
			if err != nil {
				t.Fatal(err)
			}
			expected := "bafybeibxm2nsadl3fnxv2sxcxmxaco2jl53wpeorjdzidjwf5aqdg7wa6u\n"
			if string(stdout) != expected {
				t.Fatalf("expected %q, got: %q", expected, stdout)
			}
		})

		t.Run("format", func(t *testing.T) {
			cmd := exec.Command("ipfs", "cid", "format", "-v", "1", "QmS4ustL54uo8FzR9455qaxZwuMiUhyvMcX9Ba8nUH4uVv")
			cmd.Env = append(os.Environ(), "IPFS_PATH="+t.TempDir())
			stdout, err := cmd.Output()
			if err != nil {
				t.Fatal(err)
			}
			expected := "zdj7WZAAFKPvYPPzyJLso2hhxo8a7ZACFQ4DvvfrNXTHidofr\n"
			if string(stdout) != expected {
				t.Fatalf("expected %q, got: %q", expected, stdout)
			}
		})

		t.Run("bases", func(t *testing.T) {
			cmd := exec.Command("ipfs", "cid", "bases")
			cmd.Env = append(os.Environ(), "IPFS_PATH="+t.TempDir())
			stdout, err := cmd.Output()
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(stdout), "base32") {
				t.Fatalf("expected base32 in output, got: %s", stdout)
			}
		})

		t.Run("codecs", func(t *testing.T) {
			cmd := exec.Command("ipfs", "cid", "codecs")
			cmd.Env = append(os.Environ(), "IPFS_PATH="+t.TempDir())
			stdout, err := cmd.Output()
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(stdout), "dag-pb") {
				t.Fatalf("expected dag-pb in output, got: %s", stdout)
			}
		})

		t.Run("hashes", func(t *testing.T) {
			cmd := exec.Command("ipfs", "cid", "hashes")
			cmd.Env = append(os.Environ(), "IPFS_PATH="+t.TempDir())
			stdout, err := cmd.Output()
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(stdout), "sha2-256") {
				t.Fatalf("expected sha2-256 in output, got: %s", stdout)
			}
		})
	})

	t.Run("multibase", func(t *testing.T) {
		t.Run("list", func(t *testing.T) {
			cmd := exec.Command("ipfs", "multibase", "list")
			cmd.Env = append(os.Environ(), "IPFS_PATH="+t.TempDir())
			stdout, err := cmd.Output()
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(stdout), "base32") {
				t.Fatalf("expected base32 in output, got: %s", stdout)
			}
		})

		t.Run("encode", func(t *testing.T) {
			cmd := exec.Command("ipfs", "multibase", "encode", "-b", "base32")
			cmd.Env = append(os.Environ(), "IPFS_PATH="+t.TempDir())
			cmd.Stdin = strings.NewReader("hello\n")
			stdout, err := cmd.Output()
			if err != nil {
				t.Fatal(err)
			}
			expected := "bnbswy3dpbi"
			if string(stdout) != expected {
				t.Fatalf("expected %q, got: %q", expected, stdout)
			}
		})

		t.Run("decode", func(t *testing.T) {
			cmd := exec.Command("ipfs", "multibase", "decode")
			cmd.Env = append(os.Environ(), "IPFS_PATH="+t.TempDir())
			cmd.Stdin = strings.NewReader("bnbswy3dpbi")
			stdout, err := cmd.Output()
			if err != nil {
				t.Fatal(err)
			}
			expected := "hello\n"
			if string(stdout) != expected {
				t.Fatalf("expected %q, got: %q", expected, stdout)
			}
		})

		t.Run("transcode", func(t *testing.T) {
			cmd := exec.Command("ipfs", "multibase", "transcode", "-b", "base64")
			cmd.Env = append(os.Environ(), "IPFS_PATH="+t.TempDir())
			cmd.Stdin = strings.NewReader("bnbswy3dpbi")
			stdout, err := cmd.Output()
			if err != nil {
				t.Fatal(err)
			}
			expected := "maGVsbG8K"
			if string(stdout) != expected {
				t.Fatalf("expected %q, got: %q", expected, stdout)
			}
		})
	})
}
