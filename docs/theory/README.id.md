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

16. [SVG Rendering](16-svg-rendering.id.md) — kenapa SVG (scaling lossless, tepi tajam, embedding HTML — bukan ukuran file), model dokumen SVG untuk simbol QR, menggambar satu-path versus satu rect per modul, viewBox unit-modul dengan sizing pixel, crispEdges dan decodability, warna-ke-hex dengan fill-opacity untuk alpha, dan kenapa renderer-nya sibling function alih-alih Render interface.

### Kelengkapan encoder (v0.6.0)

17. [Optimal Mode Segmentation](17-optimal-segmentation.id.md) — kenapa greedy single-mode meninggalkan bit terbuang, cost model per-segment, dynamic program atas posisi dan mode, contoh terkerjakan, interplay version-group, aturan batas rune UTF-8, dan jaminan identitas input-homogen.

### Konvensi payload (v0.7.0)

18. [Payload Formats](18-payload-formats.id.md) — konvensi scanner yang dihasilkan convenience builder (Wi-Fi, vCard, mailto, tel, SMS, geo): template persis, aturan escaping, sitasi, dan contoh terkerjakan, plus kenapa string builder compose dengan setiap format output.

### Format output (v0.8.0)

19. [Terminal Rendering](19-terminal-rendering.id.md) — kenapa renderer terminal (kesegeraan, teks pipeable, bisa dipindai dari layar), aspect ratio sel yang melatari half-block packing, pemetaan pasangan-modul ke glyph, fallback ASCII, masalah polaritas dan saklar invert, penanganan quiet zone dan ekor baris ganjil, serta kenapa renderer ini fungsi sibling yang mengembalikan string.

### Kelengkapan encoding (v0.9.0)

20. [ECI Segments](20-eci-segments.id.md) — Extended Channel Interpretation: ECI header (mode `0111` plus designator 1/2/3-codeword), encode bentuk-terpendek versus decode yang menerima non-minimal, nomor assignment dan nuansa default-charset ISO-8859-1-versus-UTF-8, scope ECI, serta batas transcoding zero-dependency (UTF-8 dan Latin-1 saja).

### Structured append (v0.10.0)

21. [Structured Append](21-structured-append.id.md) — memecah pesan ke maksimal 16 simbol tertaut dan menyatukannya: structured-append header (mode `0011` plus sequence indicator posisi/count dan parity byte bersama), cap 16-simbol, parity XOR atas pesan asli, penempatan header sebelum ECI, serta split batas-rune dengan reassembly terurut.

### Kelengkapan encoding (v0.11.0)

22. [Kanji Mode](22-kanji-mode.id.md) — mode data keempat: meng-encode karakter JIS X 0208 sebagai nilai 13-bit di bawah mode indicator `1000` (lebar count 8/10/12), derivasi Shift-JIS, tabel yang di-generate + di-commit yang menjaga runtime zero-dependency, bagaimana Kanji bergabung ke DP segmenter, dan densitas jujur ~1,8x versus byte mode.

### Output styling (v0.12.0)

23. [Logo Embedding](23-logo-embedding.id.md) — menaruh logo di tengah sambil tetap bisa dipindai: kenapa recovery Reed–Solomon dan interleaving block menyerap okusi, ukuran default aman, bump EC-H otomatis, background pad, compositing PNG versus penyisipan data-URI SVG, dan resampler zero-dependency.

### Ketangguhan decoder (v0.13.0)

24. [Decoding pada Sudut Sembarang](24-arbitrary-angle-detection.id.md) — membaca simbol yang miring jauh melewati titik gagal pipeline afin: kenapa penghambat sebenarnya adalah homografi afin tiga-centroid dan bukan deteksi finder, kenapa perspektif butuh transformasi projektif, mengekstrak sudut pola finder lewat analisis connected-component, homografi least-squares dua-belas-sudut, seed version berbasis sudut plus trial decode, fallback afin-dulu yang membiarkan decode tegak tak berubah, dan validasi tilt sintetis dengan pita yang jujur.

### Multi-symbol decoding (v0.14.0)

25. [Multi-Symbol Decoding](25-multi-symbol-decoding.id.md) — membaca setiap simbol QR dalam satu gambar: kenapa detektor connected-component sudah meng-enumerasi semua finder, mengelompokkannya jadi triple valid disjoint dengan pitch modul bersama dan geometri right-isosceles, kenapa keberhasilan decode (bukan geometri) harus jadi penentu klaim agar triple cross-symbol pada grid same-size tak bisa mencuri finder nyata, memakai ulang pipeline projektif per-simbol, menurunkan garis luar tiap simbol dari homography-nya, semantik reading-order dan skip-per-simbol, serta batas yang jujur.

### Micro QR Code (v0.15.0)

26. [Micro QR Code](26-micro-qr.id.md) — saudara kecil QR di standar yang sama: delapan tipe simbol M1 sampai M4 dan kapasitasnya, mode indicator dan lebar count yang lebih pendek, aturan terminator dan padding termasuk half codeword 4 bit di M1 dan M3, satu blok Reed-Solomon (hanya deteksi di M1), tata letak satu finder dengan timing di baris 0 dan kolom 0, satu salinan format information dengan XOR mask sendiri, penempatan tanpa lompatan kolom 6, empat mask dengan aturan skor tepi tertinggi, perbandingan langkah demi langkah dengan QR, dan bit stream contoh yang dicek terhadap zint.

### Deteksi Micro QR (v0.16.0)

27. [Deteksi Micro QR](27-micro-qr-detection.id.md) — menemukan simbol Micro di gambar dari satu finder pattern: apa yang ditentukan empat sudut finder dan ambiguitas seperempat putaran yang tersisa, orientasi dari modul timing terdalam, membaca format information sebelum ukuran diketahui dan kenapa format word yang valid adalah bukti yang lemah, cek silang ukuran terhadap timing pattern, error ekstrapolasi dan mencari ulang sudut-sudut jauh, gerbang false positive yang berujung di Reed-Solomon, di mana pencarian ini masuk ke pipeline setelah QR, dan zxing-cpp sebagai prior art.

### rMQR Code (v0.17.0)

28. [rMQR Code](28-rmqr.id.md) — anggota keluarga QR yang berbentuk persegi panjang (ISO/IEC 23941): 32 versi dari R7x43 sampai R17x139 beserta tabel codeword, blok, remainder, dan lebar count-nya, mode indicator 3 bit, terminator dan padding, error correction pada level M dan H dengan interleaving milik QR, tata letak dengan finder sub-pattern, corner pattern, timing di tepi, dan kolom alignment, dua salinan format 18 bit di bawah BCH(18,6) dengan mask masing-masing, penempatan dari kolom kedua, satu mask tetap, perbandingan dengan QR dan Micro QR, dan bit stream contoh yang dicek terhadap zint.

### Inspeksi simbol dan web demo (v0.18.0)

29. [Inspeksi Simbol](29-symbol-inspection.id.md) — membedah lagi simbol yang sudah jadi dengan `Inspect`: lapisan fungsi dan lapisan pesan, sensus setiap peran modul beserta tumpang-tindih yang menentukannya (alignment di atas timing, area format QR, finder pada R7), pencatatan peran dan bit stream di kode penempatan milik encoder sendiri, pemetaan bit stream ke codeword termasuk half codeword Micro QR, simbol sebelum masking dan skor setiap kandidat mask, anggaran bit pesan, lokasi tiap codeword di bloknya, dan menjalankan library di browser lewat WebAssembly, beserta biaya memindai frame kamera.

### Deteksi rMQR (v0.19.0)

30. [Deteksi rMQR](30-rmqr-detection.id.md) — menemukan simbol rMQR di dalam gambar dari satu finder dan finder sub-pattern di sudut seberang: kenapa satu finder tidak bisa menempatkan ujung jauh simbol yang panjangnya sampai 139 modul, orientasi dari timing pattern atas, salinan format di sisi finder dan seberapa lemah sebuah kecocokan, menemukan sub-pattern lewat cincin terangnya karena cincin gelapnya tidak punya separator, memprediksi letaknya dengan peta afine yang dikoreksi untuk perspektif, homografi delapan titik dan fit di sepanjang tepi panjang, salinan format kedua sebagai cadangan, timing rows yang memastikan sebuah simbol beserta stage-stage error contract yang dibangun di atasnya, tempat pencarian ini bergabung ke pipeline setelah QR dan Micro QR, dan zxing-cpp sebagai prior art.

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
| 11-rs-decoding.id.md                   | `qrgen/rs_decode.go`       |
| 12-image-processing.id.md              | `qrgen/decode_image.go`|
| 13-decoder-pipeline.id.md              | `qrgen/decode.go`    |
| 14-adaptive-thresholding.id.md         | `qrgen/decode_image_sauvola.go` |
| 15-rotation-handling.id.md             | `qrgen/decode_image.go` (`orderFinderTriple`) |
| 16-svg-rendering.id.md                 | `qrgen/render_svg.go` |
| 17-optimal-segmentation.id.md          | `qrgen/segment.go` |
| 18-payload-formats.id.md               | `qrgen/payload.go` |
| 19-terminal-rendering.id.md            | `qrgen/render_terminal.go` |
| 20-eci-segments.id.md                  | `qrgen/eci.go` |
| 21-structured-append.id.md             | `qrgen/structured_append.go` |
| 22-kanji-mode.id.md                    | `qrgen/kanji.go` |
| 23-logo-embedding.id.md                | `qrgen/logo.go` |
| 24-arbitrary-angle-detection.id.md     | `qrgen/decode_finder_cc.go`, `qrgen/decode_image.go` |
| 25-multi-symbol-decoding.id.md          | `qrgen/decode_multi.go`, `qrgen/decode_finder_cc.go` |
| 26-micro-qr.id.md                       | `qrgen/micro.go`, `qrgen/micro_encode.go`, `qrgen/micro_matrix.go`, `qrgen/micro_decode.go` |
| 27-micro-qr-detection.id.md             | `qrgen/decode_micro.go`, `qrgen/decode_finder_cc.go`, `qrgen/decode_image.go`, `qrgen/decode_multi.go` |
| 28-rmqr.id.md                           | `qrgen/rmqr.go`, `qrgen/rmqr_encode.go`, `qrgen/rmqr_matrix.go`, `qrgen/rmqr_decode.go` |
| 29-symbol-inspection.id.md              | `qrgen/inspect.go`, `web/internal/bridge/bridge.go`, `web/site/inspect.js` |
| 30-rmqr-detection.id.md                 | `qrgen/decode_rmqr.go`, `qrgen/decode_finder_cc.go`, `qrgen/decode_image.go`, `qrgen/decode_multi.go` |

Bila mengubah algoritma, mohon perbarui dokumen terkait di folder ini. Dokumen teori adalah penjelasan tahan lama tentang *mengapa* kode terlihat seperti sekarang.
