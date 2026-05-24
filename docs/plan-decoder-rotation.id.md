# QR Decoder — Plan Rotation Handling

Dokumen ini menjelaskan rencana implementasi enhancement **rotation handling** yang menargetkan rilis minor `v0.4.0`. Ini adalah cabang robustness kedua yang dipecah dari Roadmap v0.2.0 (cabang pertama, **adaptive thresholding**, sudah rilis sebagai `v0.3.0` dari branch `decoder-thresholding`).

> Status: **draft / dokumen hidup.** Milestone R1..R6 dikerjakan bertahap di branch `decoder-rotation`; tiap milestone berupa commit fokus (atau seri commit kecil) yang sudah lengkap dengan tes, mengikuti ritme M1..M11, D1..D14, dan T1..T6.

> Versi Inggris: [docs/plan-decoder-rotation.md](plan-decoder-rotation.md).

---

## 1. Visi & Tujuan

- Mengangkat batasan **"No rotated-image decoding"** yang dicatat di README pada bagian `## Limitations` dan `## Roadmap` supaya decoder dapat memulihkan QR code dari image yang ter-rotasi 90 / 180 / 270 derajat (HP yang dipegang menyamping ketika men-scan halaman cetak adalah kasus kanonik) plus soft tilt hingga sekitar 30 derajat off-axis.
- Menjaga permukaan API publik identik: caller terus memanggil `qrgen.Decode` / `qrgen.DecodeBytes` / `qrgen.DecodeMatrix` tanpa perubahan. Semua perilaku baru bersifat internal di tahap finder-ordering.
- Tetap theory-first dan bilingual: menulis `docs/theory/15-rotation-handling.md` (EN + ID) sebelum kode apa pun mendarat, dalam semangat yang sama dengan urutan v0.3 (plan -> theory -> code).
- Tetap pure Go: tanpa third-party dependency baru, tanpa alokasi tambahan di hot path.

## 2. Prinsip Desain

1. **Cari bug struktural, bukan gejala permukaan.** Inspeksi [qrgen/decode_image.go:367-371](../qrgen/decode_image.go#L367-L371) menunjukkan bahwa `orderFinderTriple` sebenarnya sudah mengidentifikasi vertex sudut-siku-siku (top-left) via "vertex berlawanan dengan sisi terpanjang" — langkah itu sudah rotation-invariant. Satu-satunya langkah yang patah karena rotasi adalah diskriminator terakhir `if tr.y > bl.y` yang memutuskan mana di antara dua finder sisa yang top-right. Mengganti check tunggal itu dengan tes handedness cross-product cukup untuk membuka rotation handling.
2. **Percayai homography.** `homographyFromFinders` sudah menyelesaikan transform projektif 3x3 umum dari empat korespondensi anchor-point. Begitu `orderFinderTriple` mengembalikan label yang benar, homography menyerap rotasi, tilt, dan perspektif tanpa perubahan. Tidak perlu menyentuh jalur linear solver.
3. **Finder detection itu sendiri rotation-symmetric.** Row scan 1:1:3:1:1 menangkap finder pattern di orientasi apa pun karena pattern-nya berupa kotak konsentris — sebuah garis horizontal melalui pusatnya selalu melewati dark/light/dark/light/dark dengan rasio 1:1:3:1:1 (dalam toleransi ±50% per modul). Sama juga secara vertikal. Rotasi axis-aligned (90 / 180 / 270) dan soft tilt (~0–30 derajat) tidak butuh perubahan scanner.
4. **Tidak ada knob publik baru di v0.4.** Tidak ada functional option, tidak ada sentinel error baru. Tilt di luar ~30 derajat dan rotasi sembarang 0..360 tetap pekerjaan masa depan karena membutuhkan finder detector berbasis contour atau scanner 1:1:3:1:1 yang lebih fleksibel — keduanya rewrite signifikan yang tidak cocok untuk rilis minor.
5. **Tes lebih dulu.** Setiap milestone disertai Go test berbasis tabel dan minimal satu kasus round-trip yang gagal di `master` (`Decode` mengembalikan `ErrFinderNotFound`) dan lulus di branch (`Decode` mengembalikan payload asli).

## 3. Cakupan

### Termasuk di v0.4.0

- **Rotasi axis-aligned:** 90, 180, dan 270 derajat di kedua arah.
- **Soft tilt:** hingga sekitar 30 derajat off-axis (dibatasi oleh toleransi ±50% yang sudah ada di dalam `fitsFinderRatio`).
- **Rewrite `orderFinderTriple`:** ganti diskriminator koordinat `y` untuk gambar tegak dengan tes handedness cross-product yang bekerja di rotasi apa pun.
- **Theory doc** `15-rotation-handling.md` (EN + ID) yang membahas deteksi vertex sudut-siku-siku, handedness cross-product, sketsa bukti bahwa homography menangani sisanya, dan pernyataan cakupan eksplisit (axis-aligned + soft tilt di v0.4, sembarang 0..360 ditunda).
- **Fixture rotasi sintetis in-memory** dibangun secara prosedural di dalam file test: encode `"HELLO"` dengan API yang sudah ada, render matrix ke `*image.Gray` yang dirotasi sebesar 0 / 90 / 180 / 270 plus soft tilt (misal 15, 30 derajat) memakai aritmatika Go langsung, dan assert `DecodeBytes` round-trip.
- **Update dokumentasi:** menghapus `**No rotated-image decoding**` dari `## Limitations`; mengupdate `## Roadmap` untuk memfokuskan pekerjaan terbuka pada decoding sembarang sudut alih-alih seluruh kategori rotasi; menambah entry `v0.4.0` di CHANGELOG.

### Belum termasuk

- **Rotasi sembarang di [30, 90) derajat.** Scanner 1:1:3:1:1 dapat menghasilkan false negative pada sudut di mana tepi modul mengenai pixel baris pada sudut oblique; memulihkan band ini membutuhkan contour-tracing atau pencarian kipas orientasi. Ditunda ke rilis minor di masa depan.
- **Simbol QR mirror.** Check cross-product menolak handedness mirror by purpose karena QR code nyata tidak pernah mirror.
- **Multi-symbol detection.** Sama seperti v0.3: tidak masuk cakupan.
- **Kombinasi rotasi + pencahayaan tidak rata berat.** Fallback Sauvola v0.3 dan fix ordering v0.4 secara alami compose, tapi tidak ada fixture baru yang secara eksplisit menguji kombinasinya — recovery adalah union dari coverage tiap milestone.

---

## 4. Milestone

Milestone dikerjakan berurutan. **Checkpoint A** (setelah R2) memberi finder ordering rotation-invariant yang unit test-nya lulus di koordinat sintetis. **Checkpoint B** (R6) adalah rilis `v0.4.0`.

### R1 — Theory Doc Rotation Handling `(S)`

Goal: menutupi geometri dan algoritma di `docs/theory/` sebelum kode apa pun mendarat.

- [ ] `docs/theory/15-rotation-handling.md` — Kenapa `orderFinderTriple` yang ada gagal untuk simbol ter-rotasi (shortcut `tr.y > bl.y`), kenapa "vertex berlawanan dengan sisi terpanjang" sudah mengunci top-left di rotasi apa pun, identitas handedness cross-product `(TR - TL) x (BL - TL) > 0` dan analisis sign yang dikerjakan di 0 / 90 / 180 / 270 derajat di koordinat image (dengan `y` tumbuh ke bawah), kenapa identitas yang sama menolak simbol mirror dengan bersih, dan sketsa bukti pendek bahwa tahap homography menyerap rotasi setelah label-nya benar.
- [ ] Versi Indonesia `docs/theory/15-rotation-handling.id.md`.
- [ ] Update `docs/theory/README.md` dan `docs/theory/README.id.md` untuk menambah entry 15 dengan ringkasan satu baris di subsection baru "Decoder robustness (v0.4.0)", plus satu baris di tabel "Hubungan dengan kode" yang merujuk ke `qrgen/decode_image.go` (lingkungan `orderFinderTriple`).

### R2 — `orderFinderTriple` Rotation-Invariant `(S)`

Goal: mengganti diskriminator y-untuk-gambar-tegak dengan check handedness cross-product supaya pelabelan finder bekerja di rotasi apa pun.

- [ ] Pertahankan jalur "sisi terpanjang berlawanan dengan vertex sudut-siku-siku" yang ada, yang sudah memilih top-left dengan benar.
- [ ] Ganti blok `if tr.y > bl.y || (math.Abs(tr.y - bl.y) < 1 && tr.x < bl.x) { tr, bl = bl, tr }` dengan `if cross((tr - tl), (bl - tl)) < 0 { tr, bl = bl, tr }` supaya pelabelan bertahan di rotasi apa pun. (Konvensi sign-nya adalah cross product di koordinat image dengan `y` tumbuh ke bawah, sehingga kasus QR nyata yang tidak mirror duduk di sisi positif.)
- [ ] Pertahankan sanity check right-angle dan rasio kaki yang sudah ada tanpa perubahan; mereka sudah rotation-invariant.
- [ ] Unit test di `qrgen/decode_image_test.go` yang membangun tiga triple `finderCandidate` sintetis pada 0 / 90 / 180 / 270 derajat dan pada tilt 30 derajat, lalu assert `orderFinderTriple` menghasilkan identitas `(tl, tr, bl)` yang sama setiap kali.

### Checkpoint A — Ordering rotation-invariant kompil dan lulus tes level koordinat.

### R3 — Fixture Rotasi Sintetis `(M)`

Goal: mengunci coverage recovery end-to-end untuk rotasi axis-aligned dan soft tilt via generasi image in-memory.

- [ ] Tambahkan helper `rotateImage(src image.Image, angleDeg float64) *image.Gray` di dalam `qrgen/decode_rotation_test.go` yang me-render image sumber ke buffer gray baru dengan sampling bilinear, memakai rectangle tujuan yang cukup besar untuk menampung konten yang dirotasi plus quiet zone yang sudah ada. Background fill adalah warna quiet-zone sumber.
- [ ] Fixture `TestRotation90`, `TestRotation180`, `TestRotation270`: encode `"HELLO"`, rotasi sebesar sudut yang sesuai, assert `DecodeBytes` mengembalikan payload dan `decodeImageDebug` melaporkan `binariserOtsu` (rotasi seharusnya tidak mengganggu dispatch binariser).
- [ ] Fixture `TestRotationSoftTilt15` dan `TestRotationSoftTilt30`: bentuk yang sama tapi pada 15 dan 30 derajat; soft tilt ada di dalam toleransi rasio ±50% dan harusnya round-trip.
- [ ] Satu fixture negative eksplisit `TestRotationSoftTiltOutOfBand` pada 45 derajat yang meng-assert `ErrFinderNotFound` (tanpa rotasi), mendokumentasikan boundary v0.4 di dalam test suite itu sendiri.
- [ ] Jaga semua fixture in-process, V1 saja, sehingga batch rotation tetap di bawah 200 ms di laptop.

### R4 — Polish Dokumentasi `(S)`

Goal: meluruskan README dan CHANGELOG dengan apa yang dirilis.

- [ ] README `## Limitations`: hapus bullet `**No rotated-image decoding**`; ganti dengan `**Limited arbitrary-angle decoding**` yang mencatat 90 / 180 / 270 dan tilt hingga ~30 derajat bekerja tapi band 30..90 derajat di luar jangkauan sampai scanner diupdate. Tetap jujur soal cakupan.
- [ ] README `## Roadmap`: persempit bullet robustness decoder dari "arbitrary rotations" (sekarang sebagian selesai) ke "decoding sembarang sudut untuk band 30..90 derajat, contour-based finder detection".
- [ ] README `## Decoding QR codes`: tambah satu kalimat yang mengakui dukungan rotasi axis-aligned, menunjuk ke `docs/theory/15-rotation-handling.md`.
- [ ] Entry CHANGELOG `v0.4.0` di bawah `### Added` (cross-product handedness di `orderFinderTriple`, theory doc 15, plan doc, fixture rotation sintetis), `### Validated` (fixture R3, `go test -race` bersih, tidak ada regresi encoder).
- [ ] Checklist plan untuk R1..R6 di-tick.

### R5 — Benchmark & Pengaman Regresi `(S)`

Goal: mengkonfirmasi perubahan ordering allocation-neutral dan dalam noise run-to-run milik v0.3.

- [ ] Menjalankan ulang `BenchmarkDecodeImageSmall`, `BenchmarkDecodeImageMultiBlock`, `BenchmarkDecodeImageURL`, `BenchmarkDecodeImageFromPNGDecode`, `BenchmarkDecodeImageSauvolaFallback` terhadap tag v0.3.0 dan HEAD branch. Cross-product hanyalah satu multiply-subtract-compare, jadi budget regresi-nya sama dengan v0.3 (dalam ~1% dari baseline).
- [ ] Opsional menambah `BenchmarkDecodeImageRotated90` yang menjalankan fixture rotation lewat pipeline penuh untuk mempublikasikan biaya rotasi axis-aligned.
- [ ] `go test -race ./...` tetap bersih.

### R6 — Polish & Rilis `(S)`

Goal: memotong `v0.4.0`.

- [ ] Tidak ada perubahan API publik; tidak ada yang perlu ditambahkan ke tabel ringkasan API.
- [ ] Tag `v0.4.0` setelah push pertama ke GitHub supaya tag mendarat pada commit yang dilihat remote: `git tag -a v0.4.0 -F -` dengan baris subjek `QR rotation handling release` diikuti paragraf yang diturunkan dari CHANGELOG. Dikerjakan manual oleh user.

---

## 5. Usulan Delta Layout File

Rotation handling mendarat sebagai patch minimal pada image stage yang ada; tidak ada direktori package baru, hanya satu file test baru dan doc theory + plan.

```
qrgen/
├── decode_image.go              # eksisting — hanya orderFinderTriple yang disentuh
├── decode_image_test.go         # eksisting — mendapat unit test ordering rotation-invariant
├── decode_rotation_test.go      # baru — fixture rotation sintetis + helper rotateImage
└── decode_bench_test.go         # eksisting — opsional BenchmarkDecodeImageRotated90
docs/
├── plan-decoder-rotation.md     # versi Inggris
├── plan-decoder-rotation.id.md  # file ini
└── theory/
    ├── 15-rotation-handling.md
    └── 15-rotation-handling.id.md
```

## 6. Risiko & Catatan Teknis

- **Scanner 1:1:3:1:1 pada sudut oblique.** Row scan menabrak modul finder yang ter-rotasi pada sudut selain 0 atau 90 derajat, sehingga lebar modul di proyeksi garis-scan berbeda dari ukuran modul sebenarnya. Toleransi ±50% yang ada di `fitsFinderRatio` menyerap tilt hingga sekitar 30 derajat dengan nyaman; di luar itu rasio mulai melenceng keluar dari band toleransi. Cakupan v0.4 sengaja berhenti di boundary toleransi; coverage yang lebih luas membutuhkan finder detector berbasis contour atau pencarian kipas orientasi.
- **Rotasi bilinear vs nearest-neighbour di fixture.** Merotasi image sumber dengan sampling bilinear menghasilkan beberapa pixel grey perantara di sepanjang tepi yang tidak ada di output encoder. Otsu fast path tetap membinarisasi mereka dengan benar karena mode ink dan paper tetap terpisah baik, tapi sudut ekstrem menghasilkan tepi yang lebih berisik dan dapat menantang rasio 1:1:3:1:1 yang ketat. Fixture tetap di dalam band aman by design.
- **Interaksi dengan dispatch Sauvola v0.3.** Perubahan rotasi orthogonal terhadap dispatch binariser. Kita berharap `binariserOtsu` menyala untuk PNG rotated yang bersih dan jalur Sauvola menyala untuk input rotated-dan-shadowed. Fixture R3 meng-assert cabang Otsu pada rotasi bersih; follow-up setelah v0.4 dapat opsional menguji kombinasinya silang.
- **Estimasi module-pitch pada tilt ekstrem.** `crossCheckVertical` merata-rata estimasi ukuran modul horizontal dan vertikal. Untuk tilt 30 derajat kedua estimasi ini berbeda sekitar 15%; perataan-nya sedikit membiaskan homography. Dapat diterima di dalam cakupan v0.4.
- **Simbol mirror.** Check cross-product mengembalikan sign yang "salah" untuk QR mirror. QR code nyata tidak pernah mirror, jadi kita memperlakukan kasus mirror sebagai kegagalan deteksi eksplisit alih-alih membalik label otomatis — itu menjaga failure mode tetap kentara alih-alih mendecode input mirror sintetis seolah-olah valid.

## 7. Referensi

- ISO/IEC 18004:2015 — §11.2 (Locator pattern detection) dan §11.3 (Image sampling). Referensi spec untuk asumsi bahwa simbol "kira-kira right-side-up".
- Hartley & Zisserman — *Multiple View Geometry in Computer Vision*, ed. 2, §4 (homography estimation). Mengkonfirmasi bahwa transform projektif 3x3 menyerap rotasi, skala, translasi, dan perspektif diberikan korespondensi corner yang benar.
- Proyek ZXing — *open-source decoder reference*: <https://github.com/zxing/zxing>, terutama method `FinderPatternFinder.orderBestPatterns`, yang memakai trik handedness cross-product yang sama yang kita adopsi di sini.
- Project Nayuki — *QR Code generator library, decoder companion notes*.

## 8. Pertanyaan Terbuka

Untuk dijawab sebelum milestone yang bersangkutan dimulai:

- **Daftar sudut fixture rotation.** Sketsa R3 mendaftar 90 / 180 / 270 / 15 / 30 plus negative 45. Layak menambah 60 dan 75 untuk mendokumentasikan di mana band soft-tilt berakhir, atau biarkan ke v0.5?
- **Error untuk deteksi mirror.** Sekarang input mirror gagal sebagai `ErrFinderNotFound`. Layak menambah sentinel `ErrMirroredSymbol` khusus, atau diam saja dan perlakukan sebagai kegagalan "tidak ditemukan" yang sudah ada? Default: diam saja, jaga API publik tetap stabil.
- **Rotation sebagai input publik.** Apakah caller harus bisa memberi hint sudut rotasi untuk melewati cross-product check pada device yang sudah tahu orientasi? Tidak di v0.4; tinjau lagi kalau caller nyata meminta.
