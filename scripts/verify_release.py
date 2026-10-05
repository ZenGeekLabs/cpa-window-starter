#!/usr/bin/env python3
"""Verify a release archive before extracting it for native ABI testing."""
import argparse
import hashlib
import pathlib
import re
import zipfile

from release_platforms import PLATFORMS, library_name, validate_library


def verify(directory, tag, target, extract_to=None):
    if not re.fullmatch(r'v[0-9]+(?:\.[0-9]+)+', tag):
        raise ValueError('Invalid release tag')
    name = f'cpa-window-starter_{tag[1:]}_{target}.zip'
    checksums = {}
    for line in (directory / 'checksums.txt').read_text().splitlines():
        match = re.fullmatch(r'([0-9a-f]{64})  ([A-Za-z0-9_.-]+)', line)
        if not match or match[2] in checksums:
            raise ValueError('Malformed or duplicate checksum entry')
        checksums[match[2]] = match[1]
    required = {f'cpa-window-starter_{tag[1:]}_{p}.zip' for p in PLATFORMS}
    if not required.issubset(checksums):
        raise ValueError('Checksum file must cover all five store platforms')
    archive = directory / name
    if hashlib.sha256(archive.read_bytes()).hexdigest() != checksums[name]:
        raise ValueError('Checksum mismatch: ' + name)
    with zipfile.ZipFile(archive) as bundle:
        if bundle.namelist() != [library_name(target)]:
            raise ValueError('Expected exactly one correctly named root library')
        data = bundle.read(library_name(target))
    validate_library(data, target)
    if extract_to is not None:
        extract_to.mkdir(parents=True, exist_ok=True)
        (extract_to / library_name(target)).write_bytes(data)
    return name


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument('--directory', type=pathlib.Path, default=pathlib.Path('release'))
    parser.add_argument('--tag', required=True)
    parser.add_argument('--target', choices=PLATFORMS, required=True)
    parser.add_argument('--extract-to', type=pathlib.Path)
    args = parser.parse_args()
    name = verify(args.directory, args.tag, args.target, args.extract_to)
    print(name + ': SHA-256, archive layout, and library architecture verified')
