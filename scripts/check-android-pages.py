#!/usr/bin/env python3
"""Check native ELF LOAD/RELRO and uncompressed APK entry alignment (16 KB)."""
import struct
import sys
import zipfile

PAGE = 16 * 1024


def check(apk):
    checked = 0
    problems = []
    with zipfile.ZipFile(apk) as archive, open(apk, "rb") as source:
        for entry in archive.infolist():
            if not entry.filename.startswith("lib/") or not entry.filename.endswith(".so"):
                continue
            data = archive.read(entry)
            if data[:4] != b"\x7fELF" or data[4] != 2:
                problems.append(f"{entry.filename}: expected a 64-bit ELF library")
                continue
            endian = "<" if data[5] == 1 else ">"
            offset = struct.unpack_from(endian + "Q", data, 32)[0]
            size, count = struct.unpack_from(endian + "HH", data, 54)
            headers = [struct.unpack_from(endian + "IIQQQQQQ", data, offset + i * size) for i in range(count)]
            loads = [header for header in headers if header[0] == 1]
            if not loads:
                problems.append(f"{entry.filename}: no ELF LOAD segments")
            for header in loads:
                _, _, file_offset, address, _, _, _, alignment = header
                if alignment < PAGE or alignment % PAGE or (address - file_offset) % PAGE:
                    problems.append(f"{entry.filename}: LOAD alignment {alignment}, offset {file_offset:#x}, address {address:#x}")
            for relro in [header for header in headers if header[0] == 0x6474E552]:
                end = relro[3] + relro[6]
                if end % PAGE and any(load[1] & 2 and load[3] <= relro[3] and load[3] + load[6] > end for load in loads):
                    problems.append(f"{entry.filename}: RELRO ends inside a writable 16 KB page")
            if entry.compress_type == zipfile.ZIP_STORED:
                source.seek(entry.header_offset)
                header = source.read(30)
                name_size, extra_size = struct.unpack_from("<HH", header, 26)
                payload = entry.header_offset + 30 + name_size + extra_size
                if payload % PAGE:
                    problems.append(f"{entry.filename}: APK data offset {payload} is not 16 KB aligned")
            checked += 1
            print(f"{entry.filename}: LOAD alignments {[h[7] for h in loads]}")
    if not checked:
        problems.append("No native libraries found; expected the gomobile backend")
    if problems:
        raise SystemExit("16 KB verification FAILED:\n" + "\n".join(problems))
    print(f"16 KB verification passed: {checked} native libraries; ELF LOAD/RELRO and APK entry alignment")


if __name__ == "__main__":
    if len(sys.argv) != 2:
        raise SystemExit("Usage: python3 scripts/check-android-pages.py path/to/app.apk")
    check(sys.argv[1])
