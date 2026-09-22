#!/usr/bin/env node
/**
 * Penjaga anti-hardcode teks pengguna (W8).
 *
 * Menolak literal teks pengguna yang baru di berkas halaman & komponen:
 *  1. Atribut JSX dengan nilai literal yang terlihat pengguna (title, label, dsb).
 *  2. Teks JSX (isi di antara tag) yang berbentuk frasa.
 *
 * Nilai yang memang bukan bahasa (className, kunci enum, kode, nama berkas, URL,
 * kunci localStorage) tidak dianggap pelanggaran. Bila sebuah berkas punya
 * pengecualian yang sah, daftarkan di EXCEPTIONS beserta alasannya.
 *
 * Yang sengaja TIDAK dipindai:
 *  - src/lib/**: pesan galat lapisan jaringan, bukan komponen React; sumber bahasa
 *    utama kini dari katalog i18n API.
 *  - Simbol mata uang "Rp" dan identitas bawaan aplikasi (branding), bukan bahasa.
 *
 * Jalankan: node scripts/check-i18n.mjs  (keluar dengan kode 1 bila ada temuan)
 *           node scripts/check-i18n.mjs --count  (laporan angka, tanpa gagal)
 */
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { fileURLToPath } from "node:url";

const ROOT = join(fileURLToPath(new URL(".", import.meta.url)), "..");
const SRC = join(ROOT, "src");
const SCAN_DIRS = [join(SRC, "app"), join(SRC, "components")];
const EXCLUDE_DIRS = [join(SRC, "i18n")];

// Atribut JSX yang nilainya dilihat pengguna. Nilai berupa {ekspresi} diabaikan.
const TEXT_ATTRS = new Set([
  "title",
  "description",
  "label",
  "placeholder",
  "helperText",
  "emptyMessage",
  "confirmLabel",
  "cancelLabel",
  "reason",
  "alt",
  "aria-label",
  "header",
]);

// Pengecualian eksplisit: pola literal yang sah (bukan bahasa yang perlu kamus).
const LITERAL_ALLOW = [
  "IDR - Rupiah", // nama mata uang, sama di kedua bahasa
  "CIF", // singkatan data induk nasabah, bukan bahasa
];

// Pengecualian per berkas beserta alasannya.
const FILE_ALLOW = {
  // (kosong) Semua halaman/komponen saat ini sudah memakai kamus.
};

function walk(dir, out = []) {
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry);
    if (STATS_IS_DIR(full)) {
      if (EXCLUDE_DIRS.some((ex) => full === ex || full.startsWith(ex + "/"))) {
        continue;
      }
      walk(full, out);
    } else if (entry.endsWith(".tsx")) {
      out.push(full);
    }
  }
  return out;
}

function STATS_IS_DIR(path) {
  return statSync(path).isDirectory();
}

/** Buang komentar agar tidak salah menandai teks di dalamnya. */
function stripComments(text) {
  return text
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/(^|[^:])\/\/[^\n]*/g, "$1");
}

const hasLetters = (s) => /[A-Za-z]/.test(s);
const isAllCapsOrCode = (s) => /^[A-Z0-9 _.-]+$/.test(s);
const looksLikeCode = (s) =>
  /[;{}()=>]|^(export|import|interface|type|const|let|var|function|return|class)\b|className|from "/.test(
    s,
  );
const wordCount = (s) => (s.match(/[A-Za-z]{2,}/g) || []).length;
const isFrase = (s) => wordCount(s) >= 2;
const isSingleWordLabel = (s) => /^[A-Z][a-z]+$/.test(s.trim());

function findViolations(filePath) {
  const raw = readFileSync(filePath, "utf8");
  const text = stripComments(raw);
  const findings = [];

  // 1. Atribut JSX bernilai literal.
  for (const m of text.matchAll(/([A-Za-z][\w-]*)="([^"]*)"/g)) {
    const [, attr, value] = m;
    if (!TEXT_ATTRS.has(attr)) continue;
    if (!hasLetters(value)) continue;
    if (LITERAL_ALLOW.includes(value)) continue;
    findings.push({ kind: `atribut ${attr}`, value });
  }

  // 1b. Nilai literal pada object/JS untuk kunci yang terlihat pengguna,
  // mis. { header: "Nomor Rekening" } pada kolom tabel.
  for (const m of text.matchAll(/([A-Za-z][\w-]*):\s*"([^"]*)"/g)) {
    const [, key, value] = m;
    if (!TEXT_ATTRS.has(key)) continue;
    if (!hasLetters(value)) continue;
    if (LITERAL_ALLOW.includes(value)) continue;
    findings.push({ kind: `kunci ${key}`, value });
  }

  // 2. Teks JSX (isi di antara tag) yang tidak memuat ekspresi {..}. Bagian teks
  // yang diapit beberapa ekspresi bisa lolos; ini pemeriksa sederhana, bukan parser.
  for (const m of text.matchAll(/>([^<>{}]+)</gs)) {
    const rawText = m[1].replace(/\s+/g, " ").trim();
    if (!rawText || rawText.length > 200) continue;
    if (!hasLetters(rawText)) continue;
    if (isAllCapsOrCode(rawText)) continue;
    if (looksLikeCode(rawText)) continue;
    if (!(isFrase(rawText) || isSingleWordLabel(rawText))) continue;
    if (LITERAL_ALLOW.includes(rawText)) continue;
    findings.push({ kind: "teks JSX", value: rawText });
  }

  return findings;
}

function main() {
  const files = SCAN_DIRS.flatMap((dir) => walk(dir));
  const countOnly = process.argv.includes("--count");
  let failures = 0;
  const perFile = [];

  for (const file of files) {
    const rel = relative(ROOT, file);
    if (FILE_ALLOW[rel]) continue;
    const findings = findViolations(file);
    if (findings.length === 0) continue;
    failures += findings.length;
    perFile.push([rel, findings.length]);
    if (!countOnly) {
      console.error(`\n${rel}`);
      for (const f of findings) {
        console.error(`  ${f.kind}: "${f.value}"`);
      }
    }
  }

  if (countOnly) {
    perFile
      .sort((a, b) => b[1] - a[1])
      .forEach(([rel, n]) => console.log(`${n}\t${rel}`));
    console.log(`TOTAL\t${failures}`);
    return;
  }

  if (failures > 0) {
    console.error(
      `\nGagal: ${failures} literal teks pengguna. Pindahkan ke kamus id.ts/en.ts ` +
        `atau daftarkan pengecualian yang beralasan di scripts/check-i18n.mjs.`,
    );
    process.exit(1);
  }
  console.log(
    `OK: ${files.length} berkas halaman/komponen bebas literal teks pengguna.`,
  );
}

main();
