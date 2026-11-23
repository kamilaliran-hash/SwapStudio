#!/usr/bin/env python3
"""Write a Windows resource object (.syso) holding an icon and an application manifest.

Go's linker picks up any *_windows_amd64.syso file in the package folder and puts it in the
.exe, so the program gets its icon in Explorer and runs without asking for admin rights.

usage: make_syso.py icon.ico out.syso [manifest.xml]
"""
import struct
import sys

RT_ICON, RT_GROUP_ICON, RT_MANIFEST = 3, 14, 24
LANG = 0x0409


def read_ico(path):
    data = open(path, 'rb').read()
    _, typ, count = struct.unpack_from('<HHH', data, 0)
    assert typ == 1, 'not an .ico file'
    images = []
    for i in range(count):
        w, h, colors, _, planes, bits, size, off = struct.unpack_from('<BBBBHHII', data, 6 + 16 * i)
        images.append(dict(w=w, h=h, colors=colors, planes=planes, bits=bits, data=data[off:off + size]))
    return images


def build(ico_path, manifest=None):
    images = read_ico(ico_path)
    grp = struct.pack('<HHH', 0, 1, len(images))
    for i, im in enumerate(images, 1):
        grp += struct.pack('<BBBBHHIH', im['w'], im['h'], im['colors'], 0,
                           im['planes'], im['bits'], len(im['data']), i)
    # resources: {type: [(id, bytes)]}, types sorted by id as Windows requires
    res = {RT_ICON: [(i, im['data']) for i, im in enumerate(images, 1)], RT_GROUP_ICON: [(1, grp)]}
    if manifest:
        res[RT_MANIFEST] = [(1, manifest)]
    types = sorted(res)

    def dir_header(n):
        return struct.pack('<IIHHHH', 0, 0, 0, 0, 0, n)

    # layout: root dir, then one name-dir per type, then one lang-dir per resource, data entries, raw data
    n_res = sum(len(res[t]) for t in types)
    off = 16 + 8 * len(types)
    name_dir_off = {}
    for t in types:
        name_dir_off[t] = off
        off += 16 + 8 * len(res[t])
    lang_dir_off = {}
    for t in types:
        for rid, _ in res[t]:
            lang_dir_off[(t, rid)] = off
            off += 16 + 8
    data_entry_off = {}
    for t in types:
        for rid, _ in res[t]:
            data_entry_off[(t, rid)] = off
            off += 16
    raw_off = {}
    for t in types:
        for rid, blob in res[t]:
            off = (off + 7) & ~7
            raw_off[(t, rid)] = off
            off += len(blob)
    total = (off + 7) & ~7

    buf = bytearray(total)
    relocs = []
    buf[0:16] = dir_header(len(types))
    for i, t in enumerate(types):
        struct.pack_into('<II', buf, 16 + 8 * i, t, 0x80000000 | name_dir_off[t])
    for t in types:
        o = name_dir_off[t]
        buf[o:o + 16] = dir_header(len(res[t]))
        for j, (rid, _) in enumerate(res[t]):
            struct.pack_into('<II', buf, o + 16 + 8 * j, rid, 0x80000000 | lang_dir_off[(t, rid)])
    for t in types:
        for rid, blob in res[t]:
            o = lang_dir_off[(t, rid)]
            buf[o:o + 16] = dir_header(1)
            struct.pack_into('<II', buf, o + 16, LANG, data_entry_off[(t, rid)])
            d = data_entry_off[(t, rid)]
            struct.pack_into('<IIII', buf, d, raw_off[(t, rid)], len(blob), 0, 0)
            relocs.append(d)  # OffsetToData must become an RVA
            r = raw_off[(t, rid)]
            buf[r:r + len(blob)] = blob

    # COFF object: header, one .rsrc section, relocations, one section symbol, empty string table
    hdr_size, sec_size = 20, 40
    raw_ptr = hdr_size + sec_size
    reloc_ptr = raw_ptr + len(buf)
    sym_ptr = reloc_ptr + 10 * len(relocs)
    out = bytearray()
    out += struct.pack('<HHIIIHH', 0x8664, 1, 0, sym_ptr, 1, 0, 0)
    out += struct.pack('<8sIIIIIIHHI', b'.rsrc', 0, 0, len(buf), raw_ptr, reloc_ptr, 0,
                       len(relocs), 0, 0x40000040)
    out += buf
    for r in relocs:
        out += struct.pack('<IIH', r, 0, 3)  # IMAGE_REL_AMD64_ADDR32NB against symbol 0
    out += struct.pack('<8sIhHBB', b'.rsrc', 0, 1, 0, 3, 0)
    out += struct.pack('<I', 4)
    return bytes(out)


MANIFEST = b'''<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">
  <trustInfo xmlns="urn:schemas-microsoft-com:asm.v3">
    <security><requestedPrivileges><requestedExecutionLevel level="asInvoker" uiAccess="false"/></requestedPrivileges></security>
  </trustInfo>
  <compatibility xmlns="urn:schemas-microsoft-com:compatibility.v1">
    <application><supportedOS Id="{8e0f7a12-bfb3-4fe8-b9a5-48fd50a15a9a}"/></application>
  </compatibility>
  <application xmlns="urn:schemas-microsoft-com:asm.v3">
    <windowsSettings><dpiAware xmlns="http://schemas.microsoft.com/SMI/2005/WindowsSettings">true</dpiAware></windowsSettings>
  </application>
</assembly>
'''

if __name__ == '__main__':
    manifest = open(sys.argv[3], 'rb').read() if len(sys.argv) > 3 else MANIFEST
    open(sys.argv[2], 'wb').write(build(sys.argv[1], manifest))
    print('wrote', sys.argv[2])
