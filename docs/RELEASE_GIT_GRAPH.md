# Expected git graph

![Release git graph](https://ipfs.io/ipfs/bafkreidcmmgmfjoasjimw66mgbl7vnyvjf4ag4y67urj7ejnkkhm7nxx5m)

## Continued branches

- *master* this is development branch
- *release* this branch list all the release commits
  It's important that `git show release~$X` shows version releases first.

## Creating this graph:

 1. Making the RC. From the ref you want to make release do:
   1. `git checkout -b release-vX.Y.Z` (create the branch this release is gonna be worked on)
   1. Make changelogs if you havn't done so yet.
     Changelogs are free flowing and can be done earlier on master if you want.
     It is smart to add changelogs when you create a new big feature while or just after merging this feature instead of 2 weeks later when doing the release.
   1. Update the `version.go` file (must be it's own commit with nothing else).
   1. `git tag -s vX.Y.Z-rcAAA` create signed tags
   1. `git push origin vX.Y.Z-rcAAA` push the tag (never use `git push --tags`)
 1. Making the final release (/ an other RC)
   1. Doing Back-Ports.
     - If you want to pull anything from `$(git merge-base release-vX.Y.Z $REF)..$REF` do a merge commit from `$REF` into `release-vX.Y.Z`.
       It's important to use merge for the merge algorithm to not see this as conflicts later.
       If you don't understand that sentence just skip this step and go to the next one.
     - For other commits do `git cherry-pick -x $COMMIT`.
       Using `-x` will add `(cherry picked from commit ...)`, this make the merge algorithm to not see this as conflicts later.
   1. Finalize changelogs.
   1. Update the `version.go` file (must be it's own commit with nothing else).
   1. Merge `release-vX.Y.Z` into `release` branch (USE A MERGE COMMIT).
   1. `git tag -s vX.Y.Z` create a signed tag on that merge commit
   1. On a new temporary branch update `version.go` for the next release `-dev`.
   1. Using a merge commit (to avoid conflicts one last time) merge back that temporary thing into `master`.
