# IPFS Repository Migrations

This directory contains the migration system for IPFS repositories, handling both embedded and external migrations.

## Migration System Overview

### Embedded vs External Migrations

Starting from **repo version 17**, Kubo uses **embedded migrations** that are built into the binary, eliminating the need to download external migration tools.

- **Repo versions <17**: Use external binary migrations downloaded from fs-repo-migrations
- **Repo version 17+**: Use embedded migrations built into Kubo

### Migration Functions

#### `migrations.RunEmbeddedMigrations()`
- **Purpose**: Runs migrations that are embedded directly in the Kubo binary
- **Scope**: Handles repo version 17+ migrations
- **Performance**: Fast execution, no network downloads required
- **Dependencies**: Self-contained, uses only Kubo's internal dependencies
- **Usage**: Primary migration method for modern repo versions

**Parameters**:
- `ctx`: Context for cancellation and timeouts
- `targetVersion`: Target repository version to migrate to
- `repoPath`: Path to the IPFS repository directory
- `allowDowngrade`: Whether to allow downgrade migrations

```go
err = migrations.RunEmbeddedMigrations(ctx, targetVersion, repoPath, allowDowngrade)
if err != nil {
    // Handle migration failure, may fall back to external migrations
}
```

#### `migrations.RunMigration()` with `migrations.ReadMigrationConfig()`
- **Purpose**: Runs external binary migrations downloaded from fs-repo-migrations
- **Scope**: Handles legacy repo versions <17 and serves as fallback
- **Performance**: Slower due to network downloads and external process execution
- **Dependencies**: Requires fs-repo-migrations binaries and network access
- **Usage**: Fallback method for legacy migrations

```go
// Read migration configuration for external migrations
migrationCfg, err := migrations.ReadMigrationConfig(repoPath, configFile)
fetcher, err := migrations.GetMigrationFetcher(migrationCfg.DownloadSources, ...)
err = migrations.RunMigration(ctx, fetcher, targetVersion, repoPath, allowDowngrade)
```

## Migration Flow in Daemon Startup

1. **Primary**: Try embedded migrations first (`RunEmbeddedMigrations`)
2. **Fallback**: If embedded migration fails, fall back to external migrations (`RunMigration`)
3. **Legacy Support**: External migrations ensure compatibility with older repo versions

## Directory Structure

```
repo/fsrepo/migrations/
├── README.md                    # This file
├── embedded.go                  # Embedded migration system
├── embedded_test.go             # Tests for embedded migrations
├── migrations.go                # External migration system
├── fs-repo-16-to-17/           # First embedded migration (16→17)
│   ├── migration/
│   │   ├── migration.go        # Migration logic
│   │   └── migration_test.go   # Migration tests
│   ├── atomicfile/
│   │   └── atomicfile.go       # Atomic file operations
│   ├── main.go                 # Standalone migration binary
│   └── README.md               # Migration-specific documentation
└── [other migration utilities]
```

## Adding New Embedded Migrations

To add a new embedded migration (e.g., fs-repo-17-to-18):

1. **Create migration package**: `fs-repo-17-to-18/migration/migration.go`
2. **Implement interface**: Ensure your migration implements the `EmbeddedMigration` interface
3. **Register migration**: Add to `embeddedMigrations` map in `embedded.go`
4. **Add tests**: Create comprehensive tests for your migration logic
5. **Update repo version**: Increment `RepoVersion` in `fsrepo.go`

```go
// In embedded.go
var embeddedMigrations = map[string]EmbeddedMigration{
    "fs-repo-16-to-17": &mg16.Migration{},
    "fs-repo-17-to-18": &mg17.Migration{}, // Add new migration
}
```

## Migration Requirements

Each embedded migration must:
- Implement the `EmbeddedMigration` interface
- Be reversible with proper backup handling
- Use atomic file operations to prevent corruption
- Preserve user customizations
- Include comprehensive tests
- Follow the established naming pattern

## External Migration Support

External migrations are maintained for:
- **Backward compatibility** with repo versions <17
- **Fallback mechanism** if embedded migrations fail
- **Legacy installations** that cannot be upgraded directly

The external migration system will continue to work but is not the preferred method for new migrations.

## Security and Safety

All migrations (embedded and external) include:
- **Atomic operations**: Prevent repository corruption
- **Backup creation**: Allow rollback if migration fails
- **Version validation**: Ensure migrations run on correct repo versions
- **Error handling**: Graceful failure with informative messages
- **User preservation**: Maintain custom configurations during migration

## Testing

Test both embedded and external migration systems:

```bash
# Test embedded migrations
go test ./repo/fsrepo/migrations/ -run TestEmbedded

# Test specific migration
go test ./repo/fsrepo/migrations/fs-repo-16-to-17/migration/

# Test migration registration
go test ./repo/fsrepo/migrations/ -run TestHasEmbedded
```