#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
os_name="${1:-$(go env GOOS)}"
arch_name="${2:-$(go env GOARCH)}"
case "$os_name" in
 darwin)
  extension=dylib
  if command -v xcrun >/dev/null 2>&1; then
   SDKROOT="$(xcrun --sdk macosx --show-sdk-path)"
   export SDKROOT
   CC="$(xcrun --find clang)"
   export CC
   MACOSX_DEPLOYMENT_TARGET="${MACOSX_DEPLOYMENT_TARGET:-13.0}"
   export MACOSX_DEPLOYMENT_TARGET
  fi
  ;;
 linux) extension=so ;;
 *) printf '%s\n' 'Supported targets: darwin, linux' >&2; exit 1 ;;
esac
build_dir="dist/${os_name}_${arch_name}"
build_ldflags="-s -w"
if [ "$os_name" = linux ]; then
 build_ldflags="$build_ldflags -extldflags=-Wl,--strip-debug"
fi
mkdir -p "$build_dir"
if [ "$os_name" = linux ] && [ "$(go env GOOS)" != linux ]; then
 : "${ZIG:?Set ZIG to the official zig executable for Linux cross compilation}"
 case "$arch_name" in amd64) zig_target=x86_64-linux-gnu.2.17 ;; arm64) zig_target=aarch64-linux-gnu.2.17 ;; *) exit 1 ;; esac
 export CC="$ZIG cc -target $zig_target"
 export CXX="$ZIG c++ -target $zig_target"
fi
CGO_ENABLED=1 GOOS="$os_name" GOARCH="$arch_name" go build -trimpath -ldflags="$build_ldflags" -buildmode=c-shared -o "$build_dir/cpa-window-starter.$extension" .
printf '%s\n' "$build_dir/cpa-window-starter.$extension"
