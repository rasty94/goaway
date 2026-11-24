# Contributing

When contributing to this repository, please first discuss the change you wish to make via an issue with the owners of this repository before making any changes. Once the details has been cleared and an approval is given by a project maintainer, either you or the maintainer starts with an implementation.

### Standards for contributions

- Your reported issue has been approved by a project maintainer to be implemented.
- All commits must be semantic, [read more here](https://www.conventionalcommits.org/en/v1.0.0/). You can find the accepted types [here](https://github.com/rasty94/goaway/blob/main/client/commitlint.config.ts).
- Tests, linting of server/client code and commits must all pass.

### Our Responsibilities

Project maintainers have the right and responsibility to remove, edit, or
reject comments, commits, code, wiki edits, issues, and other contributions, or to temporarily/permanently ban any contributor for other behaviors that can be deemed unfitting.

### Releases

Releases are cut manually by maintainers:

1. Update `CHANGELOG.md` and bump `version`/`appVersion` in `charts/goaway/Chart.yaml`.
2. Commit `chore(release): prepare vX.Y.Z` on `main`.
3. Push the tag: `git tag vX.Y.Z && git push origin vX.Y.Z`.

The `Release` workflow re-runs CI, checks the chart version matches the tag, publishes binaries with GoReleaser and pushes the multi-arch image to `ghcr.io/rasty94/goaway`.
