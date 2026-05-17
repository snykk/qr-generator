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
- [x] Generator polynomial Reed–Solomon per ukuran EC codeword. — `qrgen/reedsolomon.go` `genPoly` (divalidasi terhadap tabel α-exponent untuk semua 13 ukuran block EC).

### M3 — Data Encoding `(M)`

Tujuan: input string → bit stream final (sebelum RS).

- [x] **Mode analyzer**: deteksi numeric / alphanumeric / byte (algoritma greedy cukup untuk MVP; nanti bisa diganti optimal segmentation). — `qrgen/encode.go` `analyzeMode`
- [x] Encoder per mode → bit stream. — `qrgen/encode.go` `writeNumeric` / `writeAlphanumeric` / `writeByte`
- [x] Pemilihan **versi minimum** berdasarkan panjang data + EC level. — `qrgen/encode.go` `selectVersion`
- [x] Mode indicator + character count indicator + payload + **terminator + pad bytes**. — `qrgen/encode.go` `encodeText` (divalidasi end-to-end dengan worked example "HELLO WORLD").

### M4 — Reed–Solomon Error Correction `(M)`

Tujuan: data codewords → data + EC codewords yang sudah ter-interleave.

- [x] **GF(256)** arithmetic: tabel exp/log dengan primitive polynomial `0x11D`. — `qrgen/gf256.go` `expTable` / `logTable` / `gf256Mul`
- [x] Polynomial multiply/divide di GF(256). — `qrgen/gf256.go` `polyMul` / `polyMod` (divisor monic; generator QR selalu monic)
- [x] Generate **generator polynomial** untuk n EC codewords. — `qrgen/reedsolomon.go` `genPoly`
- [x] Reed–Solomon encoder per block. — `qrgen/reedsolomon.go` `encodeBlock` (divalidasi terhadap 10 EC codeword dari worked example "HELLO WORLD")
- [x] Pembagian data ke blocks sesuai tabel M2, lalu **interleave** data & EC. — `qrgen/reedsolomon.go` `splitAndEncodeBlocks` + `interleaveBlocks` + `rsEncode`

### M5 — Matrix Construction `(M)`

Tujuan: kerangka matrix QR terisi (fungsional + data) — sebelum masking.

- [x] Ukuran matrix = `21 + 4×(version-1)`. — `qrgen/matrix.go` `newMatrix`
- [x] Penempatan **finder patterns** (3 sudut) + separators. — `qrgen/matrix.go` `placeFinderPatterns` / `placeSingleFinder`
- [x] **Alignment patterns** sesuai tabel M2. — `qrgen/matrix.go` `placeAlignmentPatterns` / `placeSingleAlignment`
- [x] **Timing patterns**. — `qrgen/matrix.go` `placeTimingPatterns`
- [x] **Dark module** (`(4*version+9, 8)`). — `qrgen/matrix.go` `placeDarkModule` (catatan plan sebelumnya menulis koordinatnya terbalik; konvensi di tempat lain adalah `(row, col)`).
- [x] Area **format info & version info** di-reserve dulu. — `qrgen/matrix.go` `reserveFormatInfoArea` + `reserveVersionInfoArea`
- [x] Penempatan **data bits** secara zig-zag dari kanan-bawah, melompati area fungsional. — `qrgen/matrix.go` `placeData` (divalidasi terhadap invariant `TestDataAreaCellsMatchesCapacity` untuk semua 40 versi).

### M6 — Masking & Format/Version Info `(M)`

Tujuan: pilih mask terbaik, tulis format & version info final.

- [x] Implementasi **8 mask patterns** (mask 0..7). — `qrgen/mask.go` `maskCondition`
- [x] Apply mask hanya ke modul data (bukan area fungsional). — `qrgen/mask.go` `applyMask` (melewati cell reserved)
- [x] **Penalty evaluation** (4 aturan dari spec). — `qrgen/mask.go` `penalty` / `penaltyRule1..4`
- [x] Pilih mask dengan total penalty terendah. — `qrgen/mask.go` `selectAndApplyMask` (clone matrix per trial; tie diputuskan dengan index terkecil)
- [x] Encode **format info** (EC level + mask) dengan BCH(15,5) + XOR mask `0x5412`. — `qrgen/matrix.go` `writeFormatInfo` (tabel BCH-nya di-precompute di `qrgen/formatinfo.go` dari M2)
- [x] Encode **version info** untuk version ≥ 7 dengan BCH(18,6). — `qrgen/matrix.go` `writeVersionInfo`

### M7 — Renderer PNG `(S)`

Tujuan: matrix `[][]bool` → byte PNG.

- [x] Konversi matrix → `image.Gray` (atau RGBA bila warna kustom). — `qrgen/render_png.go` `renderGray` / `renderRGBA` (path default monokrom menjaga PNG tetap kecil).
- [x] **Quiet zone** (default 4 modul) di sekeliling. — `qrgen/render_png.go` `renderOptions.quietZone` (default 4).
- [x] Konfigurasi **module size** dalam piksel. — `qrgen/render_png.go` `renderOptions.moduleSize` (default 8).
- [x] Konfigurasi **foreground & background color**. — `qrgen/render_png.go` `renderOptions.foreground` / `background`.
- [x] Encode lewat `image/png`. — `qrgen/render_png.go` `renderPNG` (divalidasi via round-trip decode dengan sampling pixel-centre).

### M8 — Public API & Examples `(S)`

Tujuan: permukaan library yang nyaman dipakai.

- [x] Functional options: `WithECLevel`, `WithVersion`, `WithMask`, `WithModuleSize`, `WithQuietZone`, `WithColors`, dst. — `qrgen/options.go`
- [x] Entry points: — `qrgen/api.go`
  - `qrgen.Encode(text string, opts ...Option) ([]byte, error)` → PNG bytes.
  - `qrgen.EncodeToFile(text, path string, opts ...Option) error`.
  - `qrgen.Matrix(text string, opts ...Option) ([][]bool, error)` (raw access).
- [x] Godoc lengkap untuk semua simbol publik. — semua simbol exported di `qrgen/*.go` punya doc comment yang diawali nama simbolnya.
- [x] `examples/basic/main.go` & `examples/styled/main.go`. — bisa dijalankan via `go run ./examples/basic` dan `go run ./examples/styled`.

### M9 — CLI Tipis `(S)`

Tujuan: binary `qrgen` untuk pemakaian cepat & demo.

- [x] `cmd/qrgen/main.go` dengan flags: — sudah mencakup `-text`, `-out` (pakai `-` untuk stdout), `-size`, `-ec`, `-fg`, `-bg`, `-quiet-zone`, plus tambahan `-version` dan `-mask`.
- [x] Exit code & pesan error yang jelas. — `run()` mengembalikan error yang dicetak `main` dengan prefix `qrgen:` lalu exit 1.
- [x] Contoh penggunaan di README. — section CLI ditambahkan dengan install, contoh basic, stdin-pipe, dan styled.

### M10 — Quality Gate `(M)`

Tujuan: keyakinan bahwa output benar & stabil.

- [x] Unit test per komponen (gf256, RS, encoder per mode, matrix, mask). — tersebar di `qrgen/*_test.go`; lebih dari 80 kasus, termasuk tabel alpha-exponent, sweep komutativitas GF(256), jumlah alignment per versi, involutivitas mask, dst.
- [x] **Golden tests** untuk beberapa pasangan (input, EC level) → bandingkan matrix dengan referensi yang diketahui benar (mis. dari Nayuki). — `TestEncodeTextHelloWorld`, `TestEncodeBlockHelloWorld`, `TestRSEncodeHelloWorld`, plus tabel generator-polynomial dari Nayuki di `TestGenPolyAlphaExponents`.
- [x] Round-trip test pakai decoder pihak ketiga di test-only dependency (boleh, karena ini bukan dependency runtime). — `qrgen/roundtrip_test.go` menjalankan 12 kasus (text, EC, versi) melalui `github.com/makiuchi-d/gozxing`.
- [x] Benchmarks untuk path encode utama. — `qrgen/bench_test.go` mencakup payload small/URL/multi-block/large plus varian matrix-only.
- [x] `go test -race ./...` bersih. — tiap run CI menjalankan dengan `-race`; tidak terdeteksi data race.

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
