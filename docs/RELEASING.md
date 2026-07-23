# Releasing gdbforge

Automated builds and GitHub Releases are driven by [`.github/workflows/release.yml`](../.github/workflows/release.yml).

## What the workflow does

| Trigger | Result |
|---------|--------|
| Push tag `v*` (e.g. `v1.0.0`) | Cross-build binaries → GitHub Release with assets |
| Push tag with a hyphen (e.g. `v1.0.0-rc.1`) | Same, but marked **prerelease** |
| Actions → **Release** → Run workflow (`dry_run=true`) | Build only; upload workflow **artifacts** (no Release) |

Targets: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64` (`CGO_ENABLED=0`).

Binaries are named `gdbforge-<version>-<os>-<arch>` with a `.sha256` sidecar. The version is stamped via `-ldflags -X main.version=…` (`gdbforge -version`).

## Test without junking `main`

Releases are **tag-driven**, not `main`-push-driven. You can validate on a feature branch first.

### 1. Dry run (safest — no Release entry)

1. Push the workflow file on your feature branch.
2. GitHub → **Actions** → **Release** → **Run workflow**.
3. Leave **dry_run** checked (`true`).
4. Download the job artifacts and smoke-test a binary locally.

Nothing is published under Releases.

### 2. Prerelease tag (optional end-to-end)

From any commit (does **not** require merging to `main`):

```bash
git tag v1.0.0-rc.1
git push origin v1.0.0-rc.1
```

That creates a **pre-release**. After you verify it:

```bash
gh release delete v1.0.0-rc.1 --yes
git push origin :refs/tags/v1.0.0-rc.1
git tag -d v1.0.0-rc.1
```

### 3. Real v1.0.0 (after squash/merge to `main`)

```bash
git checkout main
git pull
git tag -a v1.0.0 -m "gdbforge v1.0.0"
git push origin v1.0.0
```

Watch **Actions → Release**; the GitHub Release page fills in with notes + binaries.

## Notes

- Do **not** put auto-release on every push to `main` — tags keep history clean.
- Docs Pages stay on [docs.yml](../.github/workflows/docs.yml) (`push` to `main` under `docs/`).
- Prefer annotated tags (`git tag -a`) for releases.
