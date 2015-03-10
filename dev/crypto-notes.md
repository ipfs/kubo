## Important notes

### Key-pair generation

When compared to gpg without hardware-based (P)RNG, IPFS generates key-pair
alarmingly fast: it takes ipfs about 1 minute to generate 4096-bit
key-pair, but for gpg it takes about 10 minutes. In the same time
entropy_avail show severe drop in available entropy for gpg, but for
ipfs entropy drops about 100 bits.

[This issue (#911)](https://github.com/jbenet/go-ipfs/issues/911) seems to be caused
by `crypto/rand` implementation in the Go programming language:

1. [in UNIX-like](http://golang.org/src/crypto/rand/rand_unix.go) operating system it uses /dev/urandom device:
  ```
  // Easy implementation: read from /dev/urandom.
  // This is sufficient on Linux, OS X, and FreeBSD.
  ```

  For OS X that would use 160-bit Yarrow PRNG based on SHA-1 key, for FreeBSD - 256-bit Yarrow algorithm. For both operating systems /dev/random and /dev/urandom are equal.

2. [in Linux](http://golang.org/src/crypto/rand/rand_linux.go#L22) it falls back to urandom in several cases:
  ```
  // Test whether we should use the system call or /dev/urandom.
  // We'll fall back to urandom if:
  // - the kernel is too old (before 3.17)
  // - the machine has no entropy available (early boot + no hardware
  //   entropy source?) and we want to avoid blocking later.
  ```

  The first clause would be used for several production-class operationg systems.

3. [in Windows]() it uses Windows CryptGenRandom API.

According to [wikipedia](https://en.wikipedia.org/?title=/dev/random) using /dev/urandom instead of /dev/random seems to be safe.
