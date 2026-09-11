#!/bin/bash
# Copyright (c) 2026 LucX-UI Project.
# Licensed under the PolyForm Noncommercial License 1.0.0.
# SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
set -euo pipefail
# Official MTProxy is x86 (pclmul/sse4.2/cpuid). Portable table CRC + OpenSSL AES stay.

sed -i 's/-mpclmul//;s/-march=core2//;s/-mfpmath=sse//;s/-mssse3//' Makefile

cat > common/cpuid.c <<'EOF'
#include "cpuid.h"

kdb_cpuid_t *kdb_cpuid (void) {
  static kdb_cpuid_t cached = { .magic = 0x280147b8, .ebx = 0, .ecx = 0, .edx = 0 };
  return &cached;
}
EOF

python3 - <<'PY'
from pathlib import Path

def wrap_x86(path: Path, start: str, end: str) -> None:
    text = path.read_text()
    i = text.find(start)
    j = text.find(end)
    if i < 0 or j < 0 or j <= i:
        raise SystemExit(f"markers missing in {path}: {start!r} .. {end!r}")
    path.write_text(text[:i] + "#if defined(__x86_64__) || defined(__i386__)\n" + text[i:j] + "#endif\n" + text[j:])

wrap_x86(Path("common/crc32c.c"), "static unsigned crc32c_partial_sse42", "unsigned crc32c_partial_four_tables")
wrap_x86(Path("common/crc32c.c"), "static unsigned crc32c_combine_clmul", "static void crc32c_init")

c = Path("common/crc32c.c").read_text()
old = """void crc32c_init (void) {
  kdb_cpuid_t *p = kdb_cpuid ();
  compute_crc32c_combine = &crc32c_combine_generic;
  if (p->ecx & (1 << 20)) {
    crc32c_partial = crc32c_partial_sse42;
#ifdef __LP64__
    if (p->ecx & 2) {
      crc32c_partial = crc32c_partial_sse42_clmul;
      compute_crc32c_combine = &crc32c_combine_clmul;
    }
#endif
  } else {
    crc32c_partial = &crc32c_partial_four_tables;
  }
}"""
new = """void crc32c_init (void) {
#if defined(__x86_64__) || defined(__i386__)
  kdb_cpuid_t *p = kdb_cpuid ();
  compute_crc32c_combine = &crc32c_combine_generic;
  if (p->ecx & (1 << 20)) {
    crc32c_partial = crc32c_partial_sse42;
#ifdef __LP64__
    if (p->ecx & 2) {
      crc32c_partial = crc32c_partial_sse42_clmul;
      compute_crc32c_combine = &crc32c_combine_clmul;
    }
#endif
  } else {
    crc32c_partial = &crc32c_partial_four_tables;
  }
#else
  compute_crc32c_combine = &crc32c_combine_generic;
  crc32c_partial = &crc32c_partial_four_tables;
#endif
}"""
if old not in c:
    raise SystemExit("crc32c_init not found")
Path("common/crc32c.c").write_text(c.replace(old, new, 1))

wrap_x86(Path("common/crc32.c"), "/******************** CLMUL ********************/", "static void crc32_init")

c = Path("common/crc32.c").read_text()
old = """void crc32_init (void) {
  kdb_cpuid_t *p = kdb_cpuid ();
  if (p->ecx & 2) {
    crc32_partial = crc32_partial_clmul;
    crc64_partial = crc64_partial_clmul;
    compute_crc32_combine = compute_crc32_combine_clmul;
    compute_crc64_combine = compute_crc64_combine_clmul;
  } else {
    crc32_partial = crc32_partial_generic;
    crc64_partial = crc64_partial_one_table;
    compute_crc32_combine = compute_crc32_combine_generic;
    compute_crc64_combine = compute_crc64_combine_generic;
  }
}"""
new = """void crc32_init (void) {
#if defined(__x86_64__) || defined(__i386__)
  kdb_cpuid_t *p = kdb_cpuid ();
  if (p->ecx & 2) {
    crc32_partial = crc32_partial_clmul;
    crc64_partial = crc64_partial_clmul;
    compute_crc32_combine = compute_crc32_combine_clmul;
    compute_crc64_combine = compute_crc64_combine_clmul;
  } else
#endif
  {
    crc32_partial = crc32_partial_generic;
    crc64_partial = crc64_partial_one_table;
    compute_crc32_combine = compute_crc32_combine_generic;
    compute_crc64_combine = compute_crc64_combine_generic;
  }
}"""
if old not in c:
    raise SystemExit("crc32_init not found")
Path("common/crc32.c").write_text(c.replace(old, new, 1))
print("mtproxy arm64 patch ok")
PY
