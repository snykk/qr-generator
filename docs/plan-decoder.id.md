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

- [ ] `docs/theory/11-rs-decoding.md` — syndrome, Berlekamp–Massey, Chien search, Forney, koreksi error vs. erasure.
- [ ] `docs/theory/12-image-processing.md` — konversi grayscale, Otsu thresholding, scan finder-pattern, homography, refinement alignment-pattern.
- [ ] `docs/theory/13-decoder-pipeline.md` — alur menyeluruh `image → matrix → text` plus aturan error-handling.
- [ ] Versi Indonesia untuk masing-masing.
- [ ] Update index `docs/theory/README.md` agar mencakup entri baru.

### D2 — Operasi GF(256) Sisi Decoder `(S)`

Tujuan: perluas `qrgen/gf256.go` dengan operasi field dan polynomial yang dibutuhkan RS decoding.

- [ ] `gf256Inverse(a) uint8` — invers perkalian memakai tabel log/exp yang sudah ada.
- [ ] `polyDivQR(dividend, divisor) (quotient, remainder []uint8)` — division penuh yang mengembalikan keduanya (`polyMod` yang ada hanya mengembalikan remainder).
- [ ] `polyEval(p []uint8, x uint8) uint8` — evaluasi Horner, dipakai Chien search.
- [ ] `polyDeriv(p []uint8) []uint8` — turunan formal (dependency algoritma Forney).
- [ ] Tests: round-trip `polyMul`/`polyDivQR`; verifikasi `polyEval` cocok dengan komputasi langsung; spot-check `gf256Inverse` lewat `a · a⁻¹ = 1` di seluruh field.

### D3 — Decoder Reed–Solomon `(M)`

Tujuan: `rsDecode(block []byte, n int) ([]byte, error)` yang memulihkan `block[:k]` dari maksimum `floor(n/2)` codeword yang korup.

- [ ] **Kalkulasi syndrome** — `n` syndrome dengan mengevaluasi polinomial yang diterima pada `α⁰..α^(n−1)`.
- [ ] **Berlekamp–Massey** — temukan polinomial error-locator Λ(x).
- [ ] **Chien search** — temukan akar Λ(x) di atas GF(256) → posisi error.
- [ ] **Algoritma Forney** — hitung magnitude error dari polinomial evaluator.
- [ ] Kembalikan `ErrTooManyErrors` ketika degree(Λ) melebihi kapasitas koreksi atau tidak ada solusi valid.
- [ ] Tests: korup 1..⌊n/2⌋ byte di block encoded HELLO WORLD dari M4 dan assert pemulihan persis.

### D4 — Reader Format Information `(S)`

Tujuan: baca codeword format 15-bit dari matrix dan recover (EC level, mask).

- [ ] Baca kedua salinan redundan codeword 15-bit.
- [ ] Decoder BCH(15,5) lewat brute force pada 32 entri precomputed dari M2: hitung jarak Hamming ke setiap entri, ambil minimum, jumlahkan jarak antar kedua salinan.
- [ ] Kembalikan `ErrFormatUnreadable` hanya bila kedua salinan terlalu jauh dari semua codeword valid.
- [ ] Ekstrak `ECLevel` dan `mask`.
- [ ] Tests: feed tiap pasangan (EC, mask) valid plus varian yang sengaja di-flip bit-nya hingga kapasitas koreksi BCH (3 error).

### D5 — Mask Reversal & Walk Area Data `(S)`

Tujuan: balikkan zig-zag walk dari M5 untuk menghasilkan byte stream codeword ter-interleave dari (versi, mask, matrix) yang sudah diketahui.

- [ ] Pakai ulang walk `placeData` secara terbalik: iterasi jalur yang sama dan *baca* bit dari sel yang tidak reserved.
- [ ] Terapkan XOR mask sebelum membaca (encoder menerapkannya setelah penempatan data, jadi baca harus undo).
- [ ] Buang remainder bits sesuai `Version.RemainderBits()`.
- [ ] Kembalikan `[]byte` interleaved mentah.
- [ ] Tests: encode HELLO WORLD → matrix → balikkan walk → assert byte-for-byte sama dengan output `rsEncode`.

### D6 — Deinterleaving Block + Koreksi Error `(M)`

Tujuan: balikkan interleave kolom-major dari M4 dan jalankan `rsDecode` di setiap block.

- [ ] Hitung layout block dari `Version.ECBlocks(ec)` (pakai ulang tabel M2 yang sudah ada).
- [ ] Walk stream interleaved kolom demi kolom untuk memecahnya kembali menjadi slice data + EC per block.
- [ ] Jalankan `rsDecode` di setiap block; teruskan `ErrTooManyErrors` bila ada block yang gagal.
- [ ] Concat codeword data yang sudah dikoreksi dari semua block menjadi satu byte stream.
- [ ] Tests: round-trip setiap kelas (versi, EC) V1..V40 dengan meng-encode payload acak, opsional flip bit dalam budget, dan konfirmasi output terkoreksi cocok dengan data codeword asli.

### D7 — Bit Stream → Text + Public API `DecodeMatrix` `(M)`

Tujuan: parse stream codeword data kembali menjadi teks sumber, lalu expose sebagai public function.

- [ ] Baca mode indicator 4-bit dan dispatch berdasarkan mode.
- [ ] Baca character count indicator memakai `Mode.CharCountBits(v)` (pakai ulang dari M3).
- [ ] Decoder per-mode: numeric (group 10 / 7 / 4 bit), alphanumeric (pasangan 11 / tunggal 6 bit), byte (raw 8-bit → string UTF-8).
- [ ] Stop saat terminator atau end-of-stream; abaikan pad bytes.
- [ ] Public API: `qrgen.DecodeMatrix([][]bool) (string, error)` — jalankan D4 → D5 → D6 → D7.
- [ ] Tests: encode → DecodeMatrix round-trip untuk setiap test fixture yang dipakai di `roundtrip_test.go`.

### ✅ Checkpoint 1 — Decoder Matrix-ke-Text sudah feature-complete.

### D8 — Preprocessing Image `(S)`

Tujuan: ubah `image.Image` apa pun menjadi grid biner 2D yang siap untuk deteksi pattern.

- [ ] Konversi ke grayscale single-channel (handle `image.Gray`, `image.RGBA`, `image.NRGBA`).
- [ ] **Otsu thresholding** — temukan threshold global yang meminimalkan within-class variance.
- [ ] Fallback opsional local thresholding untuk image dengan pencahayaan sangat tidak merata (Sauvola atau block-based).
- [ ] Kembalikan struct `bitmap` (width, height, `[]bool` untuk sel).
- [ ] Tests: image gradien sintetis, image kontras rendah, dan PNG encoder kita sendiri (di mana output seharusnya cocok dengan matrix asli secara persis).

### D9 — Deteksi Finder Pattern `(M)`

Tujuan: tempatkan tiga finder pattern di bitmap.

- [ ] Horizontal scan untuk **rasio gelap/terang 1:1:3:1:1** di antar baris.
- [ ] Vertical scan untuk konfirmasi kandidat.
- [ ] Cluster centre kandidat dan validasi **geometri triple** (segitiga siku-siku, ukuran modul yang mirip).
- [ ] Hitung pitch modul estimasi dari jarak antar finder.
- [ ] Kembalikan tiga centre `(x, y)` terurut sebagai kiri-atas, kanan-atas, kiri-bawah.
- [ ] `ErrFinderNotFound` bila kurang dari tiga finder valid yang terdeteksi.
- [ ] Tests: deteksi finder di PNG encoder kita sendiri pada berbagai ukuran dan rotasi.

### D10 — Perspective Transform `(M)`

Tujuan: peta koordinat pixel sumber → koordinat modul grid.

- [ ] Perkirakan sudut keempat (kanan-bawah) dari tiga centre finder + geometri yang bergantung versi.
- [ ] Hitung **matriks homography 3×3** yang memetakan (koordinat modul matrix) → (koordinat pixel sumber).
- [ ] Sediakan inverse map untuk sampling.
- [ ] Tests: round-trip segitiga finder yang diketahui melewati transform.

### D11 — Refinement Alignment Pattern (V2+) `(S)`

Tujuan: refine perspective transform memakai alignment pattern untuk mengurangi error sampling pada versi tinggi.

- [ ] Untuk tiap centre alignment-pattern yang diharapkan, cari di jendela kecil pada image sumber untuk alignment pattern 5×5.
- [ ] Adjust homography atau interpolasi koreksi lokal.
- [ ] Skip dengan bersih ketika alignment pattern tidak ditemukan (V1 selalu; simbol yang sangat rusak).

### D12 — Sampling Modul + Public API `Decode` `(M)`

Tujuan: ikat pipeline image dan matrix.

- [ ] Pada tiap centre modul, sample image sumber (titik atau rata-rata 3×3) dan threshold.
- [ ] Bangun matrix `[][]bool` dan hand off ke `DecodeMatrix`.
- [ ] Public API: `qrgen.Decode(img image.Image) (string, error)` dan convenience `qrgen.DecodeBytes(data []byte) (string, error)`.
- [ ] Tests: encode payload via encoder kita → render PNG → round-trip `Decode` di antar mode dan versi. Tambah test rotasi/scale sintetis.

### ✅ Checkpoint 2 — Decoder Image-ke-Text sudah feature-complete.

### D13 — Quality Gate `(M)`

Tujuan: pastikan decoder robust sebelum release.

- [ ] Cross-validation: encode → decode round-trip pada 12-case matrix yang sama yang dipakai `roundtrip_test.go` encoder (sekarang menutup loop tanpa decoder pihak ketiga).
- [ ] Robustness: sengaja korup N byte per block (dalam kapasitas RS) dan assert pemulihan.
- [ ] Robustness image: render dengan warna kustom, kontras rendah, dirotasi, di-downscale.
- [ ] Benchmarks: `BenchmarkDecodeSmall`, `BenchmarkDecodeMultiBlock`, `BenchmarkDecodeImage`.
- [ ] `go test -race ./...` tetap bersih.

### D14 — Polish & Release `(S)`

Tujuan: tag `v0.2.0`.

- [ ] Update README: baris API summary baru, contoh penggunaan decode, update Limitations (decoder ditambahkan, ECI/Kanji masih tertunda).
- [ ] Entri `CHANGELOG.md` `v0.2.0` di bawah "Added" dan "Validated".
- [ ] `examples/decode/main.go` baru yang menampilkan `Decode` pada PNG yang sudah disimpan.
- [ ] Tag `v0.2.0`.

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
