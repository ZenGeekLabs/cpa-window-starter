# Changelog

## 0.2.7 — 2026-10-05

- Add macOS Intel (`darwin_amd64`), Linux ARM64 (`linux_arm64`), and Windows x64 (`windows_amd64`) packages alongside the existing two targets.
- Build and run the native ABI harness on all five matching GitHub runner platforms before packaging.
- Verify all published platform packages by downloading, checking SHA-256 and binary architecture, and loading the actual library.
- Refuse incomplete five-platform distributions and incorrect binary / archive formats.
- Update bilingual installation and build documentation. Scheduling and UI behavior are unchanged.

## 0.2.6 — 2026-10-05

First public release of CPA Window Starter.

- Configurable daily, account-pinned requests for Codex and Gemini / Antigravity.
- Independent provider plans, global IANA time zone selection, CPA model selection, and account grouping.
- Chinese / English interface following CPA language and appearance, including initial-page theme application.
- Per-account execution history and upstream-confirmed Codex reset times where available.
- Public repository metadata, bilingual documentation, a custom store registry, macOS arm64 and Linux amd64 packages, and SHA-256 checksums.

This release retains the scheduling and interface behavior accepted in the private 0.2.5 build. It updates the version and repository metadata for public distribution. Earlier 0.1.x–0.2.5 versions were local development builds.
