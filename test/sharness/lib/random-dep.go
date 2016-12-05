// package randomdep is here to introduce a dependency in random for godep to
// function properly. this way we can keep go-random vendored and not
// accidentally break our tests when we change it.
package randomdep

import (
	_ "gx/ipfs/QmVeTQJruz98RkXfsJPzKWJcTFzFk3S8ZnoWQdYijuha34/go-random"

	_ "gx/ipfs/QmY4r4SdwZCBfWn4wFkDshVG59Qt2ApcPB63Bcyb9HseEm/go-random-files"
)
