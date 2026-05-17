# QR Generator — Plan

Dokumen ini menjelaskan rencana implementasi `qr-generator`: sebuah library Go untuk membuat QR code, ditulis dari nol mengikuti spesifikasi **ISO/IEC 18004**, tanpa dependensi eksternal (hanya Go standard library).

> Status: **draft / living document.** Plan ini akan terus diperbarui seiring berjalannya implementasi. Setiap milestone dianggap selesai bila semua item di dalamnya sudah teruji dan terdokumentasi.

> Versi English: [docs/plan.md](plan.md).

---

## 1. Visi & Tujuan

- Menyediakan **library Go pure** (`import "github.com/.../qrgen"`) yang dapat dipakai project lain untuk generate QR code menjadi gambar PNG.
- Implementasi **dari nol** — bukan wrapper. Tujuan utamanya adalah pembelajaran bagaimana QR bekerja (mode encoding, Reed–Solomon, masking, matrix layout), sambil menghasilkan artefak yang nyata dan reusable.
- Menyertakan **CLI tipis** di `cmd/qrgen` sebagai contoh konsumen sekaligus alat demo cepat.

## 2. Prinsip Desain

1. **Zero external deps** — hanya standard library Go. Tidak ada image library pihak ketiga, tidak ada dependency untuk Reed–Solomon, dsb.
2. **Spec-first** — setiap algoritma dirujuk ke bagian spesifikasi ISO/IEC 18004 (atau referensi terbuka yang setara, mis. Thonky tutorial, Nayuki).
3. **API ergonomis** — default masuk akal (auto-version, EC level M, quiet zone 4 modul), advanced lewat *functional options*.
4. **Testable** — setiap layer (encoder, RS, matrix, mask, render) punya unit test sendiri; output PNG diverifikasi via *golden test*.
5. **Stabilitas API** ditentukan via semver. Pra-`v1.0.0` masih boleh breaking, tapi selalu dicatat di `CHANGELOG.md`.

## 3. Scope Versi Awal

### In scope (target ≤ v0.1.0)

- Mode encoding: **numeric, alphanumeric, byte (UTF-8)**.
- Versi QR: **1–40** (semua versi standar).
- Error correction level: **L, M, Q, H**.
- Output: **PNG** (via `image/png` stdlib).
- CLI sederhana untuk encode dari string/stdin ke file PNG.

### Out of scope (untuk sekarang)

- Mode **Kanji** dan **ECI** (encoding non-default).
- **Micro QR**, **rMQR**, **structured append** (multi-symbol).
- Output **SVG / terminal / JPEG** (kandidat v0.2+).
- **Logo embedding** di tengah QR (kandidat v0.2+).
- **Decoder** / reader QR (di luar scope library ini).
- HTTP service / web playground.

---

## 4. Milestones

Tiap milestone ditulis berurutan karena ada dependensi natural antara komponen. Sizing (`S` / `M`) adalah ukuran usaha relatif, bukan komitmen waktu.

### M1 — Foundation `(S)`

Tujuan: kerangka project siap dipakai untuk milestone berikutnya.

- [ ] `go mod init` dengan module path yang benar.
- [ ] Struktur folder awal (lihat bagian 5).
- [x] `LICENSE` — Apache 2.0 (sudah ditambahkan via license generator GitHub).
- [ ] `.gitignore` Go-standard.
- [ ] `README.md` skeleton (overview + status "in development").
- [ ] CI workflow minimum: `go vet`, `go test ./...`, `go build ./...`.

### M2 — Tabel & Konstanta Spec `(M)`

Tujuan: data statis dari spec siap dipakai oleh layer encoder.

- [x] Tabel **character count indicator** (jumlah bit untuk panjang data per mode × range version). — `qrgen/mode.go`
- [x] Tabel **kapasitas data** per (version × EC level). — `qrgen/version.go`
- [x] Tabel **jumlah & ukuran block error correction** per (version × EC level). — `qrgen/version.go`
- [x] Tabel **posisi alignment pattern** per version. — `qrgen/version.go`
- [x] Tabel **format info** (BCH-encoded) dan **version info** (untuk version ≥ 7). — `qrgen/formatinfo.go`
- [ ] Generator polynomial Reed–Solomon per ukuran EC codeword. — ditunda ke M4 (dihitung via GF(256) setelah arithmetic-nya tersedia).

### M3 — Data Encoding `(M)`

Tujuan: input string → bit stream final (sebelum RS).

- [x] **Mode analyzer**: deteksi numeric / alphanumeric / byte (algoritma greedy cukup untuk MVP; nanti bisa diganti optimal segmentation). — `qrgen/encode.go` `analyzeMode`
- [x] Encoder per mode → bit stream. — `qrgen/encode.go` `writeNumeric` / `writeAlphanumeric` / `writeByte`
- [x] Pemilihan **versi minimum** berdasarkan panjang data + EC level. — `qrgen/encode.go` `selectVersion`
- [x] Mode indicator + character count indicator + payload + **terminator + pad bytes**. — `qrgen/encode.go` `encodeText` (divalidasi end-to-end dengan worked example "HELLO WORLD").

### M4 — Reed–Solomon Error Correction `(M)`

Tujuan: data codewords → data + EC codewords yang sudah ter-interleave.

- [ ] **GF(256)** arithmetic: tabel exp/log dengan primitive polynomial `0x11D`.
- [ ] Polynomial multiply/divide di GF(256).
- [ ] Generate **generator polynomial** untuk n EC codewords.
- [ ] Reed–Solomon encoder per block.
- [ ] Pembagian data ke blocks sesuai tabel M2, lalu **interleave** data & EC.

### M5 — Matrix Construction `(M)`

Tujuan: kerangka matrix QR terisi (fungsional + data) — sebelum masking.

- [ ] Ukuran matrix = `21 + 4×(version-1)`.
- [ ] Penempatan **finder patterns** (3 sudut) + separators.
- [ ] **Alignment patterns** sesuai tabel M2.
- [ ] **Timing patterns**.
- [ ] **Dark module** (`(8, 4*version+9)`).
- [ ] Area **format info & version info** di-reserve dulu.
- [ ] Penempatan **data bits** secara zig-zag dari kanan-bawah, melompati area fungsional.

### M6 — Masking & Format/Version Info `(M)`

Tujuan: pilih mask terbaik, tulis format & version info final.

- [ ] Implementasi **8 mask patterns** (mask 0..7).
- [ ] Apply mask hanya ke modul data (bukan area fungsional).
- [ ] **Penalty evaluation** (4 aturan dari spec).
- [ ] Pilih mask dengan total penalty terendah.
- [ ] Encode **format info** (EC level + mask) dengan BCH(15,5) + XOR mask `0x5412`.
- [ ] Encode **version info** untuk version ≥ 7 dengan BCH(18,6).

### M7 — Renderer PNG `(S)`

Tujuan: matrix `[][]bool` → byte PNG.

- [ ] Konversi matrix → `image.Gray` (atau RGBA bila warna kustom).
- [ ] **Quiet zone** (default 4 modul) di sekeliling.
- [ ] Konfigurasi **module size** dalam piksel.
- [ ] Konfigurasi **foreground & background color**.
- [ ] Encode lewat `image/png`.

### M8 — Public API & Examples `(S)`

Tujuan: permukaan library yang nyaman dipakai.

- [ ] Functional options: `WithECLevel`, `WithVersion`, `WithMask`, `WithModuleSize`, `WithQuietZone`, `WithColors`, dst.
- [ ] Entry points:
  - `qrgen.Encode(text string, opts ...Option) ([]byte, error)` → PNG bytes.
  - `qrgen.EncodeToFile(text, path string, opts ...Option) error`.
  - `qrgen.Matrix(text string, opts ...Option) ([][]bool, error)` (raw access).
- [ ] Godoc lengkap untuk semua simbol publik.
- [ ] `examples/basic/main.go` & `examples/styled/main.go`.

### M9 — CLI Tipis `(S)`

Tujuan: binary `qrgen` untuk pemakaian cepat & demo.

- [ ] `cmd/qrgen/main.go` dengan flags:
  - `-text` (opsional; fallback ke stdin).
  - `-out` (path file output; default `qr.png`).
  - `-size` (module size px).
  - `-ec` (`L|M|Q|H`).
  - `-fg`, `-bg` (hex color).
  - `-quiet-zone` (jumlah modul).
- [ ] Exit code & pesan error yang jelas.
- [ ] Contoh penggunaan di README.

### M10 — Quality Gate `(M)`

Tujuan: keyakinan bahwa output benar & stabil.

- [ ] Unit test per komponen (gf256, RS, encoder per mode, matrix, mask).
- [ ] **Golden tests** untuk beberapa pasangan (input, EC level) → bandingkan matrix dengan referensi yang diketahui benar (mis. dari Nayuki).
- [ ] Round-trip test pakai decoder pihak ketiga di test-only dependency (boleh, karena ini bukan dependency runtime).
- [ ] Benchmarks untuk path encode utama.
- [ ] `go test -race ./...` bersih.

### M11 — Polish & Release `(S)`

Tujuan: siap dipakai orang lain.

- [ ] `README.md` final: badge, contoh kode, ringkasan API, kompatibilitas Go.
- [ ] `CHANGELOG.md` (Keep a Changelog).
- [ ] Tag `v0.1.0`.
- [ ] Verifikasi `go install` dan `go get` jalan dari module path.

---

## 5. Struktur Folder yang Diusulkan

```
qr-generator/
├── README.md
├── LICENSE
├── CHANGELOG.md
├── go.mod
├── go.sum
├── docs/
│   ├── plan.md             # versi English
│   ├── plan.id.md          # dokumen ini
│   └── theory/             # tinjauan pustaka bilingual (algoritma + tabel data + contoh end-to-end)
├── qrgen/                  # package utama (importable)
│   ├── qrgen.go            # entry points + doc package
│   ├── options.go          # functional options
│   ├── mode.go             # mode analyzer + per-mode encoder
│   ├── version.go          # tabel kapasitas, pemilihan version
│   ├── gf256.go            # arithmetic GF(256)
│   ├── reedsolomon.go      # generator poly + encoder
│   ├── matrix.go           # placement finder/alignment/timing/data
│   ├── mask.go             # 8 mask + penalty + pemilihan mask terbaik
│   ├── formatinfo.go       # BCH untuk format & version info
│   ├── render_png.go       # renderer ke PNG
│   └── *_test.go
├── examples/
│   ├── basic/main.go
│   └── styled/main.go
└── cmd/
    └── qrgen/
        └── main.go
```

## 6. Risiko & Catatan Teknis

- **Reed–Solomon di GF(256)** adalah bagian paling rawan bug. Strategi mitigasi: unit test pakai test vector dari spec/Nayuki sebelum dipakai layer di atasnya.
- **Mask penalty rules** rentan salah interpretasi (aturan 1 & 3 sering keliru). Mitigasi: verifikasi dengan tabel mask score dari referensi terbuka.
- **Mode selection optimal** itu non-trivial (algoritma DP untuk segmentasi). Untuk MVP cukup greedy single-mode; segmentasi ditunda ke pasca v0.1.
- **Encoding karakter byte** mengasumsikan UTF-8 langsung (tanpa ECI). Decoder modern umumnya menebak UTF-8, tapi ini bukan compliant ECI behavior — dicatat di README sebagai limitation.

## 7. Referensi

- ISO/IEC 18004:2015 — *Information technology — Automatic identification and data capture techniques — QR code bar code symbology specification.*
- Thonky QR Code Tutorial — https://www.thonky.com/qr-code-tutorial/
- Project Nayuki — *QR Code generator library* (reference implementation).

## 8. Pertanyaan Terbuka

Akan diisi seiring jalannya implementasi. Contoh awal:

- Module path final untuk `go.mod` (apakah pakai path GitHub `github.com/<user>/qr-generator`?).
- Apakah perlu compatibility matrix versi Go (minimum 1.21? 1.22?).
- Apakah `qrgen.Matrix(...)` cukup, atau sediakan juga renderer pluggable?
