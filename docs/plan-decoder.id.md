# QR Decoder — Plan

Dokumen ini menjelaskan rencana implementasi fitur **decoder** QR yang menargetkan rilis minor `v0.2.0`. Paralel dengan [`docs/plan.id.md`](plan.id.md) yang mencakup encoder untuk `v0.1.0` dan sekarang sebagian besar sudah selesai.

> Status: **draft / living document.** Milestone D1..D14 didaratkan secara bertahap; setiap milestone adalah commit (atau rangkaian commit kecil) yang fokus, lengkap dengan tests, sama seperti milestone encoder M1..M11.

> Versi English: [docs/plan-decoder.md](plan-decoder.md).

---

## 1. Visi & Tujuan

- Menambahkan **decoder** ke package `qrgen` yang membaca simbol QR kembali menjadi teks aslinya, dengan dua entry point:
  - `qrgen.DecodeMatrix([][]bool) (string, error)` — bekerja pada matrix boolean top-down yang sudah bersih; berguna untuk caller yang sudah punya matrix.
  - `qrgen.Decode(img image.Image) (string, error)` — membaca image nyata (foto kamera, scan, PNG hasil generate) dan menjalankan pipeline penuh.
- Mempertahankan **filosofi yang sama** dengan encoder: pure Go, zero runtime dependencies di luar standard library, spec-first, dengan theory docs bilingual dan golden test fixtures.
- Cross-validate terhadap encoder agar package menjadi loop tertutup sungguhan: output encoder yang didecode oleh decoder kita sendiri harus round-trip persis di semua mode, versi, dan EC level.

## 2. Prinsip Desain

1. **Pipeline dua tahap.** `DecodeMatrix` melakukan logika murni (RS decoding, mask reversal, parsing bit-stream). `Decode` menambahkan image processing di atasnya (binarisasi, deteksi finder, perspective transform, sampling). Keduanya bisa di-test secara terpisah.
2. **Decoding Reed–Solomon adalah hal tersendiri.** RS encoding (M4) memakai polynomial division; RS decoding memakai syndrome, Berlekamp–Massey, Chien search, dan algoritma Forney. Kita perlakukan ini sebagai topik baru dengan theory doc sendiri, bukan "kebalikan dari M4".
3. **Toleran terhadap input dunia nyata.** Decoder menerima input yang noisy / skewed / rusak sebagian. Decoder harus memakai budget error-correction spec (`floor(n/2)` codeword korup per block) sebelum menyatakan gagal.
4. **Error return yang jelas.** Kegagalan decode dikembalikan sebagai typed errors (`ErrFinderNotFound`, `ErrFormatUnreadable`, `ErrTooManyErrors`, …) sehingga caller bisa branch berdasarkan penyebab.
5. **Pure Go, tanpa CGo, tanpa library CV pihak ketiga.** Image processing dilakukan dengan `image.Image` + routine binarisasi dan homography kustom yang kecil.

## 3. Scope

### In scope untuk rilis decoder

- Mode decoding: **numeric, alphanumeric, byte** (mirror dari scope encoder).
- Seluruh **40 versi QR standar**.
- Seluruh **4 EC level** (L, M, Q, H), dengan error-correction penuh hingga budget spec.
- Dua entry point: berbasis matrix (`DecodeMatrix`) dan berbasis image (`Decode`).
- Helper convenience `DecodeBytes([]byte) (string, error)` untuk PNG bytes di memori.

### Out of scope (masih)

- Mode Kanji dan segmen ECI — simetris dengan keterbatasan encoder.
- Micro QR / rMQR.
- Rakit ulang structured-append dari banyak simbol.
- Menemukan banyak QR dalam satu image.
- Decoding berbantu ML untuk simbol yang rusak parah.

---

## 4. Milestones

Milestone dipecah menurut checkpoint. **Checkpoint 1** (setelah D7) memberikan decoder matrix-ke-teks yang berfungsi. **Checkpoint 2** (setelah D12) memperluas ke image-ke-teks. **Checkpoint 3** (D14) adalah rilis `v0.2.0`.

### D1 — Theory Docs Decoder `(S)`

Tujuan: dokumentasikan algoritma baru di `docs/theory/` sebelum kode masuk, mengikuti pendekatan spec-first dari encoder.

- [x] `docs/theory/11-rs-decoding.md` — syndrome, Berlekamp–Massey, Chien search, Forney, koreksi error vs. erasure.
- [x] `docs/theory/12-image-processing.md` — konversi grayscale, Otsu thresholding, scan finder-pattern, homography, refinement alignment-pattern.
- [x] `docs/theory/13-decoder-pipeline.md` — alur menyeluruh `image → matrix → text` plus aturan error-handling.
- [x] Versi Indonesia untuk masing-masing.
- [x] Update index `docs/theory/README.md` agar mencakup entri baru.

### D2 — Operasi GF(256) Sisi Decoder `(S)`

Tujuan: perluas `qrgen/gf256.go` dengan operasi field dan polynomial yang dibutuhkan RS decoding.

- [x] `gf256Inverse(a) uint8` — invers perkalian memakai tabel log/exp yang sudah ada. Panic untuk input nol.
- [x] `polyDivQR(dividend, divisor) (quotient, remainder []uint8)` — division penuh yang mengembalikan keduanya; toleran terhadap divisor non-monic dengan menormalkan koefisien pemimpin.
- [x] `polyEval(p []uint8, x uint8) uint8` — evaluasi Horner, dipakai kalkulasi syndrome, Chien search, dan Forney.
- [x] `polyDeriv(p []uint8) []uint8` — turunan formal; hanya menyimpan term berdegree ganjil (kolaps karakteristik-2).
- [x] Tests: sweep eksahustif 255-elemen untuk `gf256Inverse` (`a · a⁻¹ = 1`); test panic untuk input nol; kasus table-driven untuk `polyEval` / `polyDeriv`; `polyDivQR` correctness pada kasus langsung plus 11-pair property test yang merekonstruksi dividend via `q · divisor + r`.

### D3 — Decoder Reed–Solomon `(M)`

Tujuan: `rsDecode(block []byte, n int) ([]byte, error)` yang memulihkan `block[:k]` dari maksimum `floor(n/2)` codeword yang korup.

- [x] **Kalkulasi syndrome** — `n` syndrome dengan mengevaluasi polinomial yang diterima pada `α⁰..α^(n−1)` lewat `polyEval`.
- [x] **Berlekamp–Massey** — `berlekampMassey` bekerja di lowest-degree-first secara internal dan mengembalikan Λ yang sudah dibalik ke high-degree-first untuk tahap selanjutnya.
- [x] **Chien search** — `chienSearch` mengembalikan slice paralel `(positions, locators)` untuk pipeline selanjutnya.
- [x] **Algoritma Forney** — `forneyMagnitudes` memakai bentuk standar `Y_k = X_k · Ω(X_k^{-1}) / Λ'(X_k^{-1})` (akar generator mulai dari α⁰).
- [x] Kembalikan `ErrTooManyErrors` ketika degree(Λ) melebihi kapasitas koreksi atau jumlah posisi tidak sesuai dengan `L`.
- [x] Tests: fixture HELLO WORLD dengan korupsi 0, 1, 2..5 byte, bucket over-capacity, dan property test 250 trial acak di bentuk block V1-M / V1-L / V1-H / V5-M / lebih besar.

### D4 — Reader Format Information `(S)`

Tujuan: baca codeword format 15-bit dari matrix dan recover (EC level, mask).

- [x] Baca kedua salinan redundan codeword 15-bit. — `qrgen/format_decode.go` `readFormatInfo`
- [x] Decoder BCH(15,5) lewat brute force pada 32 entri precomputed dari M2: jarak Hamming gabungan minimum yang menang. — joint budget 6 (3+3) sesuai jarak minimum BCH.
- [x] Kembalikan `ErrFormatUnreadable` hanya bila kedua salinan melebihi budget. — sentinel diekspor di samping decoder.
- [x] Ekstrak `ECLevel` dan `mask`.
- [x] Tests: round-trip 32 pasangan plus kasus korupsi per-copy dan gabungan termasuk skenario asimetris "satu bersih, satu rusak".

### D5 — Mask Reversal & Walk Area Data `(S)`

Tujuan: balikkan zig-zag walk dari M5 untuk menghasilkan byte stream codeword ter-interleave dari (versi, mask, matrix) yang sudah diketahui.

- [x] Pakai ulang walk `placeData` secara terbalik via `qrgen/decode_matrix.go` `readCodewordStream`, plus `matrixFromGrid` yang membangun ulang mask area reserved dari input `[][]bool`.
- [x] XOR mask dibatalkan saat membaca sehingga `applyMask` encoder terurai.
- [x] Buang remainder bits sesuai `Version.RemainderBits()`.
- [x] Kembalikan `[]byte` interleaved mentah.
- [x] Tests: round-trip lintas V1..V10 (single-block, multi-block, dengan version info) dan kedelapan mask; validasi ukuran matrix / row tidak rata di `matrixFromGrid`.

### D6 — Deinterleaving Block + Koreksi Error `(M)`

Tujuan: balikkan interleave kolom-major dari M4 dan jalankan `rsDecode` di setiap block.

- [x] Hitung layout block dari `Version.ECBlocks(ec)` (pakai ulang tabel M2 yang sudah ada) di `deinterleaveBlocks`.
- [x] Walk stream interleaved kolom demi kolom untuk memecahnya kembali menjadi slice data + EC per block, mirror dari interleaver encoder.
- [x] Jalankan `rsDecode` di setiap block via `deinterleaveAndCorrect`; bungkus `ErrTooManyErrors` dengan indeks block yang gagal.
- [x] Concat codeword data yang sudah dikoreksi dari semua block menjadi satu byte stream.
- [x] Tests: reversal layout per-block V1-M / V1-H / V5-Q / V10-M; round-trip dengan flip 0 / 5 / 6 (dalam dan di luar budget V1-M).

### D7 — Bit Stream → Text + Public API `DecodeMatrix` `(M)`

Tujuan: parse stream codeword data kembali menjadi teks sumber, lalu expose sebagai public function.

- [x] Baca mode indicator 4-bit dan dispatch berdasarkan mode via `decodeText` + `bitReader`.
- [x] Baca character count indicator memakai `Mode.CharCountBits(v)` (pakai ulang dari M3).
- [x] Decoder per-mode: `decodeNumeric` (group 10 / 7 / 4 bit), `decodeAlphanumeric` (11 / 6 bit), `decodeByteMode` (raw 8-bit → UTF-8).
- [x] Stop saat terminator (`0000`) atau ketika kurang dari 4 bit tersisa; pad bytes diabaikan secara implisit.
- [x] Public API: `qrgen.DecodeMatrix([][]bool) (string, error)` di `qrgen/decode.go` — jalankan D4 → D5 → D6 → D7.
- [x] Tests: 15 kasus round-trip lintas mode, EC level, V1..V10, multi-block, version-info, dan forced version+mask, plus coverage typed-error untuk input korup.

### Checkpoint 1 — Decoder Matrix-ke-Text sudah feature-complete.

### D8 — Preprocessing Image `(S)`

Tujuan: ubah `image.Image` apa pun menjadi grid biner 2D yang siap untuk deteksi pattern.

- [x] Konversi ke grayscale single-channel (handle `image.Gray`, `image.RGBA`, `image.NRGBA`). — `qrgen/decode_image.go` `imageToGrayscale` via `color.GrayModel`.
- [x] **Otsu thresholding** — temukan threshold global yang memaksimalkan between-class variance. — `qrgen/decode_image.go` `otsuThreshold`.
- [ ] Fallback opsional local thresholding untuk image dengan pencahayaan sangat tidak merata (Sauvola atau block-based). — ditunda ke v0.3 sesuai open question §8.
- [x] Kembalikan struct `bitmap` (width, height, `[]bool` untuk sel). — `qrgen/decode_image.go` `bitmap` + helper `get`.
- [x] Tests: kasus tepi monokrom, histogram bimodal, sub-image dengan bound bukan-nol, dan cek integrasi per-modul bahwa tiap sel dari PNG encoder kita kembali terklasifikasi dengan benar melalui `binarise`.

### D9 — Deteksi Finder Pattern `(M)`

Tujuan: tempatkan tiga finder pattern di bitmap.

- [x] Horizontal scan untuk **rasio gelap/terang 1:1:3:1:1** di antar baris. — `qrgen/decode_image.go` `scanRowForFinders`.
- [x] Vertical scan untuk konfirmasi kandidat. — `crossCheckVertical` cross-validate tiap row hit dan refine centre y.
- [x] Cluster centre kandidat dan validasi **geometri triple** (segitiga siku-siku, ukuran modul yang mirip). — `clusterFinderCandidates` + `orderFinderTriple` (rasio leg < 1.5, Pythagoras toleransi 15%).
- [x] Hitung pitch modul estimasi dari jarak antar finder. — diekspos via `finderCandidate.moduleSize`, rata-rata dari fit horizontal dan vertikal.
- [x] Kembalikan tiga centre `(x, y)` terurut sebagai kiri-atas, kanan-atas, kiri-bawah. — asumsi simbol right-side-up; support rotasi penuh ditunda.
- [x] `ErrFinderNotFound` diekspor saat kurang dari tiga finder valid yang bertahan setelah clustering atau geometri triple-nya tidak masuk akal.
- [x] Tests: PNG encoder V1 dan forced-V5 dengan toleransi centre ±2px, plus penolakan image all-white dan kasus negatif sudut yang dihapus. Rotasi arbitrer masuk roadmap.

### D10 — Perspective Transform `(M)`

Tujuan: peta koordinat pixel sumber → koordinat modul grid.

- [x] Perkirakan sudut keempat (kanan-bawah) dari tiga centre finder + geometri yang bergantung versi. — `qrgen/decode_image.go` `estimateBottomRight` (parallelogram completion) plus `estimateVersion` dari `(distance − 14) / 4 + 1`.
- [x] Hitung **matriks homography 3×3** yang memetakan (koordinat modul matrix) → (koordinat pixel sumber). — `computeHomography` menyelesaikan sistem linear 8×8 standar dengan `solveLinear8` (Gaussian elimination + partial pivoting + singularity guard).
- [x] Sediakan inverse map untuk sampling. — `homography.apply(col, row)` adalah forward map yang dipakai langsung oleh sampler; untuk decoder QR kita hanya butuh module → pixel sehingga tidak perlu helper inverse terpisah.
- [x] Tests: identity round-trip, translate-and-scale, degenerate (collinear) input mengembalikan error, estimasi versi V1/V5/V10, dan sampling per-modul HELLO WORLD V1 di mana tiap bit yang di-sample match matrix asli.

### D11 — Refinement Alignment Pattern (V2+) `(S)`

Tujuan: refine perspective transform memakai alignment pattern untuk mengurangi error sampling pada versi tinggi.

- [x] Untuk tiap centre alignment-pattern yang diharapkan, cari di jendela kecil pada image sumber untuk alignment pattern 5×5. — `qrgen/decode_image.go` `checkAlignmentAt` + `searchAlignmentPattern`.
- [x] Adjust homography dengan mengganti anchor BR yang di-complete parallelogram dengan centre alignment-pattern yang ditemukan. — `refineHomography`.
- [x] Skip dengan bersih ketika alignment pattern tidak ditemukan (V1 selalu; simbol yang sangat rusak). — fallback ke transform input bila search window tidak punya pattern valid atau bila sistem yang dihitung ulang singular.

### D12 — Sampling Modul + Public API `Decode` `(M)`

Tujuan: ikat pipeline image dan matrix.

- [x] Pada tiap centre modul, sample image sumber (point sample) dan threshold. — `qrgen/decode_image.go` `sampleMatrix` membaca satu pixel per modul dari bitmap yang sudah ter-binarisasi via homography yang sudah direfine.
- [x] Bangun matrix `[][]bool` dan hand off ke `DecodeMatrix`. — `decodeImage` merangkai seluruh tahap image dan mendelegasikan ke decoder matrix.
- [x] Public API: `qrgen.Decode(img image.Image) (string, error)` dan convenience `qrgen.DecodeBytes(data []byte) (string, error)`. — keduanya di `qrgen/decode.go`; `DecodeBytes` mendaftarkan codec PNG/JPEG/GIF.
- [x] Tests: 10 kasus round-trip Encode → DecodeBytes lintas mode, V1..V10, multi-block, warna kustom, module/quiet zone lebih besar, plus `Decode(image.Image)` dan jalur error bytes invalid.

### Checkpoint 2 — Decoder Image-ke-Text sudah feature-complete.

### D13 — Quality Gate `(M)`

Tujuan: pastikan decoder robust sebelum release.

- [x] Cross-validation: encode → decode round-trip pada 12-case matrix yang sama yang dipakai `roundtrip_test.go` encoder, sekarang menutup loop dengan `DecodeBytes` kita sendiri. — `qrgen/decode_roundtrip_test.go` `TestRoundTripWithOwnDecoder`.
- [x] Robustness: sengaja korup N modul di matrix (dalam kapasitas RS) dan assert pemulihan. — `TestRoundTripRobustnessFlippedBits` mencakup V1-Q dengan 3 flip dan V1-H dengan 5 flip.
- [x] Robustness image: render dengan warna kustom, kontras rendah, ukuran modul beragam, quiet zone lebih besar. — `TestRoundTripImageRobustness`. Rotasi arbitrer tetap roadmap.
- [x] Benchmarks: `BenchmarkDecodeMatrixSmall` / `MultiBlock`, `BenchmarkDecodeImageSmall` / `MultiBlock` / `URL`, plus `FromPNGDecode` yang memisahkan biaya CV dari parsing PNG. — `qrgen/decode_bench_test.go`.
- [x] `go test -race ./...` tetap bersih.

### D14 — Polish & Release `(S)`

Tujuan: tag `v0.2.0`.

- [x] Update README: baris API summary baru untuk `Decode`/`DecodeBytes`/`DecodeMatrix` plus lima sentinel error, section khusus "Decoding QR codes", update Scope/Limitations/Roadmap yang mencerminkan permukaan v0.2.
- [x] Entri `CHANGELOG.md` `v0.2.0` di bawah "Added" dan "Validated" dengan rujukan konkret ke tahap image, tahap matrix, public API, error, theory docs, dan bukti validasi.
- [x] `examples/decode/main.go` baru dengan demo round-trip built-in plus argumen `path` opsional untuk decode PNG yang sudah ada.
- [x] CLI `cmd/qrgen` mendapat mode `-decode` dengan `-in` untuk file input (atau stdin), `-out` untuk teks hasil (atau stdout), plus section help / README yang sesuai. — melengkapi permukaan v0.2 sehingga encoder dan decoder sama-sama bisa diakses dari binary.
- [ ] Tag `v0.2.0`. — ditunda untuk push pertama ke GitHub agar tag mendarat di commit yang dilihat remote; jalankan `git tag -a v0.2.0 -m "Initial decoder release" && git push origin v0.2.0`.

---

## 5. Delta Layout Folder yang Diusulkan

Kode decoder mendarat berdampingan dengan encoder di package yang sama, sehingga user bisa memanggil `qrgen.Decode` secara simetris dengan `qrgen.Encode`. Pembagian file yang disarankan:

```
qrgen/
├── decode.go              # public Decode / DecodeMatrix / DecodeBytes
├── decode_matrix.go       # mask reversal, walk area data terbalik, parsing bit-stream
├── decode_image.go        # binarisasi, deteksi finder, homography, sampling modul
├── rs_decode.go           # syndrome, Berlekamp-Massey, Chien, Forney
├── format_decode.go       # reader format-info 15-bit dengan koreksi error BCH
├── gf256.go               # diperluas dengan Inverse, PolyDivQR, PolyEval, PolyDeriv
└── *_test.go              # mirror tests per file
```

## 6. Risiko & Catatan Teknis

- **Correctness Berlekamp–Massey** adalah komponen dengan risiko bug tertinggi. Mitigasi: validasi terhadap worked example (10 EC codeword HELLO WORLD dengan korupsi sengaja) plus property test atas block acak.
- **False positive deteksi finder-pattern** bisa mengalahkan deteksi pada background ramai. Mitigasi: wajibkan check geometri siku-siku; reject triple yang jarak antar finder-nya sangat berbeda.
- **Stabilitas numerik homography** pada versi tinggi. Mitigasi: pakai `float64` di seluruh, lebih memilih least-squares daripada inversi langsung.
- **Handling UTF-8 saat decode** mencerminkan keterbatasan encoder: byte-mode bytes diperlakukan sebagai UTF-8 tanpa konsultasi segmen ECI mana pun. Ini didokumentasikan sebagai keterbatasan yang diketahui, bukan bug.
- **Ukuran library** tumbuh signifikan. Kita tidak boleh meregresi benchmark encoder; pertimbangkan menjaga decoder di belakang build tag jika sampai membengkakkan binary untuk user encode-only. Mungkin tidak diperlukan untuk v0.2 tapi worth diukur.

## 7. Referensi

- ISO/IEC 18004:2015 — §9 (Reference decode algorithm), Annex C / D (BCH codes), Annex E (alignment positions).
- Berlekamp, E. R. — *Algebraic Coding Theory* (1968), algoritma Berlekamp asli.
- Massey, J. L. — "Shift-Register Synthesis and BCH Decoding," IEEE Trans. Info. Theory, 1969.
- Forney, G. D. — "On Decoding BCH Codes," IEEE Trans. Info. Theory, 1965.
- Proyek ZXing — *referensi decoder open-source*: <https://github.com/zxing/zxing>.
- Project Nayuki — *catatan companion decoder QR Code generator library*.

## 8. Pertanyaan Terbuka

Untuk dijawab sebelum milestone terkait dimulai:

- **Berlekamp–Massey vs. Peterson–Gorenstein–Zierler.** Yang kedua lebih sederhana secara konseptual tapi kurang efisien. Kita akan mulai dengan Berlekamp–Massey untuk match standar komunitas QR.
- **Binarisasi lokal vs. global.** Otsu bekerja untuk sebagian besar input sintetis; apakah kita ship local thresholding di v0.2 atau ditunda ke v0.3?
- **Format input sisi-image.** Hanya `image.Image`, atau juga raw bytes + sniffing content-type (PNG / JPEG / dll.)? `image.Decode` sudah cover PNG/JPEG/GIF, jadi mungkin "hanya `image.Image`" plus convenience `DecodeBytes`.
- **Hierarki tipe error.** Sentinel error (`var ErrXxx = errors.New(...)`) atau struct `DecodeError` typed? Default ke sentinel untuk v0.2; revisit jika caller ingin info lebih kaya.
