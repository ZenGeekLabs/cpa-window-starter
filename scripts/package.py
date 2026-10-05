#!/usr/bin/env python3
"""Package local native builds and source without publishing anything."""
import argparse
import hashlib
import pathlib
import re
import zipfile

from release_platforms import PLATFORMS, library_name, validate_library

root = pathlib.Path(__file__).resolve().parents[1]
parser = argparse.ArgumentParser()
parser.add_argument('--output-dir', default=str(root / 'release'))
args = parser.parse_args()
output = pathlib.Path(args.output_dir).resolve()
output.mkdir(parents=True, exist_ok=True)
version = re.search(r'const Version = "([^"]+)"', (root / 'internal/starter/config.go').read_text()).group(1)
artifacts = []
# Validate all five targets before writing any package or checksum file.
libraries = {target: root / 'dist' / target / library_name(target) for target in PLATFORMS}
for target, library in libraries.items():
    if not library.is_file():
        raise SystemExit(f'Missing native build: {library}')
    validate_library(library.read_bytes(), target)
for target, library in libraries.items():
    archive = output / f'cpa-window-starter_{version}_{target}.zip'
    with zipfile.ZipFile(archive, 'w', zipfile.ZIP_DEFLATED, compresslevel=9) as bundle:
        bundle.write(library, library.name)
    artifacts.append(archive)
source = output / f'cpa-window-starter_{version}_source.zip'
skip = {'dist', '.git', '.build', '.cache', '.demo', '__pycache__', 'work', 'plugins', 'release'}
with zipfile.ZipFile(source, 'w', zipfile.ZIP_DEFLATED, compresslevel=9) as bundle:
    for path in sorted(root.rglob('*')):
        relative = path.relative_to(root)
        if path.is_file() and not any(part in skip for part in relative.parts):
            bundle.write(path, pathlib.PurePosixPath('cpa-window-starter') / relative)
artifacts.append(source)
checksums = output / f'cpa-window-starter_{version}_SHA256SUMS.txt'
checksum_text = ''.join(f'{hashlib.sha256(path.read_bytes()).hexdigest()}  {path.name}\n' for path in artifacts)
checksums.write_text(checksum_text)
(output / 'checksums.txt').write_text(checksum_text)
for artifact in artifacts:
    print(f'{artifact.name}: {artifact.stat().st_size:,} bytes')
print(checksums.name)
print("checksums.txt")
