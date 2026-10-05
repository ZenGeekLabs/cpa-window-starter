# CPA Window Starter

**Schedule small requests for your CPA accounts, on your own timetable.**

[简体中文](README_CN.md) · English · [Releases](https://github.com/ZenGeekLabs/cpa-window-starter/releases) · [Report an issue](https://github.com/ZenGeekLabs/cpa-window-starter/issues)

CPA Window Starter is a native [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) plugin for **GPT / Codex** and **Gemini / Antigravity** accounts already connected to CPA. Choose accounts, a model, a time zone, and daily times; CPA sends a short request to each selected account when the time arrives.

> A successful request does not prove that a new quota window has started. Reset rules and available quota remain controlled by the upstream provider. Scheduled requests consume normal account quota.

## Features

- **Independent plans** for Codex and Antigravity: accounts, model, time zone, times, and on/off switch.
- **Your daily schedule**: up to 24 `HH:MM` times, with searchable IANA time zones and daylight saving support. `07:00`, `13:00`, and `19:00` in `Asia/Shanghai` are editable defaults.
- **Existing CPA accounts**: requests are pinned to the selected account through CPA's native host API. Disabled or missing accounts are skipped.
- **Model selection from CPA**: choose from models shared by the selected accounts. The Antigravity plan shows Gemini models.
- **Account groups**: Plus, Team / Business, Pro, and other plans use explicit CPA metadata; unavailable plan information stays “Unknown”.
- **Per-account results**: execution time, status, and upstream-confirmed Codex five-hour reset time when available. One account failure does not stop the remaining accounts.
- **Integrated interface**: Chinese and English, following CPA's language and light/dark appearance. No separate language or theme controls.
- **Server-side scheduling**: close the browser after saving; CPA must remain running. Missed times are not replayed after downtime.

## Install

### Plugin store

Official store inclusion is being submitted; it is not yet guaranteed to appear in the default catalog. Until the listing is merged, add this repository as an additional store source in CPA:

```text
https://raw.githubusercontent.com/ZenGeekLabs/cpa-window-starter/main/registry.json
```

The corresponding CPA configuration is shown below. Merge it into your existing `plugins` block and retain any existing sources and plugin settings:

```yaml
plugins:
  enabled: true
  dir: plugins
  store-sources:
    - https://raw.githubusercontent.com/ZenGeekLabs/cpa-window-starter/main/registry.json
```

Refresh the plugin store, select **CPA Window Starter · 账号定时请求**, install, and enable it. Once the official listing is merged, the same plugin can be installed from the default store.

### Manual installation

Download the matching ZIP and `checksums.txt` from the [latest release](https://github.com/ZenGeekLabs/cpa-window-starter/releases/latest).

| CPA server platform | Archive suffix | Library inside |
| --- | --- | --- |
| macOS, Intel | `darwin_amd64.zip` | `cpa-window-starter.dylib` |
| macOS, Apple Silicon | `darwin_arm64.zip` | `cpa-window-starter.dylib` |
| Linux, x86_64 with glibc | `linux_amd64.zip` | `cpa-window-starter.so` |
| Linux, ARM64 with glibc | `linux_arm64.zip` | `cpa-window-starter.so` |
| Windows, x86_64 | `windows_amd64.zip` | `cpa-window-starter.dll` |

Select the platform of the **machine running CPA**, regardless of the browser's operating system. All five store-required targets are supplied. Linux packages target glibc 2.17 or later; musl / Alpine is not supported. macOS packages target macOS 13 or later; the ABI tests run on macOS 15. Windows verification runs on Windows Server 2022.

1. Compare the archive's SHA-256 with its entry in `checksums.txt` (`shasum -a 256 <archive>` on macOS, `sha256sum <archive>` on Linux, or PowerShell `Get-FileHash <archive> -Algorithm SHA256` on Windows).
2. Extract the library into CPA's configured plugin directory. Back up an existing library before replacing it.
3. Enable plugins and this plugin in CPA. A configuration example is available in [config.example.yaml](config.example.yaml).
4. Load or reload the plugin from CPA's plugin management page, then open its menu entry. If your host requires a restart, schedule it when no requests are in progress.

Requires a CPA build with native plugin support and the `host.auth.list` / `host.model.execute` callbacks. Integration work has been exercised with CPA 8.0.10 on macOS and 8.0.13 on Linux; this is not a claim of compatibility with every older version or platform.

## Quick start

1. Open the plugin from your logged-in CPA management page.
2. Choose **GPT / Codex** or **Gemini / Antigravity**.
3. Select the accounts to schedule. Choose a model from the available list.
4. Set a time zone and daily times, turn on automatic requests, and click **Save plan**.
5. Optionally click **Run once** to send a real request immediately; inspect the per-account results.

New installations have no selected accounts, so they do not send requests until configured. The Antigravity schedule is also off by default. Each provider's plan is saved separately.

The page reuses a compatible remembered CPA login on the same origin. There is no second management-key field. If that login is unavailable, the page links to CPA's native plugin settings. Account tokens remain managed by CPA; the plugin uses account IDs and native host callbacks.

## Quota behavior

The request asks the model to reply with `OK`. It is a normal model request and may consume tokens and request quota.

For a provider whose window starts on first use, a scheduled request may start an idle window. It cannot move an already active window, refill exhausted quota, or override weekly limits. In particular, **a request at 07:00 does not guarantee a reset at 12:00**. Codex reset times are shown only when the upstream response provides a recognized five-hour reset header; absent that, the time remains unknown.

Gemini / Antigravity scheduling sends requests using your existing Antigravity authentication files. Its quota recovery mechanism is not inferred from request success; this plugin does not promise a five-hour cycle for Gemini.

## Persistence and operation

CPA saves the plan in its plugin configuration. Execution records use `state_file` (default: `plugins/state/cpa-window-starter.json`); the Antigravity record file appends `.antigravity`. Keep these locations writable and persistent across CPA restarts or container replacements.

Automatic attempts are recorded before dispatch to avoid repeating the same scheduled slot after a restart. A missed or interrupted attempt is not automatically replayed. **Run once** is an explicit new request.

This is a native library loaded into CPA's process. It needs no standalone daemon, browser extension, or separate provider login.

## Development

Requirements: Go 1.24 or later, a C compiler, Node.js with `node:test`, and Python 3 for packaging / ABI checks. macOS builds also need Xcode Command Line Tools.

```sh
make test
make build
python3 tests/abi_smoke.py dist/darwin_arm64/cpa-window-starter.dylib --work-dir .build/abi
```

Use the `.so` path on Linux or `.dll` path on Windows for the ABI check. Windows builds use `sh scripts/build.sh windows amd64` in Git Bash with MinGW-w64 GCC; Intel Mac builds use `sh scripts/build.sh darwin amd64`. Native Linux builds use `sh scripts/build.sh linux amd64`; Linux cross-compilation from macOS uses `ZIG=/path/to/zig sh scripts/build.sh linux amd64` (GNU / glibc target).

```sh
make demo
```

Open <http://127.0.0.1:18418/v0/resource/plugins/cpa-window-starter/status?demo=1>. The local demo uses fictional accounts, models, and responses and sends no real provider requests. Its displayed reset times are simulated.

The CI workflow builds and loads each target on its matching OS and architecture, then packages only after all five succeed. A separate workflow downloads each published package and repeats its checksum, architecture, and native ABI checks.

After building all five supported platforms, run `python3 scripts/package.py`. Release archives and `checksums.txt` are written to `release/`. See [release verification](docs/release-verification.md) and the [changelog](CHANGELOG.md).

## License and attribution

Code is licensed under [MIT](LICENSE), © 2026 ZenGeekLabs / 禅极科技.

OpenAI and Gemini names and artwork identify their respective services and remain the property of their owners. They are not covered by this project's MIT grant, and their inclusion does not imply endorsement. Asset provenance: [OpenAI](docs/openai-asset-source.md), [Gemini](docs/gemini-asset-source.md).
