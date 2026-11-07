# QR Generator — Rencana Deteksi Micro QR

Dokumen ini menjelaskan rencana implementasi untuk **menemukan simbol Micro QR Code di dalam gambar**, untuk rilis minor `v0.16.0`. v0.15 menghadirkan encoder Micro dan decoder tingkat matriks, sehingga grid Micro yang bersih sudah bisa di-decode lewat `DecodeMatrix`; tetapi `Decode`, `DecodeBytes`, dan `DecodeAll` masih membutuhkan tiga finder pattern milik simbol QR, sehingga ketiganya tidak bisa menemukan simbol Micro di dalam foto. Milestone ini menutup celah itu: satu finder pattern, dibaca bersama timing pattern dan format information, sudah cukup untuk menemukan simbol Micro, menentukan orientasinya, dan menentukan ukurannya, lalu decoder matriks yang sudah ada mengambil alih.

> Status: **draf / dokumen hidup.** Milestone MD1..MD7 mendarat bertahap di branch `micro-qr-detection`; tiap milestone adalah commit fokus (atau deret kecil) berikut test, mengikuti irama milestone sebelumnya.

> Versi bahasa Inggris: [docs/plan-micro-qr-detection.md](plan-micro-qr-detection.md).

---

## 1. Visi & Tujuan

- `Decode` dan `DecodeBytes`, dan lewat keduanya `-decode` milik CLI, membaca simbol Micro QR dari gambar tanpa fungsi atau option baru, sama seperti `DecodeMatrix` yang sudah mengenali grid Micro dari ukurannya.
- `DecodeAll` melaporkan simbol Micro bersama simbol QR yang ditemukan di gambar yang sama.
- Membiarkan decoding QR persis seperti sekarang. Pencarian Micro baru berjalan setelah percobaan QR pada gambar hasil binarisasi yang sama gagal, sehingga gambar yang hari ini ter-decode sebagai QR tidak pernah menyentuh kode baru.
- Memakai ulang pipeline gambar, bukan membangunnya lagi: binarisasi, detektor finder connected-component, ekstraksi sudut, solver homografi, sampling grid, dan decoder matriks v0.15 semuanya terbawa.
- Memvalidasi terhadap implementasi independen: simbol zint yang sudah tersimpan sebagai teks di `qrgen/testdata/thirdparty/micro_symbols.txt`, dirender menjadi gambar oleh test, dan reader zxing-cpp yang dijalankan pada scene sintetis yang sama.
- Menjaga **zero runtime dependency** dan tanpa fixture biner, serta melaporkan rentang rotasi, tilt, dan skala yang memang berhasil di-decode.

## 2. Apa yang diberikan satu finder pattern, dan apa yang tidak

Pipeline QR dibangun di sekitar tiga finder. Jalur afin (`findFinders`) maupun jalur projektif (`findFinderQuadTriple`) menolak gambar yang finder-nya kurang dari tiga, dan geometrinya berasal dari segitiga yang dibentuk ketiga pusat finder. Simbol Micro hanya punya satu finder, jadi kedua jalur itu tidak bisa memulai.

Satu finder tetap membawa banyak informasi. Empat sudut luarnya, yang sudah diekstrak oleh detektor connected-component, adalah empat pasangan titik: pas cukup untuk homografi projektif dari koordinat modul ke piksel. Yang tidak bisa mereka katakan adalah sudut mana yang mana. Finder tampak sama setelah diputar seperempat putaran, jadi keempat sudut itu cocok dengan persegi 7 kali 7 dalam empat rotasi, dan homografinya hanya diketahui sampai pilihan itu. Bagian lain simbol yang menjawabnya:

- **Orientasi.** Timing pattern berjalan di baris 0 dan kolom 0, mulai tepat setelah separator, gelap pada indeks genap, sehingga modul (baris 0, kolom 8) dan (baris 8, kolom 0) gelap di setiap versi Micro. Di masing-masing dari tiga rotasi yang salah, setidaknya satu dari keduanya jatuh di quiet zone. Mengenali dua sisi finder mana yang menghadap quiet zone sama dengan mengenali orientasinya.
- **Format information.** Ke-15 modul format terletak di blok 9 kali 9 pada sudut finder (kolom 8 baris 1..8 dan baris 8 kolom 1..7), jadi bisa dibaca lewat homografi finder sebelum ukuran simbol diketahui. Setelah di-decode terhadap ke-32 format word Micro, modul-modul itu memberi nomor simbol, dan dari situ version, level error correction, dan mask.
- **Cek silang version.** Timing pattern di baris 0 berselang-seling sampai kolom n-1 lalu diikuti quiet zone, sehingga panjangnya secara independen mengonfirmasi ukuran yang diklaim format information.
- **Geometri.** Finder membentang 7 modul; simbol M4 membentang 17. Menyampel seluruh simbol lewat homografi finder berarti mengekstrapolasinya sekitar dua setengah kali melewati titik-titik yang menentukannya, sehingga kesalahan apa pun di sudut-sudut itu membesar ke arah luar. Apakah itu berpengaruh pada sudut kemiringan yang kita pedulikan adalah soal pengukuran, bukan asumsi (bagian 5).

## 3. Prior art: zxing-cpp

zxing-cpp, yang sudah menjadi decoder Micro independen proyek ini, menemukan simbol Micro dengan garis besar yang sama. `SampleMQR` miliknya (core/src/qrcode/QRDetector.cpp, master yang dibaca pada November 2026) mencocokkan homografi ke quad sudut dari satu finder di masing-masing empat rotasi, mensyaratkan dua modul timing terdalam (kolom 8 di baris 0, dan baris 8 di kolom 0) gelap, membaca 15 bit format dan men-decode-nya terhadap ke-32 word Micro dalam dua urutan bit (yang kedua untuk simbol yang di-mirror), lalu menyimpan rotasi dengan jarak Hamming terkecil. Version diambil dari nomor simbol. Cek quiet zone kemudian menolak kandidat kalau lebih dari 2n/3 dari 2n sampel di sepanjang baris n dan kolom n gelap; dengan cara itulah ia menghindari membaca sudut simbol QR biasa sebagai simbol Micro. Terakhir, ia mencari ulang sudut kanan-atas, kiri-bawah, dan kanan-bawah lewat pencarian grid lokal sebelum menyampel. Reader-nya menjalankan semua ini pada setiap finder pattern yang tidak dipakai oleh simbol QR yang berhasil di-decode, setelah tahap QR.

Dua detail desain itu penting bagi desain kita. Pertama, zxing-cpp tidak memercayai homografi finder untuk sudut-sudut jauh: ia mencarinya lagi. Kedua, quad sudutnya berada di tengah cincin luar finder (pusat modul-modul sudut), hasil perpaduan fit terhadap kedua tepi cincin; quad kita berasal dari piksel-piksel ekstrem cincin luar.

## 4. Pendekatan, berpijak pada kode

Pembacaan pipeline gambar (decode_image.go, decode_finder_cc.go, decode_multi.go, decode_image_sauvola.go) dan decoder Micro (micro_decode.go, micro_matrix.go) membagi pekerjaan menjadi tiga kelompok.

- **Bisa dipakai apa adanya:** binariser Otsu dan Sauvola beserta dispatch-nya; `labelComponents`, `finderNestings`, dan `extractQuadCorners`; `computeHomography` dan `computeHomographyLS`; `sampleMatrix`, yang sudah menerima ukuran grid sebagai parameter; `decodeMicroMatrix`, yang membaca ulang format information untuk ukuran hasil sampel, menjalankan Reed-Solomon (hanya deteksi di M1), dan mem-parsing segmen Micro; `microFormatInfo`; serta claim loop dan `Result` milik `DecodeAll`.
- **Khusus Micro, baru:** accessor untuk setiap finder quad, karena `detectFinderQuads` menolak kalau jumlahnya kurang dari tiga; menyusun sudut-sudut sebuah quad dalam winding (arah urutan sudut) yang konsisten, karena `extractQuadCorners` mengembalikannya dalam urutan sembarang; homografi per rotasi dari satu finder; gerbang orientasi; reader format sisi gambar yang mencari di ke-32 word; cek silang timing; penyempurnaan sudut jauh kalau pengukuran memintanya; tahap Micro di dalam percobaan connected-component dan di dalam loop `DecodeAll`; serta satu field di `Result` untuk version Micro.
- **Asumsi tersembunyi yang harus dihindari:** batas minimum tiga nesting di `detectFinderQuads`; `homographyFromFinderQuads` dan `estimateVersionQuad`, yang menerima atau menghasilkan version QR; `Result.Version`, yang khusus QR dan harus tetap begitu (v0.15 sengaja menjadikan `MicroVersion` tipe tersendiri); `decodeImage` yang selalu mengembalikan error jalur afin; `decodeImageWhich` yang hanya mengenal dua jalur; `readMicroFormatInfo` yang butuh version di awal, padahal justru itu yang belum diketahui sisi gambar; serta piksel sudut yang merupakan pusat piksel, bukan tepi luar cincin, yaitu bias setengah piksel yang bisa diabaikan pada 8 piksel per modul tetapi besar pada 2.

## 5. Keputusan desain

- **Transparan, tanpa API single-symbol baru.** `Decode` dan `DecodeBytes` mendapat dukungan Micro di tempat; dokumentasinya serta catatan di `DecodeMatrix` tentang gambar diperbarui.
- **QR dulu, di setiap binarisasi.** Pipeline QR afin berjalan pertama, tidak berubah, dengan dispatch Otsu-lalu-Sauvola miliknya sendiri. Percobaan connected-component lalu me-label setiap binarisasi sekali (Otsu, lalu Sauvola bila perlu) dan memakai finder quad-nya dua kali: triple QR terbaik lebih dulu, dan hanya kalau itu gagal, setiap quad dicoba sebagai finder Micro, dari yang terbesar, dan yang pertama berhasil di-decode yang menang. Labelling dipakai bersama, gambar Micro yang bersih sudah ditemukan di binarisasi pertama, dan tidak ada kandidat QR di sebuah bitmap yang pernah didahului kandidat Micro di bitmap yang sama.
- **Orientasi.** Keempat sudut disusun dalam winding simbol yang tidak di-mirror lalu dicoba dalam empat rotasinya. Sebuah rotasi lolos gerbang hanya kalau modul timing terdalam (baris 0 kolom 8, baris 8 kolom 0) gelap: dokumen 27 menunjukkan bahwa di setiap rotasi yang salah setidaknya satu dari keduanya jatuh di quiet zone, sedangkan modul separator bisa lolos di sebagian rotasi yang salah sehingga diserahkan ke cek silang timing. Di antara rotasi yang lolos, yang format word-nya paling dekat ke word valid yang menang. Simbol yang di-mirror di luar cakupan, sama seperti untuk QR.
- **Format dan version.** Ke-15 bit di-decode terhadap ke-32 word Micro dengan budget 3 bit yang sama seperti `readMicroFormatInfo`; nomor simbol memberi version dan ukuran grid n. Bagian 7 dokumen 26 sudah mengingatkan bahwa strip yang seluruhnya terang masih dalam budget dari M4-L mask 1, dan word 15 bit acak punya peluang lebih dari separuh untuk berada dalam budget dari salah satu word Micro; itulah sebabnya format word saja tidak pernah cukup untuk menerima kandidat.
- **Cek silang timing, bukan hitungan quiet zone.** Untuk ukuran yang diklaim, baris 0 dan kolom 0 dari indeks 7 sampai n harus terbaca: separator terang, timing yang berselang-seling dari gelap di 8 sampai gelap di n-1, lalu quiet zone terang di n, dengan toleransi kecil yang ditetapkan lewat pengukuran di MD3. Ini menolak sudut simbol QR (yang baris 0-nya di luar finder berisi data, bukan timing) dan ukuran yang salah, memakai struktur yang memang diletakkan standar di sana untuk keperluan itu. Hitungan quiet zone 2n/3 milik zxing-cpp disimpan sebagai cadangan kalau pengukuran menunjukkan perlu.
- **Geometri, diputuskan lewat pengukuran.** Versi pertama menyampel lewat homografi finder pada ukuran n. MD5 mengukur di mana itu gagal di bawah rotasi, tilt, dan ukuran modul kecil, dan baru setelah itu menambahkan yang diperlukan: koreksi sudut setengah piksel, dan mencari ulang sudut kanan-atas dan kiri-bawah dari modul terakhir masing-masing timing pattern (indeks n-1 genap, jadi modul itu selalu gelap, dan quiet zone membatasinya di dua sisi), lalu fit ulang dengan `computeHomographyLS` atas sudut-sudut finder plus kedua sudut itu. Aturan mask Micro, yang mengutamakan modul gelap di baris dan kolom terakhir, membuat sudut kanan-bawah juga bisa ditemukan, kalau pengukuran memintanya.
- **Reed-Solomon sebagai penentu akhir.** Kandidat hanya diterima kalau `decodeMicroMatrix` berhasil pada grid hasil sampel. M1 adalah kasus terlemah: dua EC codeword-nya yang hanya mendeteksi meloloskan sekitar 1 dari 65.536 word acak, dan itulah sebabnya gerbang-gerbang sebelumnya penting.
- **Kontrak error.** Saat gagal total, `Decode` tetap mengembalikan error dari percobaan afin, sehingga setiap error yang ada tidak berubah, dengan satu pengecualian: ketika percobaan afin gagal karena tidak ada triple finder (`ErrFinderNotFound`) dan tahap Micro menemukan finder yang meyakinkan sebagai bagian dari simbol Micro (timing pattern yang cocok persis dengan salah satu ukuran Micro sementara format information-nya tak terbaca, atau semua gerbang lolos dan hanya datanya yang gagal), error Micro yang dikembalikan (`ErrFormatUnreadable`, `ErrTooManyErrors`, atau `ErrCorruptedPayload`), karena simbol Micro yang rusak bukanlah finder yang hilang. Lolos gerbang orientasi saja tidak cukup: MD4 mengukur bahwa sekitar sepertiga finder QR lolos gerbang itu.
- **`DecodeAll`.** Setelah claim loop QR, setiap finder quad yang belum diklaim dicoba sebagai finder Micro dan diklaim kalau berhasil. `Result` mendapat `MicroVersion MicroVersion`, nol untuk simbol QR; untuk simbol Micro `Version` bernilai nol, `Finders[0]` adalah pusat finder dan dua entri lainnya nol, dan `Corners` berasal dari homografi pada ukuran n, seperti untuk QR. Perubahan ini aditif dan membiarkan hasil QR tidak berubah (pertanyaan terbuka 1).
- **Di luar cakupan:** simbol yang di-mirror, jalur line-scan (afin) untuk Micro, jalur cepat untuk gambar murni, decoding simbol Micro yang format information-nya tak terbaca, Micro di `DecodeStructuredAppend` (Micro tidak punya structured append), dan rMQR.

## 6. Milestone

Milestone mendarat berurutan. **Checkpoint A** (setelah MD5) membaca simbol Micro dari gambar lewat `Decode` dan `DecodeBytes`, divalidasi terhadap simbol zint dan reader zxing-cpp. **Checkpoint B** (MD7) adalah rilis `v0.16.0`.

### MD1 — Plan Doc `(S)`

- [x] Dokumen ini dan padanan bahasa Inggrisnya.

### MD2 — Dokumen Referensi `(S)`

- [x] `docs/theory/27-micro-qr-detection.md` (EN + ID): mengapa pipeline QR tidak bisa memulai dari satu finder; apa yang ditentukan empat sudut finder dan ambiguitas seperempat putaran yang tersisa; orientasi dari modul separator dan timing; membaca format information sebelum ukuran diketahui, dan mengapa format word saja adalah bukti yang lemah; cek silang version; ekstrapolasi dari finder 7 modul dan bagaimana sudut-sudut jauh bisa dicari ulang; false positive dan gerbang yang menghentikannya; di mana pencarian ini masuk ke pipeline; prior art. Plus entri README theory dan baris pemetaan kode.

### MD3 — Geometri Satu Finder `(L)`

- [x] `allFinderQuads`: setiap finder quad, tanpa batas minimum; `detectFinderQuads` mempertahankan kontraknya di atas fungsi itu.
- [x] Normalisasi winding dan homografi per rotasi dari satu finder quad.
- [x] Gerbang orientasi; pembacaan format sisi gambar atas ke-32 word; version dan cek silang timing, dengan toleransi yang diukur.
- [x] `decodeMicroQuad(bm, quad)`: semua langkah di atas, lalu `sampleMatrix` pada ukuran n dan `decodeMicroMatrix`, mengembalikan teks, version, dan homografi.
- [x] Test pada bitmap yang dilukis di empat rotasi kelipatan 90 derajat, dan pada setiap simbol zint yang dirender tegak.

### MD4 — Integrasi Pipeline `(M)`

- [x] Tahap Micro di percobaan connected-component, setelah triple QR, per binarisasi; `decodeImageWhich` melaporkan `"micro"`.
- [x] Kontrak error di bagian 5, serta doc comment `Decode` dan `DecodeMatrix`.
- [x] Koreksi sudut setengah piksel, dimajukan dari MD5: dirender pada 3 piksel per modul, setiap simbol M4 dari zint gagal di cek silang timing tanpa koreksi itu.
- [x] Test: `Decode` dan `DecodeBytes` membaca setiap simbol zint dan PNG kita sendiri untuk kedelapan tipe; `-decode` milik CLI membaca PNG Micro; finder pattern tunggal, gambar kosong, dan simbol QR yang rusak tidak menghasilkan teks Micro; fixture QR yang ada tetap ter-decode lewat jalur yang sama seperti sebelumnya.

### MD5 — Pengukuran Ketahanan `(M)`

- [ ] Sweep informasional atas rotasi di bidang gambar, tilt perspektif (`warpTilt`), ukuran modul, dan lebar quiet zone, masing-masing mencatat tahap mana yang gagal.
- [ ] Tambahkan penyempurnaan sudut jauh hanya di tempat sweep menunjukkan ia memperlebar rentang (koreksi setengah piksel sudah masuk di MD4); assert rentang yang terbukti.
- [ ] Cek sekali jalan dengan zxing-cpp pada scene yang sama, dilaporkan di commit: di mana ia membaca scene yang tidak bisa kita baca, celahnya didokumentasikan.

### Checkpoint A — `Decode` membaca simbol Micro dari gambar, dicek terhadap simbol zint dan reader zxing-cpp.

### MD6 — Micro di DecodeAll `(M)`

- [ ] Tahap Micro atas finder quad yang belum diklaim di `decodeMultipleFromBitmap`, dan `Result.MicroVersion`.
- [ ] Test pada scene komposit: beberapa simbol Micro berbeda version, Micro dan QR bersama, tile yang dirotasi, dan urutan baca; cek sekali jalan dengan zxing-cpp pada satu komposit.

### MD7 — Polish & Rilis `(S)`

- [ ] README (bagian decoding, scope, limitations, roadmap) dan `examples/decode/micro`.
- [ ] Entri `v0.16.0` di `CHANGELOG.md`, dibiarkan unstaged untuk release branch.
- [ ] Baris peta kode theory untuk dokumen 27, tanpa tanda direncanakan.
- [ ] `go test -race ./...` bersih, gofmt bersih.

## 7. Strategi validasi

1. **Simbol zint sebagai gambar.** Ke-34 simbol zint di `micro_symbols.txt` dirender oleh test pada beberapa ukuran modul, quiet zone, dan rotasi, dan `Decode` harus mengembalikan teks masing-masing. Simbol-simbol itu berasal dari encoder yang tidak berbagi kode dengan kita, jadi kesalahan yang dimiliki bersama oleh encoder dan detektor kita tidak bisa bersembunyi di balik round trip.
2. **PNG kita sendiri.** Setiap tipe simbol dari `Encode(..., WithMicro())` ter-decode lewat `DecodeBytes`; perlu, tetapi tidak pernah cukup dengan sendirinya.
3. **Reader independen.** zxing-cpp membaca scene sintetis yang sama, yang ditulis ke file di direktori scratch; cek sekali jalan yang dilaporkan di commit MD5 dan MD6, karena test Go tidak bisa memanggil Python.
4. **Kasus negatif.** Gambar kosong, finder pattern tunggal tanpa timing, sudut-sudut simbol QR (utuh, dan dengan satu finder dihapus agar percobaan QR gagal), serta noise tidak boleh menghasilkan teks Micro.
5. **Tanpa regresi QR.** Setiap test yang ada lolos tanpa perubahan, dan fixture QR ter-decode lewat jalur yang sama (`affine` atau `projective`) seperti sebelumnya.

## 8. Usulan Delta Tata Letak File

```
qrgen/
├── decode_micro.go         # baru — winding, homografi per rotasi, gerbang orientasi, pembacaan format dan timing, decodeMicroQuad
├── decode_micro_test.go    # baru — bitmap yang dilukis, simbol zint sebagai gambar, sweep, kasus negatif
├── decode_finder_cc.go     # allFinderQuads; detectFinderQuads di atasnya
├── decode_image.go         # tahap Micro di percobaan connected-component, kontrak error, decodeImageWhich
├── decode_multi.go         # tahap Micro atas quad yang belum diklaim, Result.MicroVersion
├── decode_multi_test.go    # scene Micro untuk DecodeAll
└── decode.go               # doc comment Decode dan DecodeMatrix
docs/
├── plan-micro-qr-detection.md / .id.md
└── theory/27-micro-qr-detection.md / .id.md
examples/decode/micro/main.go
```

## 9. Risiko & Catatan Teknis

- **Ekstrapolasi.** Homografi finder ditentukan oleh persegi 7 modul dan harus menjangkau 17 modul di M4, sehingga noise di sudut diperbesar sekitar dua setengah kali di sudut terjauh, lebih lagi di bawah perspektif. Diukur di MD5 sebelum penyempurnaan apa pun ditambahkan.
- **False positive.** Sudut simbol QR memuat finder pattern dengan bentuk yang persis sama. Menjalankan Micro hanya setelah QR gagal di bitmap yang sama, gerbang orientasi, cek silang timing, dan Reed-Solomon berdiri di antara sudut itu dan decode palsu. M1, dengan dua EC codeword yang hanya mendeteksi dan timing pattern 3 modul, adalah kasus terlemah dan mendapat test negatif khusus.
- **Modul kecil.** Piksel sudut adalah pusat piksel, setengah piksel di dalam sudut luar yang sebenarnya; pada 2 piksel per modul itu sudah seperempat modul sebelum ekstrapolasi. Sweep ukuran modul yang memutuskan apakah perlu dikoreksi.
- **Quiet zone yang sempit.** Micro hanya butuh quiet zone 2 modul, dan cek silang timing membaca modul pertama di luar simbol, jadi objek lain yang berjarak 2 modul tidak masalah, tetapi yang menempel pada simbol akan mengganggu. Didokumentasikan bersama rentangnya.
- **Kontrak error.** Satu-satunya perubahan pada error yang dikembalikan hanya menyangkut gambar yang percobaan afinnya tidak menemukan triple finder tetapi punya finder yang meyakinkan sebagai bagian dari simbol Micro; test yang ada mengunci sisanya, dan simbol QR yang satu finder-nya terhapus tetap dengan error-nya (MD4).
- **Biaya.** Paling banyak `maxCCCandidates` (64) quad, empat rotasi masing-masing, beberapa lusin pembacaan modul per rotasi, dan sampling serta decode penuh hanya untuk kandidat yang lolos gerbang: tak berarti dibanding labelling, yang dipakai bersama dengan percobaan QR.

## 10. Referensi

- ISO/IEC 18004:2015 — tata letak Micro QR, timing pattern, format information, quiet zone, dan aturan masking yang menjaga tepi-tepi jauh tetap kaya modul gelap.
- zxing-cpp, core/src/qrcode/QRDetector.cpp (`SampleMQR`, `DetectPureMQR`), QRReader.cpp, dan QRFormatInformation.cpp (`DecodeMQR`), master yang dibaca pada November 2026 — prior art yang dirangkum di bagian 3; reader-nya menjadi cek independen untuk validasi.
- zint, lewat zxing-cpp 2.3.0 — encoder di balik simbol Micro yang tersimpan di repo.
- `docs/theory/24-arbitrary-angle-detection.md`, `25-multi-symbol-decoding.md`, `26-micro-qr.md` — detektor connected-component dan geometri projektif yang dipakai ulang milestone ini, claim loop milik `DecodeAll`, dan simbol Micro itu sendiri.

## 11. Pertanyaan Terbuka

- **Micro di `Result`.** Usulannya menambah `MicroVersion MicroVersion` dan membiarkan `Finders[1]` dan `Finders[2]` nol untuk simbol Micro. Alternatifnya, tipe result terpisah untuk Micro, akan memecah loop setiap pemanggil menjadi dua. Dikonfirmasi sebelum MD6.
- **Toleransi timing.** Terjawab di MD3: dua. Simbol yang bersih terbaca tanpa ketidakcocokan; pada setiap finder dari 224 simbol QR (version 1 sampai 7, semua level EC dan mask), penetapan yang sampai ke cek silang tidak cocok di 1 sampai 14 modul, dengan median 8, dan tiga yang masuk toleransi ditolak oleh Reed-Solomon.
- **Penyempurnaan sudut jauh.** Apakah homografi finder saja bertahan di rentang yang ingin kita klaim, atau sudut dari ujung timing diperlukan; diukur di MD5.
- **Kontrak error.** Terjawab di MD4: ya, tetapi hanya untuk bukti Micro yang meyakinkan. Lolos gerbang orientasi, yang dicapai sekitar sepertiga finder QR, ternyata terlalu lemah; timing pattern yang cocok persis dengan format information yang tak terbaca, atau simbol yang sudah ditemukan tetapi datanya gagal, dicapai kurang dari satu dari seratus finder QR, dan tidak satu pun dari 96 simbol QR yang satu finder-nya terhapus berubah error-nya.
