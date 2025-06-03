package migrations

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/blang/semver/v4"
)

const distVersions = "versions"

// LatestDistVersion returns the latest version, of the specified distribution,
// that is available on the distribution site.
func LatestDistVersion(ctx context.Context, fetcher Fetcher, dist string, stableOnly bool) (string, error) {
	vs, err := DistVersions(ctx, fetcher, dist, false)
	if err != nil {
		return "", err
	}

	for i := len(vs) - 1; i >= 0; i-- {
		ver := vs[i]
		if stableOnly && strings.Contains(ver, "-rc") {
			continue
		}
		if strings.Contains(ver, "-dev") {
			continue
		}
		return ver, nil
	}
	return "", errors.New("could not find a non dev version")
}

// DistVersions returns all versions of the specified distribution, that are
// available on the distriburion site.  List is in ascending order, unless
// sortDesc is true.
func DistVersions(ctx context.Context, fetcher Fetcher, dist string, sortDesc bool) ([]string, error) {
	versionBytes, err := fetcher.Fetch(ctx, path.Join(dist, distVersions))
	if err != nil {
		return nil, err
	}

	prefix := "v"
	var vers []semver.Version

	scan := bufio.NewScanner(bytes.NewReader(versionBytes))
	for scan.Scan() {
		ver, err := semver.Make(strings.TrimLeft(scan.Text(), prefix))
		if err != nil {
			continue
		}
		vers = append(vers, ver)
	}
	if scan.Err() != nil {
		return nil, fmt.Errorf("could not read versions: %w", scan.Err())
	}

	if sortDesc {
		sort.Sort(sort.Reverse(semver.Versions(vers)))
	} else {
		sort.Sort(semver.Versions(vers))
	}

	out := make([]string, len(vers))
	for i := range vers {
		out[i] = prefix + vers[i].String()
	}

	return out, nil
}
