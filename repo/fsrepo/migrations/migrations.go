package migrations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"
	"sync"

	config "github.com/ipfs/kubo/config"
)

const (
	// Migrations subdirectory in distribution. Empty for root (no subdir).
	distMigsRoot = ""
	distFSRM     = "fs-repo-migrations"
)

// RunMigration finds, downloads, and runs the individual migrations needed to
// migrate the repo from its current version to the target version.
//
// Deprecated: This function downloads migration binaries from the internet and will be removed
// in a future version. Use RunHybridMigrations for modern migrations with embedded support,
// or RunEmbeddedMigrations for repo versions ≥16.
func RunMigration(ctx context.Context, fetcher Fetcher, targetVer int, ipfsDir string, allowDowngrade bool) error {
	ipfsDir, err := CheckIpfsDir(ipfsDir)
	if err != nil {
		return err
	}
	fromVer, err := RepoVersion(ipfsDir)
	if err != nil {
		return fmt.Errorf("could not get repo version: %w", err)
	}
	if fromVer == targetVer {
		// repo already at target version number
		return nil
	}
	if fromVer > targetVer && !allowDowngrade {
		return fmt.Errorf("downgrade not allowed from %d to %d", fromVer, targetVer)
	}

	logger := log.New(os.Stdout, "", 0)

	logger.Print("Looking for suitable migration binaries.")

	migrations, binPaths, err := findMigrations(ctx, fromVer, targetVer)
	if err != nil {
		return err
	}

	// Download migrations that were not found
	if len(binPaths) < len(migrations) {
		missing := make([]string, 0, len(migrations)-len(binPaths))
		for _, mig := range migrations {
			if _, ok := binPaths[mig]; !ok {
				missing = append(missing, mig)
			}
		}

		logger.Println("Need", len(missing), "migrations, downloading.")

		tmpDir, err := os.MkdirTemp("", "migrations")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmpDir)

		fetched, err := fetchMigrations(ctx, fetcher, missing, tmpDir, logger)
		if err != nil {
			logger.Print("Failed to download migrations.")
			return err
		}

		for i := range missing {
			binPaths[missing[i]] = fetched[i]
		}
	}

	var revert bool
	if fromVer > targetVer {
		revert = true
	}
	for _, migration := range migrations {
		logger.Println("Running migration", migration, "...")
		err = runMigration(ctx, binPaths[migration], ipfsDir, revert, logger)
		if err != nil {
			return fmt.Errorf("migration %s failed: %w", migration, err)
		}
	}
	logger.Printf("Success: fs-repo migrated to version %d.\n", targetVer)

	return nil
}

func NeedMigration(target int) (bool, error) {
	vnum, err := RepoVersion("")
	if err != nil {
		return false, fmt.Errorf("could not get repo version: %w", err)
	}

	return vnum != target, nil
}

func ExeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// ReadMigrationConfig reads the Migration section of the IPFS config, avoiding
// reading anything other than the Migration section. That way, we're free to
// make arbitrary changes to all _other_ sections in migrations.
//
// Deprecated: This function is used by legacy migration downloads and will be removed
// in a future version. Use RunHybridMigrations or RunEmbeddedMigrations instead.
func ReadMigrationConfig(repoRoot string, userConfigFile string) (*config.Migration, error) {
	var cfg struct {
		Migration config.Migration
	}

	cfgPath, err := config.Filename(repoRoot, userConfigFile)
	if err != nil {
		return nil, err
	}

	cfgFile, err := os.Open(cfgPath)
	if err != nil {
		return nil, err
	}
	defer cfgFile.Close()

	err = json.NewDecoder(cfgFile).Decode(&cfg)
	if err != nil {
		return nil, err
	}

	switch cfg.Migration.Keep {
	case "":
		cfg.Migration.Keep = config.DefaultMigrationKeep
	case "discard", "cache", "keep":
	default:
		return nil, errors.New("unknown config value, Migrations.Keep must be 'cache', 'pin', or 'discard'")
	}

	if len(cfg.Migration.DownloadSources) == 0 {
		cfg.Migration.DownloadSources = config.DefaultMigrationDownloadSources
	}

	return &cfg.Migration, nil
}

// GetMigrationFetcher creates one or more fetchers according to
// downloadSources.
//
// Deprecated: This function is used by legacy migration downloads and will be removed
// in a future version. Use RunHybridMigrations or RunEmbeddedMigrations instead.
func GetMigrationFetcher(downloadSources []string, distPath string, newIpfsFetcher func(string) Fetcher) (Fetcher, error) {
	const httpUserAgent = "kubo/migration"
	const numTriesPerHTTP = 3

	var fetchers []Fetcher
	for _, src := range downloadSources {
		src := strings.TrimSpace(src)
		switch src {
		case "HTTPS", "https", "HTTP", "http":
			fetchers = append(fetchers, &RetryFetcher{NewHttpFetcher(distPath, "", httpUserAgent, 0), numTriesPerHTTP})
		case "IPFS", "ipfs":
			return nil, errors.New("IPFS downloads are not supported for legacy migrations (repo versions <16). Please use only HTTPS in Migration.DownloadSources")
		case "":
			// Ignore empty string
		default:
			u, err := url.Parse(src)
			if err != nil {
				return nil, fmt.Errorf("bad gateway address: %w", err)
			}
			switch u.Scheme {
			case "":
				u.Scheme = "https"
			case "https", "http":
			default:
				return nil, errors.New("bad gateway address: url scheme must be http or https")
			}
			fetchers = append(fetchers, &RetryFetcher{NewHttpFetcher(distPath, u.String(), httpUserAgent, 0), numTriesPerHTTP})
		}
	}

	switch len(fetchers) {
	case 0:
		return nil, errors.New("no sources specified")
	case 1:
		return fetchers[0], nil
	}

	// Wrap fetchers in a MultiFetcher to try them in order
	return NewMultiFetcher(fetchers...), nil
}

func migrationName(from, to int) string {
	return fmt.Sprintf("fs-repo-%d-to-%d", from, to)
}

// findMigrations returns a list of migrations, ordered from first to last
// migration to apply, and a map of locations of migration binaries of any
// migrations that were found.
//
// Deprecated: This function is used by legacy migration downloads and will be removed
// in a future version.
func findMigrations(ctx context.Context, from, to int) ([]string, map[string]string, error) {
	step := 1
	count := to - from
	if from > to {
		step = -1
		count = from - to
	}

	migrations := make([]string, 0, count)
	binPaths := make(map[string]string, count)

	for cur := from; cur != to; cur += step {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		var migName string
		if step == -1 {
			migName = migrationName(cur+step, cur)
		} else {
			migName = migrationName(cur, cur+step)
		}
		migrations = append(migrations, migName)
		bin, err := exec.LookPath(migName)
		if err != nil {
			continue
		}
		binPaths[migName] = bin
	}
	return migrations, binPaths, nil
}

func runMigration(ctx context.Context, binPath, ipfsDir string, revert bool, logger *log.Logger) error {
	pathArg := fmt.Sprintf("-path=%s", ipfsDir)
	var cmd *exec.Cmd
	if revert {
		logger.Println("  => Running:", binPath, pathArg, "-verbose=true -revert")
		cmd = exec.CommandContext(ctx, binPath, pathArg, "-verbose=true", "-revert")
	} else {
		logger.Println("  => Running:", binPath, pathArg, "-verbose=true")
		cmd = exec.CommandContext(ctx, binPath, pathArg, "-verbose=true")
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// fetchMigrations downloads the requested migrations, and returns a slice with
// the paths of each binary, in the same order specified by needed.
//
// Deprecated: This function downloads migration binaries from the internet and will be removed
// in a future version. Use RunHybridMigrations or RunEmbeddedMigrations instead.
func fetchMigrations(ctx context.Context, fetcher Fetcher, needed []string, destDir string, logger *log.Logger) ([]string, error) {
	osv, err := osWithVariant()
	if err != nil {
		return nil, err
	}
	if osv == "linux-musl" {
		return nil, fmt.Errorf("linux-musl not supported, you must build the binary from source for your platform")
	}

	var wg sync.WaitGroup
	wg.Add(len(needed))
	bins := make([]string, len(needed))
	// Download and unpack all requested migrations concurrently.
	for i, name := range needed {
		logger.Printf("Downloading migration: %s...", name)
		go func(i int, name string) {
			defer wg.Done()
			dist := path.Join(distMigsRoot, name)
			ver, err := LatestDistVersion(ctx, fetcher, dist, false)
			if err != nil {
				logger.Printf("could not get latest version of migration %s: %s", name, err)
				return
			}
			loc, err := FetchBinary(ctx, fetcher, dist, ver, name, destDir)
			if err != nil {
				logger.Printf("could not download %s: %s", name, err)
				return
			}
			logger.Printf("Downloaded and unpacked migration: %s (%s)", loc, ver)
			bins[i] = loc
		}(i, name)
	}
	wg.Wait()

	var fails []string
	for i := range bins {
		if bins[i] == "" {
			fails = append(fails, needed[i])
		}
	}
	if len(fails) != 0 {
		err = fmt.Errorf("failed to download migrations: %s", strings.Join(fails, " "))
		if ctx.Err() != nil {
			err = fmt.Errorf("%s, %w", ctx.Err(), err)
		}
		return nil, err
	}

	return bins, nil
}

// RunHybridMigrations intelligently runs migrations using external tools for legacy versions
// and embedded migrations for modern versions. This handles the transition from external
// fs-repo-migrations binaries (for repo versions <16) to embedded migrations (for repo versions ≥16).
//
// The function automatically:
// 1. Uses external migrations to get from current version to v16 (if needed)
// 2. Uses embedded migrations for v16+ steps
// 3. Handles pure external, pure embedded, or mixed migration scenarios
//
// Legacy external migrations (repo versions <16) only support HTTPS downloads.
//
// Parameters:
//   - ctx: Context for cancellation and timeouts
//   - targetVer: Target repository version to migrate to
//   - ipfsDir: Path to the IPFS repository directory
//   - allowDowngrade: Whether to allow downgrade migrations
//
// Returns error if migration fails at any step.
func RunHybridMigrations(ctx context.Context, targetVer int, ipfsDir string, allowDowngrade bool) error {
	const embeddedMigrationsMinVersion = 16

	// Get current repo version
	currentVer, err := RepoVersion(ipfsDir)
	if err != nil {
		return fmt.Errorf("could not get current repo version: %w", err)
	}

	var logger = log.New(os.Stdout, "", 0)

	// Check if migration is needed
	if currentVer == targetVer {
		logger.Printf("Repository is already at version %d", targetVer)
		return nil
	}

	// Validate downgrade request
	if targetVer < currentVer && !allowDowngrade {
		return fmt.Errorf("downgrade from version %d to %d requires allowDowngrade=true", currentVer, targetVer)
	}

	// Determine migration strategy based on version ranges
	needsExternal := currentVer < embeddedMigrationsMinVersion
	needsEmbedded := targetVer >= embeddedMigrationsMinVersion

	// Case 1: Pure embedded migration (both current and target ≥ 16)
	if !needsExternal && needsEmbedded {
		return RunEmbeddedMigrations(ctx, targetVer, ipfsDir, allowDowngrade)
	}

	// For cases requiring external migrations, we check if migration binaries
	// are available in PATH before attempting network downloads

	// Case 2: Pure external migration (target < 16)
	if needsExternal && !needsEmbedded {

		// Check for migration binaries in PATH first (for testing/local development)
		migrations, binPaths, err := findMigrations(ctx, currentVer, targetVer)
		if err != nil {
			return fmt.Errorf("could not determine migration paths: %w", err)
		}

		foundAll := true
		for _, migName := range migrations {
			if _, exists := binPaths[migName]; !exists {
				foundAll = false
				break
			}
		}

		if foundAll {
			return runMigrationsFromPath(ctx, migrations, binPaths, ipfsDir, logger, false)
		}

		// Fall back to network download (original behavior)
		migrationCfg, err := ReadMigrationConfig(ipfsDir, "")
		if err != nil {
			return fmt.Errorf("could not read migration config: %w", err)
		}

		// Use existing RunMigration which handles network downloads properly (HTTPS only for legacy migrations)
		fetcher, err := GetMigrationFetcher(migrationCfg.DownloadSources, GetDistPathEnv(CurrentIpfsDist), nil)
		if err != nil {
			return fmt.Errorf("failed to get migration fetcher: %w", err)
		}
		defer fetcher.Close()
		return RunMigration(ctx, fetcher, targetVer, ipfsDir, allowDowngrade)
	}

	// Case 3: Hybrid migration (current < 16, target ≥ 16)
	if needsExternal && needsEmbedded {
		logger.Printf("Starting hybrid migration from version %d to %d", currentVer, targetVer)
		logger.Print("Using hybrid migration strategy: external to v16, then embedded")

		// Phase 1: Use external migrations to get to v16
		logger.Printf("Phase 1: External migration from v%d to v%d", currentVer, embeddedMigrationsMinVersion)

		// Check for external migration binaries in PATH first
		migrations, binPaths, err := findMigrations(ctx, currentVer, embeddedMigrationsMinVersion)
		if err != nil {
			return fmt.Errorf("could not determine external migration paths: %w", err)
		}

		foundAll := true
		for _, migName := range migrations {
			if _, exists := binPaths[migName]; !exists {
				foundAll = false
				break
			}
		}

		if foundAll {
			if err = runMigrationsFromPath(ctx, migrations, binPaths, ipfsDir, logger, false); err != nil {
				return fmt.Errorf("external migration phase failed: %w", err)
			}
		} else {
			migrationCfg, err := ReadMigrationConfig(ipfsDir, "")
			if err != nil {
				return fmt.Errorf("could not read migration config: %w", err)
			}

			// Legacy migrations only support HTTPS downloads
			fetcher, err := GetMigrationFetcher(migrationCfg.DownloadSources, GetDistPathEnv(CurrentIpfsDist), nil)
			if err != nil {
				return fmt.Errorf("failed to get migration fetcher: %w", err)
			}
			defer fetcher.Close()

			if err = RunMigration(ctx, fetcher, embeddedMigrationsMinVersion, ipfsDir, allowDowngrade); err != nil {
				return fmt.Errorf("external migration phase failed: %w", err)
			}
		}

		// Phase 2: Use embedded migrations for v16+
		logger.Printf("Phase 2: Embedded migration from v%d to v%d", embeddedMigrationsMinVersion, targetVer)
		err = RunEmbeddedMigrations(ctx, targetVer, ipfsDir, allowDowngrade)
		if err != nil {
			return fmt.Errorf("embedded migration phase failed: %w", err)
		}

		logger.Printf("Hybrid migration completed successfully: v%d → v%d", currentVer, targetVer)
		return nil
	}

	// Case 4: Reverse hybrid migration (≥16 to <16)
	// Use embedded migrations for ≥16 steps, then external migrations for <16 steps
	logger.Printf("Starting reverse hybrid migration from version %d to %d", currentVer, targetVer)
	logger.Print("Using reverse hybrid migration strategy: embedded to v16, then external")

	// Phase 1: Use embedded migrations from current version down to v16 (if needed)
	if currentVer > embeddedMigrationsMinVersion {
		logger.Printf("Phase 1: Embedded downgrade from v%d to v%d", currentVer, embeddedMigrationsMinVersion)
		err = RunEmbeddedMigrations(ctx, embeddedMigrationsMinVersion, ipfsDir, allowDowngrade)
		if err != nil {
			return fmt.Errorf("embedded downgrade phase failed: %w", err)
		}
	}

	// Phase 2: Use external migrations from v16 to target (if needed)
	if embeddedMigrationsMinVersion > targetVer {
		logger.Printf("Phase 2: External downgrade from v%d to v%d", embeddedMigrationsMinVersion, targetVer)

		// Check for external migration binaries in PATH first
		migrations, binPaths, err := findMigrations(ctx, embeddedMigrationsMinVersion, targetVer)
		if err != nil {
			return fmt.Errorf("could not determine external migration paths: %w", err)
		}

		foundAll := true
		for _, migName := range migrations {
			if _, exists := binPaths[migName]; !exists {
				foundAll = false
				break
			}
		}

		if foundAll {
			if err = runMigrationsFromPath(ctx, migrations, binPaths, ipfsDir, logger, true); err != nil {
				return fmt.Errorf("external downgrade phase failed: %w", err)
			}
		} else {
			migrationCfg, err := ReadMigrationConfig(ipfsDir, "")
			if err != nil {
				return fmt.Errorf("could not read migration config: %w", err)
			}

			// Legacy migrations only support HTTPS downloads
			fetcher, err := GetMigrationFetcher(migrationCfg.DownloadSources, GetDistPathEnv(CurrentIpfsDist), nil)
			if err != nil {
				return fmt.Errorf("failed to get migration fetcher: %w", err)
			}
			defer fetcher.Close()

			if err = RunMigration(ctx, fetcher, targetVer, ipfsDir, allowDowngrade); err != nil {
				return fmt.Errorf("external downgrade phase failed: %w", err)
			}
		}
	}

	logger.Printf("Reverse hybrid migration completed successfully: v%d → v%d", currentVer, targetVer)
	return nil
}

// runMigrationsFromPath runs migrations using binaries found in PATH
func runMigrationsFromPath(ctx context.Context, migrations []string, binPaths map[string]string, ipfsDir string, logger *log.Logger, revert bool) error {
	for _, migName := range migrations {
		binPath, exists := binPaths[migName]
		if !exists {
			return fmt.Errorf("migration binary %s not found in PATH", migName)
		}

		logger.Printf("Running migration %s using binary from PATH: %s", migName, binPath)

		// Run the migration binary directly
		err := runMigration(ctx, binPath, ipfsDir, revert, logger)
		if err != nil {
			return fmt.Errorf("migration %s failed: %w", migName, err)
		}
	}
	return nil
}
