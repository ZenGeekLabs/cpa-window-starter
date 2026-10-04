# Release verification

## v0.2.6

The public release keeps the accepted 0.2.5 application behavior, updates its repository metadata and version, and uses stripped native builds. An older static-resource test fixture was updated to include the theme bootstrap asset required by 0.2.5.

### Local checks

- `go test -race ./...`: passed.
- `node --test tests/*.test.cjs`: 26 passed.
- macOS arm64 native build and fake-host ABI checks: registration, public assets, exact-account dispatch, disabled-account skip, per-account failure handling, Codex reset parsing, and independent Antigravity dispatch.
- Linux amd64 / glibc cross-build; archive layout, version metadata, and SHA-256 checks.
- Browser demo uses fictional accounts and mock provider responses only.

### Published package verification

[GitHub Actions](https://github.com/ZenGeekLabs/cpa-window-starter/actions) runs the source tests and a native Linux build. A separate release workflow downloads the published Linux ZIP, verifies its checksum and structure, and exercises that exact library through the fake-host ABI harness. Check the workflow result for the release being installed.

### Scope

Previous integration testing used CPA 8.0.10 on macOS and CPA 8.0.13 on Linux. These checks verify plugin integration and request mechanics. They do not establish an upstream quota reset policy, guarantee five-hour activation, or imply that new release binaries have been deployed to any existing server. Live-account requests are not part of the public CI suite.

Only macOS arm64 and Linux amd64 / glibc binaries are distributed. Other architectures, Windows, older macOS versions, and musl / Alpine are not verified.
