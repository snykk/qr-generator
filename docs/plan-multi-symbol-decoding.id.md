# Rencana Multi-Symbol Decoding QR

Dokumen ini menjelaskan rencana implementasi **multi-symbol decoding** untuk rilis minor `v0.14.0`: membaca setiap simbol QR yang ada dalam satu gambar, bukan hanya yang pertama. Ini dibangun langsung di atas detektor finder connected-component v0.13, yang sudah meng-enumerasi setiap pola finder di gambar; milestone ini menggeneralisasi seleksi single-symbol menjadi grouping disjoint dan menambah permukaan publik kecil (`DecodeAll`).

> Status: **draf / dokumen hidup.** Milestone MS1..MS6 mendarat bertahap di branch `multi-symbol-decoding`; tiap milestone adalah commit fokus (atau deret kecil) berikut test, mengikuti irama milestone sebelumnya.

> Versi bahasa Inggris: [docs/plan-multi-symbol-decoding.md](plan-multi-symbol-decoding.md).

---

## 1. Visi & Tujuan

- Men-decode **semua** simbol QR dalam satu gambar — satu halaman berisi banyak kode, satu lembar tiket, satu set structured-append yang difoto bersama — lewat satu panggilan, tanpa pemanggil harus meng-crop tiap simbol lebih dulu.
- Pakai ulang, jangan bangun ulang. Detektor CC v0.13 (`finderNestings`) sudah menemukan setiap cincin finder di gambar; jalur single-symbol hanya menciutkannya jadi satu triple terbaik di `bestFinderTripleIdx`. Milestone ini menggeneralisasi seleksi tunggal itu menjadi "semua triple valid yang disjoint" dan men-decode masing-masing lewat pipeline geometri per-simbol yang sudah ada, tanpa perubahan.
- Jaga API yang ada tetap **byte-for-byte**. `Decode`, `DecodeBytes`, `DecodeMatrix` tetap single-symbol. `DecodeAll` / `DecodeAllBytes` murni tambahan.
- Jaga **zero runtime dependency**. Grouping murni Go atas daftar kandidat yang ada; fixture test multi-symbol dikomposit in-tree dari QR yang di-render, di-cross-check terhadap multi reader `gozxing` yang sudah jadi test dependency.
- **Jujur soal batas.** Ini membaca yang bisa dibaca dari sebuah scene berisi kode-kode independen; simbol yang rusak, ter-crop, atau bertumpuk dilewati, bukan dipulihkan, dan perilaku terdokumentasi menyatakannya.

## 2. Pendekatan, berpijak pada kode sekarang

Jalur projektif v0.13: `detectFinderQuads(bm)` meng-enumerasi setiap cincin finder sebagai `finderQuad` (pusat + empat sudut cincin); `findFinderQuadTriple` membatasi ke `maxCCCandidates` lalu `bestFinderTripleIdx` memilih satu triple dengan `finderTripleCost` terendah; `decodeProjectiveFromBitmap` menjalankan trial version (`estimateVersionQuad` -> `candidateVersions` -> `homographyFromFinderQuads` -> `sampleMatrix` -> `DecodeMatrix`) dan mengembalikan teks pertama yang valid secara Reed-Solomon.

Multi-symbol mengubah persis dua hal dan memakai ulang sisanya:

1. **Grouping** menggantikan argmin tunggal. `groupFinderTriples` baru meng-enumerasi triple kandidat (ceiling O(N^3) yang sama seperti `bestFinderTripleIdx`, N <= `maxCCCandidates`), menyimpan yang lolos gerbang validitas yang ada (`orderFinderTriple`: rasio leg <= 1.5, residu hipotenusa <= 0.15) dan sebuah ceiling cost terukur, mengurutkannya terbaik-dulu, dan meng-klaim triple disjoint secara greedy. Sebuah finder di-klaim **hanya ketika triple yang memuatnya benar-benar men-decode** — keberhasilan decode, bukan geometri saja, adalah penentu akhir, dan itulah yang mencegah triple yang plausibel-tapi-palsu mencuri finder milik simbol nyata.

2. **Decode per-simbol difaktorkan keluar.** Body `decodeProjectiveFromBitmap` menjadi `decodeOneQuadTriple(bm, tq) (text string, v Version, hg homography, err error)` — memunculkan version dan homography yang dimenangkan, yang dibutuhkan `Result` publik. Jalur single-symbol (`decodeProjectiveFromBitmap`) menjadi `findFinderQuadTriple` + `decodeOneQuadTriple` dan tetap identik secara perilaku.

Dasarkan jalur multi pada **detektor CC/projektif, bukan line scanner afin**: `finderNestings` sudah menghasilkan semua finder, sedangkan `findFinders` afin menciut ke top-3-by-weight dan akan mencampur finder dua simbol jadi triple sampah. Konsekuensinya — simbol tegak yang blur, yang akan tertangkap jalur afin tetapi cincinnya tak melabel bersih, mungkin terlewat oleh `DecodeAll` — terdokumentasi, bukan diabaikan diam-diam (bagian 6, risiko 6).

## 3. Cakupan

### Termasuk dalam v0.14.0

- `groupFinderTriples` / `allFinderQuadTriples`: grouping greedy disjoint dengan klaim-saat-decode-sukses, pemetaan triple->quad berbasis indeks (bukan lookup float-equality).
- `decodeOneQuadTriple` diekstrak dari `decodeProjectiveFromBitmap`, mengembalikan teks + version + homography.
- Publik `DecodeAll(image.Image) ([]Result, error)` dan `DecodeAllBytes([]byte) ([]Result, error)`; sebuah `Result{Text, Corners [4]image.Point, Finders [3]image.Point, Version}` dengan helper `Bounds()`. Sudut simbol diturunkan dari homography yang dimenangkan, bukan sudut cincin finder.
- Sort reading-order deterministik; kontrak error terdokumentasi (bagian 4).
- Dokumen referensi `docs/theory/25-multi-symbol-decoding.md` (EN + ID) dan pembaruan README/limitations/roadmap.
- Test: helper gambar komposit in-tree, cross-check multi-reader `gozxing`, dan penjaga ekuivalensi single-symbol.

### Belum termasuk (masih)

- **Reassembly structured-append dari satu gambar.** `DecodeAll` mengembalikan teks per-simbol mentah; ia tidak menjahit set SA kembali jadi satu pesan. `DecodeStructuredAppend` (gambar terpisah) tetap jalurnya. `Result` menghilangkan metadata SA untuk sekarang (bisa diperluas nanti tanpa memecahkan pemanggil).
- **Jalur cepat afin multi-symbol.** `DecodeAll` hanya CC/projektif; `findFinders` afin tetap single-symbol.
- **Memulihkan simbol ter-crop.** Ketiga cincin finder sebuah simbol harus sepenuhnya dalam frame; simbol yang keluar tepi dilewati.
- **`DecodeAllMatrix`.** Grid `[][]bool` menurut konstruksi adalah satu simbol; ia hanya akan menduplikasi `DecodeMatrix`.

---

## 4. Keputusan desain (menyelesaikan kontradiksi antar-desain)

Ini mengunci interface yang dibiarkan tak konsisten oleh tiga desain eksplorasi, sesuai kritik kelengkapan.

- **Signature decode per-triple.** `decodeOneQuadTriple(bm *bitmap, tq finderTripleQuad) (text string, v Version, hg homography, err error)`. Baik grouper (butuh sukses/gagal) maupun `DecodeAll` (butuh version + homography untuk `Result`) memakai satu signature ini.
- **`Result.Corners` adalah sudut luar simbol, diturunkan dari homography yang dimenangkan** — bukan sudut cincin finder (`finderQuad.corners`), yang hanya mencakup finder 7x7. Dengan rentang modul `n = v.Size()`, keempat sudut simbol adalah `hg.apply(-0.5,-0.5)`, `hg.apply(n-0.5,-0.5)`, `hg.apply(n-0.5,n-0.5)`, `hg.apply(-0.5,n-0.5)` (tepi luar modul), masing-masing dibulatkan (bukan dipotong) ke `image.Point`. Urutan `[TL, TR, BR, BL]`.
- **Kontrak error.** `DecodeAll` mengembalikan `err != nil` hanya untuk kegagalan seluruh-gambar: `DecodeAllBytes` memunculkan error `image.Decode`, dan **nol decode sukses** — entah dari kurang dari tiga finder atau dari finder yang tak pernah RS-decode — mengembalikan `ErrFinderNotFound`, demi paritas dengan `Decode` dan `errors.Is`. Sukses apa pun mengembalikan `(results, nil)`; region gagal cukup absen. Tak pernah slice parsial berikut error non-nil.
- **Jalur single-symbol tak berubah.** `Decode` tetap memanggil `findFinderQuadTriple` (argmin global), BUKAN `allFinderQuadTriples`-lalu-pertama, karena greedy-di-bawah-threshold bisa memilih triple berbeda dari argmin pada gambar single-symbol yang noisy. Sebuah test meng-assert `DecodeAll` pada gambar satu-simbol mengembalikan tepat satu `Result` yang `Text`-nya sama dengan keluaran `Decode`.
- **Pemetaan berbasis indeks.** Grouper membawa indeks tiap kandidat lewat triple; quad dipulihkan lewat indeks, bukan float-equality `finderCandidate`, menghapus footgun itu.
- **Reading order.** Urutan total deterministik: kelompokkan simbol yang `Corners[0].Y`-nya jatuh dalam toleransi (fraksi dari tinggi simbol median) menjadi baris, urutkan baris atas-ke-bawah dan simbol kiri-ke-kanan lewat `Corners[0].X`, dengan tie-break akhir `(Y, X, indeks-stabil)` eksak agar keluaran reprodusibel. Terdokumentasi sebagai best-effort untuk layout grid/dokumen.

## 5. Milestone

Milestone mendarat berurutan. **Checkpoint A** (setelah MS5) men-decode gambar multi-symbol yang dikomposit lewat `DecodeAll` publik, sepakat dengan multi reader `gozxing`. **Checkpoint B** (MS6) adalah rilis `v0.14.0`.

### MS1 — Plan Doc `(S)`

- [x] `docs/plan-multi-symbol-decoding.md` dan padanan Indonesianya: visi, pendekatan berpijak pada kode, cakupan, keputusan desain, milestone MS1..MS6, delta tata letak file, risiko, referensi, pertanyaan terbuka.

### MS2 — Dokumen Referensi `(S)`

- [ ] `docs/theory/25-multi-symbol-decoding.md` (EN + ID): kenapa detektor CC sudah meng-enumerasi semua finder; masalah grouping (triple valid disjoint dengan pitch modul bersama dan geometri right-isosceles); kenapa keberhasilan decode harus jadi penentu klaim; pakai ulang geometri per-simbol; penurunan sudut simbol dari homography; semantik reading-order dan error; serta batas yang jujur. Plus entri README theory 25 dan baris pemetaan kode.

### MS3 — Grouping + Refactor Per-Triple `(L)`

Tujuan: mesin grouping dan decode per-simbol yang bisa dipakai ulang, diuji pada set kandidat yang dikonstruksi.

- [ ] Ekstrak `decodeOneQuadTriple(bm, tq) (string, Version, homography, error)`; ekspresikan ulang `decodeProjectiveFromBitmap` lewatnya tanpa perubahan perilaku.
- [ ] `groupFinderTriples(quads) [][3]int`: enumerasi, gerbang oleh `orderFinderTriple` + ceiling cost **terukur**, urutkan terbaik-dulu, klaim disjoint greedy (klaim-saat-decode-sukses terjadi di wrapper). Berbasis indeks.
- [ ] `allFinderQuadTriples(bm) ([]finderTripleQuad, error)` dan wrapper decode yang me-loop `decodeOneQuadTriple`, meng-klaim finder saat sukses, dibatasi cap decode-attempt **berskala-N** (bukan 32 tetap — grid same-size menghasilkan banyak triple cross-symbol yang lolos geometri; cap tetap akan diam-diam men-drop simbol nyata).
- [ ] **Kalibrasi toleransi dari data:** ukur `finderTripleCost` lintas korpus `decode_tilt_test.go` / `decode_rotation_test.go` yang ada dan set ceiling cost dengan margin di atas simbol terburuk yang masih bisa didekode sekarang, agar tak ada decode v0.13 yang regresi. Pertimbangkan window version per-triple yang dipersempit untuk jalur multi (window kedekatan sekitar `estimateVersionQuad`) untuk membatasi fan-out trial version; jaga trade-off tetap terdokumentasi.
- [ ] Unit test pada set kandidat buatan-tangan: dua triple disjoint terkelompok; finder bersama/palsu tak di-klaim ganda; triple sampah cross-symbol berpitch sama ditolak oleh kegagalan decode.

### MS4 — API Publik `(M)`

Tujuan: permukaan multi-symbol yang menghadap pengguna.

- [ ] Struct `Result` + `Bounds()`; penurunan sudut simbol dari homography yang dimenangkan (dibulatkan); `Finders` dari pusat triple terurut; `Version`.
- [ ] `DecodeAll(img) ([]Result, error)` dan `DecodeAllBytes([]byte) ([]Result, error)`: binarisasi lewat dispatch Otsu/Sauvola yang ada, `allFinderQuadTriples` -> per-triple `decodeOneQuadTriple`, kumpulkan sukses, terapkan sort reading-order, petakan nol hasil ke `ErrFinderNotFound`.

### MS5 — Test Round-Trip Multi-Symbol `(M)`

Tujuan: men-decode scene komposit nyata end-to-end.

- [ ] Helper komposit in-tree: `draw.Draw` beberapa keluaran `Encode(...)` ke satu kanvas pada offset pilihan (pakai ulang `warpTilt` untuk tile miring). Tanpa fixture biner.
- [ ] Kasus: (a) 2+ tegak same-size, (b) beda-ukuran (pitch per-simbol bertahan), (c) dua tilt berbeda, (d) nol simbol -> `ErrFinderNotFound`, (e) tepat satu -> sama dengan `Decode`, (f) berdampingan/bertumpuk (parsial yang jujur), (g) grid same-size yang menekan cap attempt.
- [ ] Cross-check `gozxing` `multi/qrcode.QRCodeMultiReader` pada satu komposit (oracle terkuat yang tersedia; sudah jadi test dependency), meng-assert set payload yang sama.

### Checkpoint A — DecodeAll publik membaca gambar multi-symbol komposit dan sepakat dengan gozxing.

### MS6 — Polish & Rilis `(S)`

Tujuan: memotong `v0.14.0`.

- [ ] README: subseksi decoding tentang `DecodeAll` dan `Result`; Limitations dan Roadmap diperbarui (multi-symbol selesai; catat asimetri CC-only dan batas crop/tumpuk); Scope diperbarui; contoh runnable `examples/decode/multi`.
- [ ] Entri `v0.14.0` di `CHANGELOG.md` plus anchor, dibiarkan **unstaged** untuk release branch (sesuai konvensi: branch fitur tidak mendefinisikan tag).
- [ ] `go test -race ./...` bersih, gofmt bersih.

---

## 6. Risiko & Catatan Teknis

- **Kalibrasi cost-gate adalah risiko teknis tertinggi.** Jalur single-symbol mengambil argmin global tanpa threshold absolut, jadi tak ada bukti bahwa simbol nyata (terutama miring) skornya di bawah cost tetap mana pun. `finderTripleCost` menjumlah residu jarak-kuadrat yang membengkak di bawah foreshortening. Mitigasi: `orderFinderTriple` tetap gerbang validitas utama (sudah terkalibrasi dan sudah dipakai), dan ceiling cost tambahan apa pun diset dari pengukuran korpus yang ada dengan margin — bukan angka bulat — agar tak ada decode v0.13 yang regresi.
- **Cap attempt vs grid same-size.** Dalam grid teratur berisi kode same-size, triple cross-symbol (satu finder dari A, dua dari B) bersifat right-isosceles dan berbagi pitch, jadi lolos geometri dan hanya ditolak saat RS. Cap attempt tetap bisa habis sebelum simbol nyata tercapai dan men-drop-nya. Mitigasi: skalakan cap dengan N, dan andalkan klaim-saat-sukses plus urutan cost-menaik agar triple nyata termurah dicoba lebih dulu.
- **Fan-out trial version adalah biaya sebenarnya.** Tiap `decodeOneQuadTriple` bisa mencoba sampai 40 version (`sampleMatrix` + RS masing-masing), jadi worst case jujurnya `attemptCap x 40` decode matriks, mengerdilkan aritmetika cost O(N^3). Mitigasi: version terurut-kedekatan mencoba yang benar lebih awal; pertimbangkan window yang dipersempit untuk jalur multi dan dokumentasikan batasnya.
- **Asimetri CC-only.** `DecodeAll` hanya memakai detektor CC/projektif, jadi simbol tegak yang blur/low-res yang didekode line scan afin tetapi cincinnya tak melabel bersih mungkin ditemukan `Decode` tetapi terlewat `DecodeAll`. Terdokumentasi sebagai batas yang diketahui; jalur afin tidak digeneralisasi ke multi.
- **Simbol ter-crop, bertumpuk, dan beda-tilt.** Ketiga cincin finder harus sepenuhnya dalam frame (cincin yang menyentuh tepi tak melabel jadi nesting, jadi simbol itu di-drop, tanpa crash — `sampleMatrix` menjaga out-of-bounds). Simbol berdampingan dengan quiet zone menyatu bisa kehilangan finder tepi. Tilt berbeda masing-masing dapat homography sendiri, yang sudah ditangani jalur per-triple, tetapi kencangkan kalibrasi cost-gate agar meloloskan simbol frontal dan curam sekaligus dalam satu gambar.
- **Truncation `maxCCCandidates = 64` berdasar-modul-terbesar** bisa men-drop finder simbol kecil dalam scene yang mencampur satu kode besar dan banyak kecil. Tandai sebagai tepi dari kasus beda-ukuran; tinjau ulang cap kalau menggigit.

## 7. Usulan Delta Tata Letak File

```
qrgen/
├── decode_finder_cc.go        # + groupFinderTriples, allFinderQuadTriples (berbasis indeks)
├── decode_image.go            # ekstrak decodeOneQuadTriple; dispatch binarisasi dipakai ulang
├── decode_multi.go            # baru — Result, Bounds, DecodeAll, DecodeAllBytes, reading order
├── decode_multi_test.go       # baru — helper komposit, kasus (a)-(g), ekuivalensi single-symbol
├── roundtrip_multi_test.go    # baru — cross-check gozxing multi/qrcode
docs/
├── plan-multi-symbol-decoding.md      # file ini
├── plan-multi-symbol-decoding.id.md   # padanan Indonesia
└── theory/
    ├── 25-multi-symbol-decoding.md     # baru
    └── 25-multi-symbol-decoding.id.md  # baru
examples/decode/multi/main.go          # baru — demo runnable
```

## 8. Referensi

- ISO/IEC 18004:2015 — struktur pola finder dan quiet zone yang diandalkan detektor.
- `docs/theory/24-arbitrary-angle-detection.md` — detektor finder CC dan pipeline projektif per-simbol yang dipakai ulang milestone ini.
- `docs/theory/12-image-processing.md`, `15-rotation-handling.md` — binarisasi dan pengurutan finder yang invarian rotasi (`orderFinderTriple`) yang dipakai ulang sebagai gerbang validitas grouping.
- ZXing `MultiDetector` / `MultiFinderPatternFinder.selectMultipleBestPatterns` — prior art untuk mengelompokkan finder jadi banyak simbol; `multi/qrcode.QRCodeMultiReader` milik `gozxing` (sudah jadi test dependency) adalah oracle cross-check.

## 9. Pertanyaan Terbuka

Dijawab sebelum milestone terkait:

- **Nilai ceiling cost.** Diselesaikan di MS3: pengukuran korpus tilt/rotasi menunjukkan simbol yang benar-benar bisa didekode mencapai `finderTripleCost` sekitar 0.76 pada 55 derajat tanpa pemisahan bersih dari noise, jadi **tak ada ceiling tetap**. `orderFinderTriple` adalah gerbang validitas, triple diurut menurut cost (simbol nyata dekat nol lebih dulu), dan cap attempt membatasi sisanya — yang juga tak bisa meregresikan jalur single-symbol, karena ia pun tak punya ceiling.
- **Window version untuk jalur multi.** Diselesaikan di MS3: decode per-triple (`decodeOneQuadTriple`) dipakai bersama jalur single-symbol dan mempertahankan sweep version terurut-kedekatan penuh, jadi perilaku single-symbol identik; jalur multi membatasi biaya lewat cap attempt, bukan window yang lebih sempit. Ditinjau ulang hanya bila kasus grid MS5 terbukti terlalu lambat.
- **Kebijakan cap-attempt.** Diselesaikan di MS3: `4*N + 16` percobaan decode (N = jumlah finder), dengan triple ter-klaim/duplikat dilewati gratis. Longgar relatif terhadap paling banyak `N/3` simbol nyata; divalidasi terhadap kasus stress grid MS5 (kasus g) dan dikencangkan di sana bila perlu.
- **Metadata Result.** Kirim `Result` ramping (Text, Corners, Finders, Version); tambah EC level / mask / header SA nanti sesuai kebutuhan, tanpa memecahkan pemanggil.
