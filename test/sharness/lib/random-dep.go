// package randomdep is here to introduce a dependency in random for godep to
// function properly. this way we can keep go-random vendored and not
// accidentally break our tests when we change it.
package randomdep

import (
	_ "gx/ipfs/QmTyCJo2KTgqLxgZoSss3ZeDG637zrw9ZaXKJ1NKQmYybz/go-random-files"
	_ "gx/ipfs/QmbRf8gHLzTjhtVR8Qc4hBXF2c3pV7zCcaS8wSXQKx44kv/go-random"
)
