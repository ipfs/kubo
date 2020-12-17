package migrations

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/coreos/go-semver/semver"
)

const distVersions = "versions"

// LatestDistVersion returns the latest version, of the specified distribution,
// that is available on the distribution site.
func LatestDistVersion(ctx context.Context, dist string) (string, error) {
	vs, err := DistVersions(ctx, dist, false)
	if err != nil {
		return "", err
	}

	for i := len(vs) - 1; i >= 0; i-- {
		ver := vs[i]
		if !strings.Contains(ver, "-dev") {
			return ver, nil
		}
	}
	return "", errors.New("could not find a non dev version")
}

// DistVersions returns all versions of the specified distribution, that are
// available on the distriburion site.  List is in ascending order, unless
// sortDesc is true.
func DistVersions(ctx context.Context, dist string, sortDesc bool) ([]string, error) {
	rc, err := fetch(ctx, path.Join(ipfsDistPath, dist, distVersions))
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	prefix := "v"
	var vers []*semver.Version

	scan := bufio.NewScanner(rc)
	for scan.Scan() {
		ver, err := semver.NewVersion(strings.TrimLeft(scan.Text(), prefix))
		if err != nil {
			continue
		}
		vers = append(vers, ver)
	}
	err = scan.Err()
	if err != nil {
		return nil, fmt.Errorf("could not read versions: %s", err)
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

// IpfsRepoVersion returns the repo version required by the ipfs daemon
func IpfsRepoVersion(ctx context.Context) (int, error) {
	out, err := exec.CommandContext(ctx, "ipfs", "version", "--repo").CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("%s: %s", err, string(out))
	}

	verStr := strings.TrimSpace(string(out))
	ver, err := strconv.Atoi(verStr)
	if err != nil {
		return 0, fmt.Errorf("repo version is not an integer: %s", verStr)
	}

	return ver, nil
}
