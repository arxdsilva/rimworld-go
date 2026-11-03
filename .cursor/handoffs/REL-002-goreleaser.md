# REL-002: GoReleaser publishes the release

The user asked for more information on each release. The build matrix stays as it was, because the binaries need cgo per OS and a macOS universal join; GoReleaser (`builds: skip`) only takes over the publish step of `.github/workflows/release.yml`.

## What changed
- `.goreleaser.yaml`: uploads `release-artifacts/*` (gitignored), `checksums.txt`, a changelog grouped by commit type from the previous tag, the old install notes as the header, `prerelease: auto`.
- `scripts/release_details.sh`: a collapsed section per commit with a body, passed with `--release-footer`, because `.Message` holds only a commit's first line.
- Release job: full history (`fetch-depth: 0`), artifacts merged into one directory, `goreleaser-action@v7` on `~> v2`.

## Verified
- `goreleaser check`, `actionlint` and `shellcheck` are clean.
- A dry run in a scratch clone (`use: git`, `--skip=publish`) wrote the grouped `CHANGELOG.md` with the details and checksums for all three archives.
- Not verified: the upload and `use: github`. Both run for the first time on the next tag.

## Gaps
- `release_details.sh` has no size cap; GitHub limits a release body to about 125k characters, so a very long tag range could fail the publish.
