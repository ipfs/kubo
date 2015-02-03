package updates

import (
	"errors"
	"fmt"
	"os"
	"time"

	config "github.com/jbenet/go-ipfs/repo/config"
	fsrepo "github.com/jbenet/go-ipfs/repo/fsrepo"
	u "github.com/jbenet/go-ipfs/util"

	semver "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/coreos/go-semver/semver"
	update "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/inconshreveable/go-update"
	check "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/inconshreveable/go-update/check"
)

const (
	// Version is the current application's version literal
	Version = config.CurrentVersionNumber

	updateEndpointURL = "https://api.equinox.io/1/Updates"
	updateAppID       = "ap_YM8nz6rGm1UPg_bf63Lw6Vjz49"

	// this is @jbenet's equinox.io public key.
	updatePubKey = `-----BEGIN RSA PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAxnwPPE4LNMjTfW/NRz1z
8uAPpwGYSzac+cwZbHbL5xFOxeX301GCdISaMm+Q8OEJqLyXfjYSuRwx00fDzWDD
ajBQOsxO08gTy1i/ow5YdEO+nYeVKO08fQFqVqdTz09BCgzt9iQJTEMeiq1kSWNo
al8usHD4SsNTxwDpSlok5UKWCHcr7D/TWX5A4B5A6ae9HSEcMB4Aum83k63Vzgm1
WTUvK0ed1zd0/KcHqIU36VZpVg4PeV4SWnOBnldQ98CWg/Mnqp3+lXMWYWTmXeX6
xj8JqOGpebzlxeISKE6fDBtrLxUbFTt3DNshl7S5CUGuc5H1MF1FTAyi+8u/nEZB
cQIDAQAB
-----END RSA PUBLIC KEY-----`

/*

You can verify the key above (updatePubKey) is indeed controlled
by @jbenet, ipfs author, with the PGP signed message below. You
can verify it in the commandline, or keybase.io.

-----BEGIN PGP SIGNED MESSAGE-----
Hash: SHA512

I hereby certify that I control the private key matching the
following public key. This is a key used for go-ipfs auto-updates
over equinox.io. - @jbenet

- -----BEGIN RSA PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAxnwPPE4LNMjTfW/NRz1z
8uAPpwGYSzac+cwZbHbL5xFOxeX301GCdISaMm+Q8OEJqLyXfjYSuRwx00fDzWDD
ajBQOsxO08gTy1i/ow5YdEO+nYeVKO08fQFqVqdTz09BCgzt9iQJTEMeiq1kSWNo
al8usHD4SsNTxwDpSlok5UKWCHcr7D/TWX5A4B5A6ae9HSEcMB4Aum83k63Vzgm1
WTUvK0ed1zd0/KcHqIU36VZpVg4PeV4SWnOBnldQ98CWg/Mnqp3+lXMWYWTmXeX6
xj8JqOGpebzlxeISKE6fDBtrLxUbFTt3DNshl7S5CUGuc5H1MF1FTAyi+8u/nEZB
cQIDAQAB
- -----END RSA PUBLIC KEY-----
-----BEGIN PGP SIGNATURE-----
Version: Keybase OpenPGP v1.1.3
Comment: https://keybase.io/crypto

wsFcBAABCgAGBQJUSCX8AAoJEFYC7bhkX9ftBcwQAJuYGSECSKFATJ1wK+zAGUH5
xEbX+yaCYj0PwzJO4Ntu2ifK68ANacKy/GiXdJYeQk7pq21UT0fcn0Uq39URu+Xb
lk3t1YZazjY7wB03jBjcMIaO2TUsWbGIBZAEZjyVDDctDUM0krCd1GIOw6Fbndva
pevlGIA55ewvXYxcWdRyOGWiqd9DKNnmi9UF0XsdpCtDFSkdjnqkqbTRxF6Jw5gI
EAF2E7mU8emDTNgtpCs0ACmEUXVVEEhF9TuR/YdX1m/715TYkkYCii6uV9vSVQd8
nOrDDTrWSjlF6Ms+dYGCheWIjKQcykn9IW021AzVN1P7Mt9qtmDNfZ0VQL3zl/fs
zZ1IHBW7BzriQ4GzWXg5GWpTSz/REvUEfKNVuDV9jX7hv67B5H6qTL5+2zljPEKv
lCas04cCMmEpJUj4qK95hdKQzKJ8b7MrRf/RFYyViRGdxvR+lgGqJ7Yca8es2kCe
XV6c+i6a7X89YL6ZVU+1MlvPwngu0VG+VInH/w9KrNYrLFhfVRiruRbkBkHDXjnU
b4kPqaus+7g0DynCk7A2kTMa3cgtO20CZ9MBJFEPqRRHHksjHVmlxPb42bB348aR
UVsWkRRYOmRML7avTgkX8WFsmdZ1d7E7aQLYnCIel85+5iP7hWyNtEMsAHk02XCL
AAb7RaEDNJOa7qvUFecB
=mzPY
-----END PGP SIGNATURE-----


*/

)

var log = u.Logger("updates")

var currentVersion *semver.Version

// ErrNoUpdateAvailable returned when a check fails to find a newer update.
var ErrNoUpdateAvailable = check.NoUpdateAvailable

func init() {
	var err error
	currentVersion, err = parseVersion()
	if err != nil {
		log.Criticalf("invalid version number in code (must be semver): %q", Version)
		os.Exit(1)
	}
	log.Infof("go-ipfs Version: %s", currentVersion)
}

func parseVersion() (*semver.Version, error) {
	return semver.NewVersion(Version)
}

// CheckForUpdate checks the equinox.io api if there is an update available
// NOTE: if equinox says there is a new update, but the version number IS NOT
// larger, we interpret that as no update (you may have gotten a newer version
// by building it yourself).
func CheckForUpdate() (*check.Result, error) {
	param := check.Params{
		AppVersion: Version,
		AppId:      updateAppID,
		Channel:    "stable",
	}

	up, err := update.New().VerifySignatureWithPEM([]byte(updatePubKey))
	if err != nil {
		return nil, fmt.Errorf("Failed to parse public key: %v", err)
	}

	res, err := param.CheckForUpdate(updateEndpointURL, up)
	if err != nil {
		return res, err
	}

	newer, err := versionIsNewer(res.Version)
	if err != nil {
		return nil, err
	}
	if !newer {
		return nil, ErrNoUpdateAvailable
	}
	return res, err
}

// Apply cheks if the running process is able to update itself
// and than updates to the passed release
func Apply(rel *check.Result) error {
	if err := update.New().CanUpdate(); err != nil {
		return err
	}

	if err, errRecover := rel.Update(); err != nil {
		err = fmt.Errorf("Update failed: %v\n", err)
		if errRecover != nil {
			err = fmt.Errorf("%s\nRecovery failed! Cause: %v\nYou may need to recover manually", err, errRecover)
		}
		return err
	}

	return nil
}

// ShouldAutoUpdate decides wether a new version should be applied
// checks against config setting and new version string. returns false in case of error
func ShouldAutoUpdate(setting config.AutoUpdateSetting, newVer string) bool {
	if setting == config.AutoUpdateNever {
		return false
	}

	nv, err := semver.NewVersion(newVer)
	if err != nil {
		log.Infof("could not parse version string: %s", err)
		return false
	}

	n := nv.Slice()
	c := currentVersion.Slice()

	switch setting {

	case config.AutoUpdatePatch:
		if n[0] < c[0] {
			return false
		}

		if n[1] < c[1] {
			return false
		}

		return n[2] > c[2]

	case config.AutoUpdateMinor:
		if n[0] != c[0] {
			return false
		}

		return n[1] > c[1] || (n[1] == c[1] && n[2] > c[2])

	case config.AutoUpdateMajor:
		for i := 0; i < 3; i++ {
			if n[i] < c[i] {
				return false
			}
		}
		return true
	}

	return false
}

// CliCheckForUpdates is the automatic update check from the commandline.
func CliCheckForUpdates(cfg *config.Config, repoPath string) error {

	// if config says not to, don't check for updates
	if !cfg.Version.ShouldCheckForUpdate() {
		log.Info("update check skipped.")
		return nil
	}

	log.Info("checking for update")
	u, err := CheckForUpdate()
	// if there is no update available, record it, and exit. NB:  only record
	// if we checked successfully.
	if err == ErrNoUpdateAvailable {
		log.Noticef("No update available, checked on %s", time.Now())
		r := fsrepo.At(repoPath)
		if err := r.Open(); err != nil {
			return err
		}
		if err := recordUpdateCheck(cfg); err != nil {
			return err
		}
		// NB: r's Config may be newer than cfg. This overwrites regardless.
		r.SetConfig(cfg)
		if err := r.Close(); err != nil {
			return err
		}
		return nil
	}

	// if another, unexpected error occurred, note it.
	if err != nil {
		log.Debugf("Error while checking for update: %v", err)
		return nil
	}

	// there is an update available

	// if we autoupdate
	if cfg.Version.AutoUpdate != config.AutoUpdateNever {
		// and we should auto update
		if ShouldAutoUpdate(cfg.Version.AutoUpdate, u.Version) {
			log.Noticef("Applying update %s", u.Version)

			if err = Apply(u); err != nil {
				log.Debug(err)
				return nil
			}

			// BUG(cryptix): no good way to restart yet. - tracking https://github.com/inconshreveable/go-update/issues/5
			fmt.Printf("update %v applied. please restart.\n", u.Version)
			os.Exit(0)
		}
	}

	// autoupdate did not exit, so regular notices.
	switch cfg.Version.Check {
	case config.CheckError:
		return fmt.Errorf(errShouldUpdate, Version, u.Version)
	case config.CheckWarn:
		// print the warning
		fmt.Printf("New version available: %s\n", u.Version)
	default: // ignore
	}
	return nil
}

func versionIsNewer(version string) (bool, error) {
	nv, err := semver.NewVersion(version)
	if err != nil {
		return false, fmt.Errorf("could not parse version string: %s", err)
	}

	cv := currentVersion
	newer := !nv.LessThan(*cv) && nv.String() != cv.String()
	return newer, nil
}

var errShouldUpdate = `
Your go-ipfs version is: %s
There is a new version available: %s
Since this is alpha software, it is strongly recommended you update.

To update, run:

    ipfs update apply

To disable this notice, run:

    ipfs config Version.Check warn

`

// recordUpdateCheck is called to record that an update check was performed,
// showing that the running version is the most recent one.
func recordUpdateCheck(cfg *config.Config) error {
	cfg.Version.CheckDate = time.Now()

	if cfg.Version.CheckPeriod == "" {
		// CheckPeriod was not initialized for some reason (e.g. config file broken)
		return errors.New("config.Version.CheckPeriod not set. config broken?")
	}
	return nil
}
