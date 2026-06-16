# Tinjauan Pustaka & Referensi

Folder ini berisi tinjauan pustaka dan landasan teori untuk library `qrgen`. Setiap dokumen membahas satu komponen encoder QR code dan ditujukan sebagai sumber belajar untuk kontributor sekaligus catatan bahwa implementasi bersandar pada algoritma yang dipublikasikan — bukan asumsi.

> Versi English: [docs/theory/README.md](README.md).

## Daftar isi

1. [Ikhtisar QR Code](01-qr-overview.id.md) — sejarah, anatomi simbol, sistem versi, level error correction, alur encode.
2. [Encoding Data](02-data-encoding.id.md) — mode encoding, character count indicator, pemilihan versi, terminator dan padding.
3. [Galois Field GF(256)](03-galois-field.id.md) — aritmatika field yang menjadi dasar Reed–Solomon.
4. [Error Correction Reed–Solomon](04-reed-solomon.id.md) — generator polynomial, encoding per-block, interleaving, remainder bits.
5. [Konstruksi Matrix](05-matrix-construction.id.md) — finder, alignment, timing pattern, area reserved, zig-zag data walk.
6. [Masking](06-masking.id.md) — 8 mask pattern dan penalty score empat-aturan.
7. [Format & Version Information](07-format-version-info.id.md) — kode BCH(15, 5) dan BCH(18, 6) beserta penempatan bit-nya.
8. [Rendering](08-rendering.id.md) — matrix → PNG, ukuran modul, quiet zone, pemilihan warna.
9. [Tabel Data & Nilai Lookup](09-data-tables.id.md) — semua tabel statis yang dibutuhkan encoder: mode indicator, pemetaan alphanumeric, kapasitas, struktur block EC, posisi alignment, generator polynomial, codeword format/version.
10. [Contoh End-to-End: `HELLO WORLD`](10-worked-example.id.md) — encode end-to-end di EC-M dengan setiap nilai intermediate, siap dijadikan golden fixture.

### Sisi decoder (v0.2.0)

11. [Decoding Reed–Solomon](11-rs-decoding.id.md) — syndrome, Berlekamp–Massey, Chien search, algoritma Forney. Kebalikan doc 04 tapi secara algoritmik berbeda.
12. [Image Processing](12-image-processing.id.md) — grayscale, binarisasi Otsu, scan finder-pattern, homography, sampling modul.
13. [Pipeline Decoder](13-decoder-pipeline.id.md) — diagram tahap end-to-end, apa yang bisa gagal di tiap tahap, dan filosofi penanganan error.

### Robustness decoder (v0.3.0)

14. [Adaptive Thresholding](14-adaptive-thresholding.id.md) — failure mode Otsu pada pencahayaan tidak rata, formula dan parameter Sauvola, integral image untuk window query O(1), dan dispatch bimodality-proaktif plus post-check-reaktif yang memutuskan binariser mana yang dijalankan.

### Robustness decoder (v0.4.0)

15. [Rotation Handling](15-rotation-handling.id.md) — di mana asumsi tegak tinggal di pipeline v0.3, kenapa deteksi vertex sudut-siku-siku sudah rotation-invariant, identitas handedness cross-product yang mendisambiguasi top-right dari bottom-left di rotasi apa pun, dan boundary cakupan di ~30 derajat yang ditetapkan oleh toleransi scanner 1:1:3:1:1.

### Format output (v0.5.0)

16. [SVG Rendering](16-svg-rendering.id.md) — kenapa SVG (scaling lossless, file kecil, tepi tajam), model dokumen SVG untuk simbol QR, menggambar satu-path versus satu rect per modul, viewBox unit-modul dengan sizing pixel, crispEdges dan decodability, warna-ke-hex dengan fill-opacity untuk alpha, dan kenapa renderer-nya sibling function alih-alih Render interface.

## Referensi utama

- **ISO/IEC 18004:2015** — *Information technology — Automatic identification and data capture techniques — QR code bar code symbology specification.* Sumber normatif.
- **Thonky QR Code Tutorial** — walkthrough praktis untuk proses encoding, berguna ketika spec terlalu padat. <https://www.thonky.com/qr-code-tutorial/>
- **Project Nayuki** — *QR Code generator library*, termasuk reference implementation step-by-step di beberapa bahasa, cocok sebagai oracle pembanding. <https://www.nayuki.io/page/qr-code-generator-library>

Referensi tambahan ada di akhir tiap dokumen.

## Konvensi yang digunakan

- Bit string ditulis kiri-ke-kanan dengan bit paling signifikan di depan.
- Koordinat matrix memakai `(row, column)` dengan `(0, 0)` di **pojok kiri-atas**, sesuai orientasi saat render ke gambar.
- Literal hex memakai prefix `0x`.
- Koefisien polinomial dituliskan dari derajat tertinggi ke terendah kecuali disebut lain.
- Penunjuk "Spec ref." merujuk ke bagian ISO/IEC 18004:2015.

## Hubungan dengan kode

| Dokumen teori                          | File implementasi utama            |
|----------------------------------------|------------------------------------|
| 02-data-encoding.id.md                 | `qrgen/mode.go`, `qrgen/version.go`|
| 03-galois-field.id.md                  | `qrgen/gf256.go`                   |
| 04-reed-solomon.id.md                  | `qrgen/reedsolomon.go`             |
| 05-matrix-construction.id.md           | `qrgen/matrix.go`                  |
| 06-masking.id.md                       | `qrgen/mask.go`                    |
| 07-format-version-info.id.md           | `qrgen/formatinfo.go`              |
| 08-rendering.id.md                     | `qrgen/render_png.go`              |
| 09-data-tables.id.md                   | `qrgen/version.go`, `qrgen/formatinfo.go`, `qrgen/matrix.go` |
| 10-worked-example.id.md                | golden test fixture di `qrgen/testdata/` |
| 11-rs-decoding.id.md                   | `qrgen/rs_decode.go` (direncanakan, D3)       |
| 12-image-processing.id.md              | `qrgen/decode_image.go` (direncanakan, D8–D12)|
| 13-decoder-pipeline.id.md              | `qrgen/decode.go` (direncanakan, D7 + D12)    |
| 14-adaptive-thresholding.id.md         | `qrgen/decode_image_sauvola.go` (direncanakan, T2 + T3) |
| 15-rotation-handling.id.md             | `qrgen/decode_image.go` (direncanakan, R2: `orderFinderTriple`) |
| 16-svg-rendering.id.md                 | `qrgen/render_svg.go` (direncanakan, S3) |

Bila mengubah algoritma, mohon perbarui dokumen terkait di folder ini. Dokumen teori adalah penjelasan tahan lama tentang *mengapa* kode terlihat seperti sekarang.
