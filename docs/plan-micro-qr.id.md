# QR Generator — Rencana Micro QR Code

Dokumen ini menjelaskan rencana implementasi **Micro QR Code** (versi M1 sampai M4) untuk rilis minor `v0.15.0`. Micro QR adalah simbologi terakhir di ISO/IEC 18004 yang belum bisa dihasilkan library ini. Milestone ini menghadirkan encoder untuk kedelapan tipe simbol Micro plus decoder tingkat matriks, sehingga simbol Micro bisa round-trip lewat `DecodeMatrix`; menemukan simbol Micro di dalam foto ditunda ke milestone berikutnya.

> Status: **draf / dokumen hidup.** Milestone MQ1..MQ6 mendarat bertahap di branch `micro-qr`; tiap milestone adalah commit fokus (atau deret kecil) berikut test, mengikuti irama milestone sebelumnya.

> Versi bahasa Inggris: [docs/plan-micro-qr.md](plan-micro-qr.md).

---

## 1. Visi & Tujuan

- Meng-encode Micro QR M1, M2, M3, dan M4 pada setiap level error correction yang mereka definisikan: delapan tipe simbol seluruhnya.
- Membuat API yang ada menghasilkan simbol Micro lewat option, seperti cara ECI, logo, dan output terminal ditambahkan, sehingga setiap fungsi encode dan renderer langsung mendukung Micro.
- Men-decode simbol Micro di tingkat matriks: `DecodeMatrix` mengenali ukuran Micro (11, 13, 15, 17) dengan sendirinya, karena ukuran itu tak mungkin menjadi ukuran QR (21 + 4k).
- Memvalidasi terhadap implementasi **independen**, tidak pernah hanya terhadap diri sendiri. v0.14.1 baru saja memperbaiki bug format information yang bertahan tiga belas rilis karena decoder kita hanya pernah membaca output encoder kita sendiri; Micro hanya punya satu strip format, jadi kesalahan sejenis akan membuat setiap simbol Micro tak terbaca oleh scanner sungguhan.
- Menjaga **zero runtime dependency**, dan menjaga fixture test tetap berupa matriks teks.
- Jujur soal cakupan: tidak ada deteksi Micro di tingkat gambar pada milestone ini.

## 2. Apa itu Micro QR

Angka di bawah berasal dari tabel ISO/IEC 18004:2015 sebagaimana ditranskripsikan di segno 1.6.6, dan sudah dicek silang terhadap zint pada 69 simbol (57 identik byte-for-byte; 12 sisanya hanya berbeda di padding, tempat segno menyimpang dari standar; lihat bagian 3).

| Nomor simbol | Tipe | Ukuran | Total codeword | Data codeword | EC codeword | Bit data |
|---|---|---|---|---|---|---|
| 0 | M1 (hanya deteksi error) | 11 | 5 | 3 (yang terakhir 4 bit) | 2 | 20 |
| 1 | M2-L | 13 | 10 | 5 | 5 | 40 |
| 2 | M2-M | 13 | 10 | 4 | 6 | 32 |
| 3 | M3-L | 15 | 17 | 11 (yang terakhir 4 bit) | 6 | 84 |
| 4 | M3-M | 15 | 17 | 9 (yang terakhir 4 bit) | 8 | 68 |
| 5 | M4-L | 17 | 24 | 16 | 8 | 128 |
| 6 | M4-M | 17 | 24 | 14 | 10 | 112 |
| 7 | M4-Q | 17 | 24 | 10 | 14 | 80 |

Setiap tipe memakai satu blok Reed-Solomon. Tidak ada level H, dan tidak ada ECI, structured append, maupun FNC1.

- **Mode.** Lebar mode indicator adalah 0, 1, 2, atau 3 bit untuk M1 sampai M4, dengan nilai numeric 0, alphanumeric 1, byte 2, Kanji 3. M1 hanya mendukung numeric; M2 menambah alphanumeric; M3 dan M4 mendukung keempat mode. Lebar character count adalah numeric 3/4/5/6, alphanumeric -/3/4/5, byte -/-/4/5, dan Kanji -/-/3/4 untuk M1 sampai M4.
- **Terminator.** 3, 5, 7, atau 9 bit nol untuk M1 sampai M4, dipotong kalau kapasitas lebih dulu habis.
- **Padding.** Setelah terminator, bit nol mengisi sampai batas codeword berikutnya (tidak ada kalau sudah tepat di batas), lalu `11101100` dan `00010001` bergantian mengisi data codeword penuh yang tersisa. Di M1 dan M3, data codeword terakhir hanya 4 bit dan di-pad sebagai `0000`. Untuk Reed-Solomon, half codeword itu adalah satu byte dengan 4 bitnya di nibble atas; hanya 4 bit itu yang ditempatkan di simbol.
- **Tata letak.** Satu finder pattern di kiri-atas, dipisahkan dari data hanya di sisi kanan dan bawahnya (blok 8 kali 8). Timing pattern berjalan di baris 0 dan kolom 0 mulai indeks 8, gelap pada indeks genap. Tidak ada alignment pattern, version information, maupun dark module.
- **Format information.** Satu salinan 15 bit: kolom 8 baris 1..8 memuat bit 0..7, dan baris 8 kolom 1..7 memuat bit 14..8. Word-nya adalah BCH(15,5) atas nomor simbol 3 bit dan mask 2 bit, dengan generator yang sama seperti QR dan XOR mask `0x4445`.
- **Penempatan data.** Zigzag QR dalam kolom selebar dua modul dari kanan, pasangan pertama ke atas, tetapi tanpa lompatan kolom 6 milik QR: kolom 0 adalah timing, dan kolom 6 di bawah finder berisi data.
- **Mask.** Empat pola, mask Micro 0..3 sama dengan mask QR 1, 4, 6, dan 7. Pemilihannya menghitung modul gelap di kolom terakhir (SUM1) dan baris terakhir (SUM2), masing-masing tanpa modul timing: `SUM1*16 + SUM2` kalau `SUM1 <= SUM2`, selain itu `SUM2*16 + SUM1`. Skor **tertinggi** yang menang, kebalikan dari penalti terendah milik QR; segno memecah seri ke indeks terendah.

## 3. Oracle, dan apa yang kita pelajari darinya

gozxing, oracle test library ini selama ini, tidak mendukung Micro QR, jadi milestone ini memakai implementasi referensi secara offline selama pengembangan dan tidak pernah sebagai dependensi modul.

- **zint**, diakses lewat `create_barcode` milik zxing-cpp 2.3.0, adalah encoder yang cermat terhadap spec dan menjadi **sumber golden**. Ia menghasilkan kedelapan tipe: `ec_level` `"1"`, `"2"`, dan `"3"` memilih L, M, dan Q, sedangkan `"4"` ditolak dengan pesan zint sendiri "Error correction level H not available". Ia memilih version dan mask sendiri; keduanya dibaca balik dari format information simbol.
- **Reader zxing-cpp** men-decode gambar Micro QR: decoder independen untuk output yang kita render.
- **segno** mentranskripsikan tabel spec dengan benar, tetapi padding-nya menyimpang di dua tempat. Ia mengisi seluruh padding dengan nol di M1 dan M3, bukan menyelang-nyelingkan pad codeword. Dan ketika terminator jatuh tepat di batas codeword, ia tetap menambahkan delapan bit nol: `"1234"` di M4-Q panjangnya tepat 32 bit, dan segno menggeser setiap pad codeword satu posisi. Decoder mengabaikan bit pad, jadi simbol segno tetap terbaca, tetapi golden yang diambil dari segno akan membuat encoder yang benar tampak salah. Tabel dicek silang dengan segno; golden diambil dari zint.
- **Seri pada segmentasi.** Dua encoder optimal bisa memecah teks mixed-mode secara berbeda dan keduanya benar. Karena itu golden byte-exact memakai input single-mode; input mixed-mode dicek lewat decoding.

## 4. Pendekatan, berpijak pada kode

Pembacaan encoder (mode.go, version.go, segment.go, encode.go, matrix.go, mask.go, formatinfo.go, options.go, api.go, renderer, dan decoder matriks) membagi pekerjaan menjadi tiga kelompok.

- **Bisa dipakai apa adanya:** `bitBuffer` (byte parsial di ujungnya yang diisi nol persis merupakan konvensi RS untuk half codeword), writer payload, GF(256) dan `encodeBlock`, primitif matriks dan `placeSingleFinder` (clipping di tepinya sudah menghasilkan separator Micro), `maskCondition` lewat pemetaan 0→1, 1→4, 2→6, 3→7, `applyMask`, `ECLevel`, ketiga renderer (mereka hanya membaca `m.size` dan `m.modules`), pola option, serta reader payload milik decoder dan `rsDecode`.
- **Khusus Micro, dibuat paralel:** tipe `MicroVersion` dan tabelnya; tabel mode indicator, lebar count, dan terminator; cost model yang disuntikkan ke DP segmentasi; `encodeMicroText` dengan padding Micro; stream data-plus-EC yang bit-exact (36 / 80 / 132 / 192 bit); reservasi timing dan format; writer dan reader format satu salinan; pemilihan empat mask dengan skor tertinggi; `microMatrixFromData`; flag option dan validasi; serta `decodeMicroMatrix`.
- **Asumsi tersembunyi yang harus dihindari:** kapasitas dihitung sebagai jumlah codeword kali 8; mode indicator dan terminator 4 bit yang di-hardcode; asumsi DP bahwa mode byte selalu tersedia (M1 dan M2 bisa membuat input tidak feasible, dan itu harus muncul sebagai "tidak muat", bukan segmen palsu); `Version(0).Size()` yang mengembalikan 17 (Micro tidak pernah masuk ke tipe `Version`); lompatan kolom 6; quiet zone default 4 modul; default EC yang tidak punya status "belum diset"; serta arah pemilihan mask.

## 5. Keputusan desain

- **API lewat option.** `WithMicro()` memilih simbol Micro terkecil yang muat; `WithMicroVersion(v MicroVersion)` memaksa satu tipe, dengan `type MicroVersion int` dan konstanta `M1`..`M4`. Satu cabang di `buildMatrix` memberi dukungan Micro ke `Encode`, `EncodeToFile`, `EncodeSVG`, `EncodeSVGToFile`, `EncodeTerminal`, dan `Matrix`. `validate` menolak, dengan error yang spesifik, `WithVersion` bersama Micro, `WithECI`, `WithLogo` (yang akan memaksa level H yang tidak dimiliki Micro), level EC H, level EC Q di bawah M4, mask di atas 3, dan `EncodeStructuredAppend` bersama Micro.
- **Kebijakan EC, termasuk M1.** Option mendapat flag yang mencatat apakah level EC diset secara eksplisit. Kalau tidak, M1 boleh dipilih untuk data numeric yang pendek dan level default M berlaku untuk sisanya (M2-M, M3-M, M4-M), sesuai pilihan otomatis zint. Level eksplisit memilih di antara tipe yang menawarkannya: L memberi M2-L sampai M4-L, M memberi M2-M sampai M4-M, Q hanya M4-Q, dan H adalah error. M1 hanya menyediakan deteksi error, jadi ia tak pernah dipilih ketika pemanggil meminta level koreksi.
- **Tidak ada fallback diam-diam.** Kalau data tidak muat di M4 pada level yang diminta, encoder mengembalikan error kapasitas khusus Micro, bukan diam-diam menghasilkan simbol QR penuh.
- **Quiet zone.** Micro default ke quiet zone 2 modul sesuai spec; `WithQuietZone` yang eksplisit tetap menang. Ini butuh flag "sudah diset", karena sentinel negatif akan merusak penolakan quiet zone negatif yang sudah ada.
- **Input kosong.** Valid di M1 menurut spec (count nol), dan inilah satu-satunya kasus M1 punya pad codeword penuh. zint menolak input kosong, jadi kasus ini dicek terhadap bit stream yang diturunkan dengan tangan dan round trip kita sendiri, bukan golden.
- **Decoding matriks.** `decodeMatrixSA` mengarahkan grid 11/13/15/17 ke `decodeMicroMatrix` sebelum cek ukuran QR. Ia membaca satu salinan format dengan budget 3 bit dan mensyaratkan nomor simbol hasil decode cocok dengan ukuran grid. M1 hanya deteksi: syndrome yang tidak nol membuat decode gagal, bukan dikoreksi.
- **Gambar, nanti.** `Decode`, `DecodeBytes`, dan `DecodeAll` tetap membutuhkan tiga finder pattern. Menemukan simbol Micro (satu finder plus timing pattern) adalah milestone berikutnya.

## 6. Milestone

Milestone mendarat berurutan. **Checkpoint A** (setelah MQ5) me-round-trip setiap tipe Micro dan cocok dengan zint byte-for-byte. **Checkpoint B** (MQ6) adalah rilis `v0.15.0`.

### MQ1 — Plan Doc `(S)`

- [x] Dokumen ini dan padanan bahasa Inggrisnya.

### MQ2 — Dokumen Referensi `(S)`

- [ ] `docs/theory/26-micro-qr.md` (EN + ID): tipe simbol dan kapasitasnya, mode dan lebar bitnya, terminator dan padding termasuk half codeword, tata letak dan format information, penempatan, mask dan penilaiannya, bagaimana Micro berbeda dari QR di tiap langkah, serta bit stream hasil turunan tangan yang mengunci encoding-nya. Plus entri README theory dan baris pemetaan kode.

### MQ3 — Tabel, Segmentasi, dan Bit Stream `(L)`

- [ ] `MicroVersion`, tabel tipe simbol, serta tabel mode, lebar count, dan terminator.
- [ ] Suntikkan cost model (mode yang diizinkan dan bit header) ke DP segmentasi, mempertahankan `segmentText` sebagai wrapper QR, dan laporkan ketidak-feasible-an.
- [ ] `encodeMicroText`: pemilihan version dan EC, segmen, terminator, padding dengan half codeword, dan stream data-plus-EC yang bit-exact.
- [ ] Test: bit stream turunan tangan, dan data codeword yang dibaca dari simbol zint.

### MQ4 — Matriks, Mask, dan Option `(L)`

- [ ] Matriks Micro: finder, separator, timing, reservasi format, penempatan tanpa lompatan kolom 6, writer format satu salinan, dan pemilihan empat mask.
- [ ] `WithMicro`, `WithMicroVersion`, flag EC-set dan quiet-zone-set, serta validasi.
- [ ] Test golden: encoder kita yang dipaku ke tipe dan mask milik zint mereproduksi setiap simbol zint modul demi modul, dan pilihan otomatis tipe dan mask kita cocok dengan zint untuk level EC default.

### MQ5 — Decoder Matriks dan Round Trip `(M)`

- [ ] `decodeMicroMatrix`: baca format, unmask, stream bit-exact, RS (hanya deteksi untuk M1), dan parsing segmen Micro; dispatch `DecodeMatrix` berdasarkan ukuran.
- [ ] Test: decode setiap fixture zint, round-trip encoder kita lintas mode dan tipe, dan sesuaikan test lama yang memakai 17 sebagai ukuran QR yang tidak valid.
- [ ] Cek sekali jalan, dilaporkan di commit: zxing-cpp membaca PNG Micro yang kita render.

### Checkpoint A — setiap tipe Micro bisa round-trip dan cocok dengan zint.

### MQ6 — Polish & Rilis `(S)`

- [ ] README (bagian encoding, scope, limitations, roadmap), `examples/encode/micro`, dan flag CLI.
- [ ] Entri `v0.15.0` di `CHANGELOG.md`, dibiarkan unstaged untuk release branch.
- [ ] `go test -race ./...` bersih, gofmt bersih.

## 7. Strategi validasi

1. **Golden byte-exact dari zint.** Kedelapan tipe, termasuk input yang mengisi kapasitas penuh, input pendek dengan pad codeword, kasus tepat-di-batas (`"1234"` di M4-Q), dan kasus half codeword M1/M3, disimpan sebagai teks di `qrgen/testdata/thirdparty/micro_symbols.txt` di samping fixture QR yang ditambahkan di v0.14.1.
2. **Decoding.** `DecodeMatrix` membaca setiap fixture zint.
3. **Round trip** encoder kita sendiri, lintas mode dan tipe; perlu, tetapi tidak pernah cukup dengan sendirinya.
4. **Decoder independen.** zxing-cpp membaca PNG yang kita render, sekali per tipe, dilaporkan di commit karena test Go tidak bisa memanggil Python.
5. **Bit stream turunan tangan.** Lima kasus sudah dicek bit demi bit terhadap aturannya: M1 `"1"` dan `"12345"` (kapasitas penuh, tanpa terminator), M2-L `"0123456789"` (terminator terpotong jadi satu bit), M4-Q `"1234"`, dan M3-M `"HELLO WORLD"` (berakhir di half codeword `1010`).
6. **Format word.** Tabel BCH yang dihitung cocok dengan 32 entri `FORMAT_INFO_MICRO` milik segno.

## 8. Usulan Delta Tata Letak File

```
qrgen/
├── micro.go                # baru — MicroVersion, tabel tipe simbol dan lebar bit
├── micro_encode.go         # baru — cost model, encodeMicroText, padding, bit stream
├── micro_matrix.go         # baru — tata letak, format information, mask
├── micro_decode.go         # baru — decodeMicroMatrix
├── micro_test.go           # baru — tabel, bit stream, round trip
├── micro_golden_test.go    # baru — golden zint dan decoding
├── segment.go              # injeksi cost model; perilaku QR tidak berubah
├── options.go, encode.go   # option Micro, flag, validasi, cabang buildMatrix
├── decode.go               # dispatch DecodeMatrix berdasarkan ukuran
└── testdata/thirdparty/micro_symbols.txt   # baru — fixture zint
docs/
├── plan-micro-qr.md / .id.md
└── theory/26-micro-qr.md / .id.md
examples/encode/micro/main.go
```

## 9. Risiko & Catatan Teknis

- **Jebakan konsistensi diri.** Round trip kita sendiri tidak membuktikan apa pun soal kesesuaian dengan spec; golden zint dan cek zxing-cpp adalah kriteria penerimaan yang sesungguhnya.
- **Refactor DP segmentasi menyentuh jalur QR.** Fixture QR pihak ketiga dari v0.14.1 harus tetap identik byte-for-byte, bersama test yang sudah ada.
- **Seri pada mask.** Cara zint memecah seri mungkin berbeda dari aturan indeks-terendah milik segno. Golden yang dipaku tidak terpengaruh; cek pilihan otomatis akan mengungkapnya, dan perbedaan apa pun didokumentasikan, bukan disembunyikan.
- **Kanji di M3/M4.** zint harus benar-benar memilih mode Kanji untuk input Kanji agar ada golden byte-exact; kalau tidak, kasus itu menjadi decode-only.
- **Flag option baru.** Flag EC-set dan quiet-zone-set harus membiarkan default dan perilaku QR persis seperti sekarang.
- **Semantik M1.** Hanya-deteksi berarti decoder harus menolak mengoreksi, sesuatu yang secara default tidak dilakukan `rsDecode`.

## 10. Referensi

- ISO/IEC 18004:2015 — tabel Micro QR, tata letak, format information, masking, dan aturan padding.
- Source segno 1.6.6 (`consts.py`, `encoder.py`) — tabel, sudah dicek silang; dua penyimpangan padding-nya dicatat di bagian 3.
- zint, lewat zxing-cpp 2.3.0 — encoder golden; reader zxing-cpp sebagai decoder independen.
- `docs/theory/05-matrix-construction.md`, `06-masking.md`, `07-format-version-info.md`, `17-optimal-segmentation.md`, `22-kanji-mode.md` — padanan QR yang diparalelkan milestone ini.

## 11. Pertanyaan Terbuka

- **Nama konstanta.** `M1`..`M4` terbaca natural sebagai `qrgen.M2`; `MicroM1`..`MicroM4` lebih eksplisit. Usulan: `M1`..`M4`.
- **Input kosong.** Ikuti spec (valid di M1) atau ikuti zint (tolak)? Usulan: ikuti spec.
- **Bentuk CLI.** Switch `-micro` plus `-micro-version M1..M4`, atau satu `-version` yang juga menerima `M1`..`M4`? Diputuskan di MQ6.
- **Deteksi gambar.** Jadwalkan deteksi Micro di dalam foto sebagai milestone tersendiri setelah v0.15.0.
