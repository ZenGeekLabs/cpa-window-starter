"""Release gates: missing platforms, wrong architectures, and damaged archives."""
import hashlib
import pathlib
import struct
import sys
import tempfile
import unittest
import zipfile

sys.path.insert(0, str(pathlib.Path(__file__).resolve().parents[1] / 'scripts'))
from release_platforms import PLATFORMS, validate_library
from verify_release import verify


def elf(machine=62):
    data = bytearray(64)
    data[:6] = b'\x7fELF\x02\x01'
    struct.pack_into('<HH', data, 16, 3, machine)
    return bytes(data)


class ReleaseValidation(unittest.TestCase):
    def fixture(self, directory, data=None, member='cpa-window-starter.so'):
        archive = directory / 'cpa-window-starter_0.2.7_linux_amd64.zip'
        with zipfile.ZipFile(archive, 'w') as bundle:
            bundle.writestr(member, elf() if data is None else data)
        entries = {f'cpa-window-starter_0.2.7_{p}.zip': '0'*64 for p in PLATFORMS}
        entries[archive.name] = hashlib.sha256(archive.read_bytes()).hexdigest()
        (directory/'checksums.txt').write_text(''.join(f'{v}  {k}\n' for k,v in entries.items()))
        return archive

    def test_wrong_architecture_is_rejected(self):
        validate_library(elf(), 'linux_amd64')
        with self.assertRaises(ValueError):
            validate_library(elf(), 'linux_arm64')
        with self.assertRaises(ValueError):
            validate_library(b'not a library', 'windows_amd64')

    def test_checksum_and_safe_layout_before_extracting(self):
        with tempfile.TemporaryDirectory() as temp:
            root = pathlib.Path(temp)
            archive = self.fixture(root)
            verify(root, 'v0.2.7', 'linux_amd64', root/'extracted')
            self.assertEqual((root/'extracted/cpa-window-starter.so').read_bytes(), elf())
            archive.write_bytes(archive.read_bytes() + b'changed')
            with self.assertRaisesRegex(ValueError, 'Checksum mismatch'):
                verify(root, 'v0.2.7', 'linux_amd64')
            self.fixture(root, member='../cpa-window-starter.so')
            with self.assertRaisesRegex(ValueError, 'root library'):
                verify(root, 'v0.2.7', 'linux_amd64')

    def test_missing_store_platform_is_rejected(self):
        with tempfile.TemporaryDirectory() as temp:
            root = pathlib.Path(temp)
            self.fixture(root)
            path = root/'checksums.txt'
            path.write_text('\n'.join(line for line in path.read_text().splitlines() if 'windows_amd64' not in line)+'\n')
            with self.assertRaisesRegex(ValueError, 'all five'):
                verify(root, 'v0.2.7', 'linux_amd64')


if __name__ == '__main__':
    unittest.main()
