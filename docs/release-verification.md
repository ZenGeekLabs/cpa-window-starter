# Release verification

## v0.2.7

This release expands the supported binary matrix to all five targets required by the store. Scheduling, account selection, quota interpretation, and UI behavior are unchanged from v0.2.6.

| Target | Build and ABI test runner | Package library |
| --- | --- | --- |
| darwin_amd64 | macos-15-intel | cpa-window-starter.dylib |
| darwin_arm64 | macos-15 | cpa-window-starter.dylib |
| linux_amd64 | ubuntu-24.04 | cpa-window-starter.so |
| linux_arm64 | ubuntu-24.04-arm | cpa-window-starter.so |
| windows_amd64 | windows-2022 | cpa-window-starter.dll |

### Release gates

The [build workflow](../.github/workflows/ci.yml) runs Go race tests, frontend tests, packaging validation tests, and the real dynamic-library ABI harness on each matching OS / architecture. Linux release builds use the pinned official Zig compiler with a glibc 2.17 target. Windows uses MinGW-w64 with static compiler-runtime linkage so no additional MinGW DLL is required by the single-file package.

Packaging starts only after all five native jobs pass. It requires every library, checks Mach-O / ELF / PE architecture, and places exactly one correctly named library at the root of each platform ZIP. `checksums.txt` covers all five platform archives and the source archive.

The [published-package workflow](../.github/workflows/verify-release.yml) downloads each release ZIP on its matching runner, checks SHA-256, requires the complete five-platform checksum matrix, validates archive structure and architecture, then loads that exact downloaded library with `ctypes` and exercises the host callbacks. Current results are visible in [GitHub Actions](https://github.com/ZenGeekLabs/cpa-window-starter/actions).

### ABI checks

- CPA C ABI registration and static web resources.
- Exact-account pinning, disabled-account skip, and manual request dispatch.
- Per-account failure handling and preserved host HTTP error status.
- Codex reset header parsing and independent Antigravity dispatch.
- Persistent execution records and host buffer ownership.

All automated requests use fictional accounts and fake host callbacks. These tests do not establish an upstream quota reset policy or guarantee five-hour activation.

### Runtime scope

Earlier real CPA integration used version 8.0.10 on macOS and 8.0.13 on Linux. New platform coverage verifies library loading and request mechanics through the ABI harness; it does not claim a live-account end-to-end CPA test on every OS. macOS libraries target 13 or later and are tested on 15. Linux binaries target glibc; musl / Alpine is not supported. Windows libraries are tested on Windows Server 2022. Existing user servers are not upgraded by publishing a release.
