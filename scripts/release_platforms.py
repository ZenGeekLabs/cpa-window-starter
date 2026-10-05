"""The required store platform matrix and binary format checks."""
import struct

PLATFORMS = {
    'darwin_amd64': ('dylib', 'macos-15-intel'),
    'darwin_arm64': ('dylib', 'macos-15'),
    'linux_amd64': ('so', 'ubuntu-24.04'),
    'linux_arm64': ('so', 'ubuntu-24.04-arm'),
    'windows_amd64': ('dll', 'windows-2022'),
}


def library_name(target):
    return 'cpa-window-starter.' + PLATFORMS[target][0]


def validate_library(data, target):
    """Reject a renamed or mismatched binary before it can be published."""
    if target not in PLATFORMS:
        raise ValueError('Unsupported platform: ' + target)
    valid = False
    if target.startswith('darwin_') and len(data) >= 16:
        machine = 0x01000007 if target.endswith('amd64') else 0x0100000C
        valid = (data[:4] == b'\xcf\xfa\xed\xfe' and
                 struct.unpack_from('<I', data, 4)[0] == machine and
                 struct.unpack_from('<I', data, 12)[0] == 6)  # MH_DYLIB
    elif target.startswith('linux_') and len(data) >= 20:
        machine = 62 if target.endswith('amd64') else 183
        valid = (data[:6] == b'\x7fELF\x02\x01' and
                 struct.unpack_from('<HH', data, 16) == (3, machine))  # ET_DYN
    elif target == 'windows_amd64' and len(data) >= 64 and data[:2] == b'MZ':
        offset = struct.unpack_from('<I', data, 60)[0]
        if offset + 26 <= len(data):
            valid = (data[offset:offset+4] == b'PE\0\0' and
                     struct.unpack_from('<H', data, offset+4)[0] == 0x8664 and
                     struct.unpack_from('<H', data, offset+22)[0] & 0x2000 and
                     struct.unpack_from('<H', data, offset+24)[0] == 0x20B)
    if not valid:
        raise ValueError('Wrong dynamic-library format or architecture for ' + target)
