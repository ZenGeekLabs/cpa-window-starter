#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
os_name="${1:-$(go env GOOS)}"
arch_name="${2:-$(go env GOARCH)}"
case "${os_name}_${arch_name}" in
 darwin_amd64|darwin_arm64|linux_amd64|linux_arm64|windows_amd64) ;;
 *) printf '%s\n' 'Supported targets: darwin amd64/arm64, linux amd64/arm64, windows amd64' >&2; exit 1 ;;
esac
build_ldflags="-s -w"
case "$os_name" in
 darwin)
  extension=dylib
  if command -v xcrun >/dev/null 2>&1; then
   SDKROOT="$(xcrun --sdk macosx --show-sdk-path)"
   CC="$(xcrun --find clang)"
   MACOSX_DEPLOYMENT_TARGET="${MACOSX_DEPLOYMENT_TARGET:-13.0}"
   export SDKROOT CC MACOSX_DEPLOYMENT_TARGET
  fi
  ;;
 linux)
  extension=so
  build_ldflags="$build_ldflags -extldflags=-Wl,--strip-debug"
  if [ -n "${ZIG:-}" ]; then
   case "$arch_name" in
    amd64) zig_target=x86_64-linux-gnu.2.17 ;;
    arm64) zig_target=aarch64-linux-gnu.2.17 ;;
   esac
   CC="$ZIG cc -target $zig_target"
   CXX="$ZIG c++ -target $zig_target"
   export CC CXX
  elif [ "$(go env GOOS)_$(go env GOARCH)" != "${os_name}_${arch_name}" ]; then
   : "${CC:?Set CC to a matching cross compiler, or set ZIG}"
  fi
  ;;
 windows)
  extension=dll
  # Keep the store's single-DLL package independent of MinGW runtime DLLs.
  build_ldflags="$build_ldflags -extldflags=-static"
  if [ "$(go env GOOS)" != windows ]; then
   CC="${CC:-x86_64-w64-mingw32-gcc}"
   export CC
  fi
  ;;
esac
build_dir="dist/${os_name}_${arch_name}"
mkdir -p "$build_dir"
CGO_ENABLED=1 GOOS="$os_name" GOARCH="$arch_name" go build -trimpath -ldflags="$build_ldflags" -buildmode=c-shared -o "$build_dir/cpa-window-starter.$extension" .
printf '%s\n' "$build_dir/cpa-window-starter.$extension"
