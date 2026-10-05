#!/bin/sh
# Install the official pinned compiler into CI's temporary directory only.
set -eu
: "${RUNNER_TEMP:?This helper is intended for GitHub Actions}"
: "${GITHUB_ENV:?GitHub Actions environment file is required}"
case "$(uname -m)" in
 x86_64) zig_arch=x86_64; zig_sha=70e49664a74374b48b51e6f3fdfbf437f6395d42509050588bd49abe52ba3d00 ;;
 aarch64) zig_arch=aarch64; zig_sha=ea4b09bfb22ec6f6c6ceac57ab63efb6b46e17ab08d21f69f3a48b38e1534f17 ;;
 *) exit 1 ;;
esac
zig_name="zig-${zig_arch}-linux-0.16.0"
archive="$RUNNER_TEMP/$zig_name.tar.xz"
curl --fail --location --retry 3 "https://ziglang.org/download/0.16.0/$zig_name.tar.xz" --output "$archive"
printf '%s  %s\n' "$zig_sha" "$archive" | sha256sum --check
tar -xf "$archive" -C "$RUNNER_TEMP"
printf 'ZIG=%s/%s/zig\n' "$RUNNER_TEMP" "$zig_name" >> "$GITHUB_ENV"
