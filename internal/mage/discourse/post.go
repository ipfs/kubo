package discourse

import (
	"context"
	"fmt"

	"github.com/magefile/mage/mg" // mg contains helpful utility functions, like Deps
	"golang.org/x/mod/semver"
)

type Post mg.Namespace

func (Post) GetTitle(ctx context.Context, version string) error {
	fmt.Printf("Kubo %s Release Candidate is out!\n", version)
	return nil
}

func (Post) GetBody(ctx context.Context, version string) error {
	mm := semver.MajorMinor(version)
	fmt.Printf(
		`## Kubo %s Release Candidate is out!

See:
- Code: https://github.com/ipfs/kubo/releases/tag/%s
- Binaries: https://dist.ipfs.tech/kubo/%s/
- Docker: `+"`"+`docker pull ipfs/kubo:%s`+"`"+`
- Release Notes (WIP): https://github.com/ipfs/kubo/blob/release-%s/docs/changelogs/%s.md
`,
		version, version, version, version, mm, mm)
	return nil
}
