# QR Generator — Rencana rMQR

Dokumen ini menjelaskan rencana implementasi **rMQR** (Rectangular Micro QR Code, ISO/IEC 23941) untuk rilis minor `v0.17.0`. rMQR adalah anggota ketiga keluarga QR setelah QR Code dan Micro QR: simbol persegi panjang setinggi 7 sampai 17 modul dan selebar 27 sampai 139 modul, dibuat untuk ruang panjang dan sempit yang tidak bisa dipakai simbol persegi. Milestone ini menghadirkan encoder untuk ke-32 versi di kedua level error correction-nya, plus decoder tingkat matriks, sehingga simbol rMQR bisa round-trip lewat `DecodeMatrix`; menemukan simbol rMQR di dalam gambar ditunda ke milestone berikutnya, seperti yang terjadi pada Micro QR.

> Status: **draf / dokumen hidup.** Milestone RQ1..RQ6 mendarat bertahap di branch `rmqr`; tiap milestone adalah commit fokus (atau deret kecil) berikut test, mengikuti irama milestone sebelumnya.

> Versi bahasa Inggris: [docs/plan-rmqr.md](plan-rmqr.md).

---

## 1. Visi & Tujuan

- Meng-encode rMQR di ke-32 versi, R7x43 sampai R17x139, pada level M dan H: 64 tipe simbol seluruhnya.
- Membuat API yang ada menghasilkan rMQR lewat option, seperti cara Micro QR ditambahkan, sehingga setiap fungsi encode dan renderer langsung mendukung rMQR.
- Men-decode rMQR di tingkat matriks: `DecodeMatrix` menerima grid persegi panjang dan mengenali simbol rMQR dari bentuknya, karena tidak ada ukuran rMQR yang persegi.
- Memvalidasi terhadap implementasi **independen**, tidak pernah hanya terhadap diri sendiri: zint untuk golden byte-exact, reader zxing-cpp sebagai decoder independen, dan setiap tabel dicek silang di tiga sumber.
- Menjaga **zero runtime dependency**, dan menjaga fixture test tetap berupa matriks teks.
- Jujur soal cakupan: tidak ada deteksi rMQR di tingkat gambar pada milestone ini.

## 2. Apa itu rMQR

Fakta di bawah berasal dari tiga implementasi yang tidak berbagi kode (bagian 3), yang sepakat di setiap tabel; ISO/IEC 23941:2022 sendiri tidak tersedia, tetapi nomor tabelnya dikutip dari komentar source implementasi-implementasi itu. Dokumen theory 28 memuat tabel lengkap per versi.

- **Versi.** 32 ukuran bernama R*h*x*w*: tinggi 7, 9, 11, 13, 15, 17 dan lebar 43, 59, 77, 99, 139, ditambah lebar 27 untuk tinggi 11 dan 13. Versi diberi nomor 1 sampai 32 dalam urutan itu (R7x43 adalah 1, R11x27 adalah 11, R17x139 adalah 32), dan format information membawa nomor itu dikurangi satu.
- **Error correction.** Hanya dua level, M dan H. Satu blok Reed-Solomon pada simbol kecil, sampai enam blok pada R17x139-H, di-interleave persis seperti QR (blok pendek lebih dulu, satu data codeword tambahan di blok panjang). Kapasitas data berkisar dari 3 codeword (R7x43-H) sampai 152 (R17x139-M).
- **Mode.** Mode indicator 3 bit untuk setiap versi: terminator `000`, numeric `001`, alphanumeric `010`, byte `011`, Kanji `100`, FNC1 `101` dan `110`, ECI `111`. Lebar character count bergantung pada versi, bukan pada kelompok versi: numeric 4 sampai 9 bit, alphanumeric 3 sampai 8, byte 3 sampai 8, Kanji 2 sampai 7.
- **Terminator dan padding.** Terminator 3 bit, dipotong kalau kapasitas habis lebih dulu, lalu bit nol sampai batas codeword dan pad codeword QR `11101100` dan `00010001`. Tidak ada half codeword.
- **Tata letak.** Finder pattern 7 kali 7 di kiri-atas, dengan separator di kanannya dan, mulai tinggi 9, di bawahnya; finder sub-pattern 5 kali 5 di kanan-bawah; corner finder pattern berbentuk L di kanan-atas dan kiri-bawah; timing pattern di sepanjang keempat tepi; dan, untuk setiap lebar kecuali 27, alignment pattern di tepi atas dan bawah yang dihubungkan timing pattern vertikal, pada kolom 21 (lebar 43), 19 dan 39 (59), 25 dan 51 (77), 23, 49, dan 75 (99), serta 27, 55, 83, dan 111 (139).
- **Format information.** Dua salinan 18 bit: 6 bit data (level, H bernilai 1, di atas version indicator 5 bit) yang dilindungi BCH(18,6) dengan generator version information QR `0x1F25`, lalu di-XOR dengan `0x1FAB2` untuk salinan di samping finder dan `0x20A7B` untuk salinan di samping sub-pattern. Jarak minimum kodenya 8, jadi setiap salinan bisa mengoreksi sampai 3 error.
- **Penempatan.** Zigzag QR dalam kolom selebar dua modul, tetapi dimulai dari kolom kedua dari kanan, karena kolom paling kanan adalah timing, dan tanpa kolom yang dilompati. Antara 0 dan 7 remainder bit mengisi modul setelah codeword terakhir.
- **Masking.** Satu mask tetap, mask 4 milik QR, `(⌊baris / 2⌋ + ⌊kolom / 3⌋) mod 2 = 0`, diterapkan ke modul data dan remainder. Tidak ada pemilihan mask dan tidak ada mask di format information.
- **Quiet zone.** 2 modul, sama seperti Micro QR.

## 3. Oracle, dan apa yang kita pelajari darinya

gozxing tidak mendukung rMQR, jadi seperti di v0.15 milestone ini memakai implementasi referensi secara offline, di virtual environment scratch, tidak pernah sebagai dependensi modul.

- **zint**, diakses lewat zint-bindings 1.2.2, adalah **sumber golden**. Berbeda dengan `create_barcode` milik zxing-cpp, yang dipakai v0.15 untuk Micro QR tetapi tidak bisa memilih ukuran simbol, zint-bindings membuka option zint sendiri, sehingga setiap versi dan level bisa dipaksa (`option_2` 1 sampai 32, `option_1` 2 untuk M dan 4 untuk H).
- **Reader zxing-cpp** men-decode rMQR: decoder independen untuk output kita. Source-nya (QRVersion, QRFormatInformation, QRBitMatrixParser, QRCodecMode) dibaca sebagai transkripsi kedua dari tabel-tabelnya.
- **rmqrcode 0.3.2**, encoder Python, adalah transkripsi ketiga. Ia sepakat dengan dua sumber lain di semua hal kecuali dua definisi blok, R13x27-M (14 data codeword, sementara zint dan zxing-cpp punya 12) dan R17x43-M (60 codeword per blok, sementara keduanya punya 61), jadi, seperti segno di v0.15, ia mengonfirmasi tabel tetapi tidak pernah memasok golden.
- **Sudah dicek saat perencanaan.** Ketiga sumber sepakat di setiap ukuran, jumlah codeword, struktur blok, lebar character count, dan kolom alignment; ke-64 format word di tiap salinan direproduksi persis oleh perhitungan BCH dan kedua mask; dan model tata letak yang dibangun dari fakta di atas cocok dengan 64 simbol zint, setiap versi di kedua level, di setiap modul fungsi dan kedua salinan format, dengan remainder bit yang sama dengan rmqrcode.
- **Pilihan otomatis zint.** Tanpa level yang diminta, zint mengambil versi dengan luas terkecil yang muat di M, lalu memakai H sebagai gantinya kalau data juga muat di H pada versi yang sama. Dengan level yang diminta, ia mengambil versi dengan luas terkecil yang muat di level itu. R7x99 sama luasnya dengan R9x77 (693 modul) tetapi tidak pernah terpilih otomatis, karena selalu ada simbol yang lebih kecil dengan kapasitas lebih besar.

## 4. Pendekatan, berpijak pada kode

Pembacaan encoder dan decoder setelah v0.16 membagi pekerjaan menjadi tiga kelompok.

- **Bisa dipakai apa adanya:** `bitBuffer` dan writer payload; DP segmentasi dengan cost model yang bisa disuntikkan dari v0.15; GF(256), `encodeBlock`, dan `rsDecode`; `maskCondition(4, …)`; writer dan reader designator ECI; reader payload milik decoder (`decodeNumeric` dan lainnya); pola option dengan flag EC-set dan quiet-zone-set; dan, setelah bisa menerima persegi panjang, ketiga renderer.
- **Khusus rMQR, dibuat paralel:** tipe `RMQRVersion` dan tabel 32 versi; cost model rMQR (indicator 3 bit, lebar count per versi); `encodeRMQRText` dengan pemilihan versi dan level, terminator, padding, pembagian blok, dan interleaving; fungsi word BCH(18,6); tata letak function pattern; writer dan reader format dua salinan; penempatan dari kolom kedua; decoder matriks dan parser segmennya; option dan validasi.
- **Asumsi tersembunyi yang harus dihindari:** satu-satunya `size` pada tipe `matrix`, yang dibaca renderer PNG, SVG, dan terminal, `Matrix`, `applyMask`, dan `placeBits` sebagai tinggi sekaligus lebar; `matrixFromGrid` dan `DecodeMatrix` yang menolak grid tidak persegi; `placeBits` yang memulai pasangan kolomnya di kolom paling kanan, yang untuk rMQR akan memasangkan kolom secara salah sambil tetap round-trip secara internal; format word 15 bit; tipe `Version` milik QR, yang harus tetap bersih dari rMQR seperti Micro; ECLevel L dan Q yang dianggap valid; pemilihan mask; dan indicator ECI 4 bit milik QR.

## 5. Keputusan desain

- **API lewat option (pertanyaan terbuka 1).** `WithRMQR()` memilih simbol terkecil yang muat; `WithRMQRVersion(v RMQRVersion)` memaksa satu, dengan konstanta `R7x43` sampai `R17x139` sesuai penamaan standar; dan `WithRMQRHeight(rows int)` menetapkan tinggi dan membiarkan lebarnya mengikuti, kasus yang menjadi alasan rMQR ada (zint menawarkan pilihan yang sama sebagai R7xW sampai R17xW). Satu cabang di `buildMatrix` memberi dukungan rMQR ke `Encode`, `EncodeToFile`, `EncodeSVG`, `EncodeSVGToFile`, `EncodeTerminal`, dan `Matrix`.
- **Validasi.** Ditolak dengan error yang spesifik: `WithVersion`, `WithMicro` atau `WithMicroVersion` bersama rMQR; `WithMask` (mask rMQR tetap); `WithLogo` (simbol setinggi 7 sampai 17 modul tidak punya ruang untuk logo); level EC L dan Q; `EncodeStructuredAppend` (rMQR tidak mendefinisikannya). `WithECI` diizinkan, karena rMQR mendefinisikan ECI: header-nya adalah indicator 3 bit `111` diikuti designator yang sama seperti QR.
- **Kebijakan level dan ukuran: milik zint.** Tanpa level yang diset, versi dengan luas terkecil yang muat di M, lalu H kalau data juga muat di H pada versi itu: koreksi tambahannya gratis. M atau H yang eksplisit memilih versi dengan luas terkecil di level itu. Dengan tinggi yang ditetapkan, aturan yang sama berlaku di antara lebar-lebar tinggi tersebut. Kalau tidak ada yang muat, error `ErrCapacityExceeded`, tidak pernah simbol QR diam-diam.
- **Matriks persegi panjang.** Tipe `matrix` mendapat jumlah baris dan kolom terpisah; renderer, `Matrix`, `applyMask`, dan `placeBits` membacanya; QR dan Micro tetap memakai matriks persegi dan output yang identik byte-for-byte.
- **Quiet zone.** Default 2 modul, sesuai permintaan standar; `WithQuietZone` yang eksplisit tetap menang.
- **Decoding matriks.** `DecodeMatrix` mengarahkan grid persegi panjang yang bentuknya salah satu dari 32 ukuran rMQR ke `decodeRMQRMatrix`. Ia membaca kedua salinan format, men-decode masing-masing terhadap 64 word miliknya dalam budget 3 bit, mengambil yang lebih dekat, dan mensyaratkan versi cocok dengan bentuk grid. Lalu ia melakukan unmask, membaca stream dari kolom kedua, melakukan deinterleave, mengoreksi setiap blok, dan mem-parsing segmen, termasuk ECI.
- **Gambar, nanti.** `Decode`, `DecodeBytes`, dan `DecodeAll` belum mencari simbol rMQR; finder plus finder sub-pattern di sudut seberang adalah milestone berikutnya.

## 6. Milestone

Milestone mendarat berurutan. **Checkpoint A** (setelah RQ5) me-round-trip setiap tipe rMQR dan cocok dengan zint simbol demi simbol. **Checkpoint B** (RQ6) adalah rilis `v0.17.0`.

### RQ1 — Plan Doc `(S)`

- [x] Dokumen ini dan padanan bahasa Inggrisnya.

### RQ2 — Dokumen Referensi `(S)`

- [x] `docs/theory/28-rmqr.md` (EN + ID): untuk apa rMQR dibuat; ke-32 versi beserta tabel codeword, blok, dan remainder-nya; mode dan lebar count per versi; terminator dan padding; error correction dan interleaving; tata letak, function pattern demi function pattern; format information dua salinan dengan kode BCH dan mask-nya; penempatan dari kolom kedua; mask tetap; rMQR dibanding QR dan Micro QR; dan bagaimana fakta-faktanya dicek. Plus entri README theory dan baris pemetaan kode.

### RQ3 — Tabel dan Bit Stream `(L)`

- [ ] `RMQRVersion`, tabel 32 versi (ukuran, total dan data codeword per level, jumlah blok, remainder bit, kolom alignment), dan tabel lebar count.
- [ ] Cost model rMQR untuk DP segmentasi; `encodeRMQRText`: pemilihan versi dan level, segmen (dengan ECI), terminator, padding, pembagian blok, Reed-Solomon, interleaving, dan remainder bit.
- [ ] Fungsi word BCH(18,6), diuji terhadap tabel version information QR (generator yang sama) dan terhadap 64 format word zint di tiap salinan.
- [ ] Test: data codeword dan stream akhir yang dibaca dari simbol zint, dan pilihan otomatis zint.

### RQ4 — Matriks Persegi Panjang, Tata Letak, dan Option `(L)`

- [ ] Jumlah baris dan kolom di `matrix`; renderer, `Matrix`, `applyMask`, dan `placeBits` memakainya, dengan output QR dan Micro tidak berubah.
- [ ] Tata letak rMQR, kedua salinan format, penempatan dari kolom kedua, dan mask tetap.
- [ ] Option di bagian 5 dan validasinya.
- [ ] Test golden: encoder kita yang dipaku ke versi dan level milik zint mereproduksi setiap simbol zint modul demi modul, dan pilihan otomatis kita cocok dengan pilihan zint.

### RQ5 — Decoder Matriks dan Round Trip `(M)`

- [ ] `decodeRMQRMatrix` dan dispatch `DecodeMatrix` berdasarkan bentuk.
- [ ] Test: decode setiap golden zint, round-trip encoder kita lintas versi, level, dan mode, koreksi error sampai kapasitas tiap blok, dan bertahan saat satu salinan format rusak.
- [ ] Cek sekali jalan, dilaporkan di commit: zxing-cpp membaca PNG rMQR yang kita render.

### Checkpoint A — setiap tipe rMQR bisa round-trip dan cocok dengan zint.

### RQ6 — Polish & Rilis `(S)`

- [ ] Flag CLI yang mencerminkan option, README (bagian encoding, tabel API, scope, limitations, roadmap), dan `examples/encode/rmqr`.
- [ ] Entri `v0.17.0` di `CHANGELOG.md`, dibiarkan unstaged untuk release branch.
- [ ] Baris peta kode theory untuk dokumen 28, tanpa tanda direncanakan.
- [ ] `go test -race ./...` bersih, gofmt bersih.

## 7. Strategi validasi

1. **Golden byte-exact dari zint.** Setiap versi di kedua level, dengan input single-mode yang tersebar di numeric, alphanumeric, byte, dan Kanji, plus input pendek dengan pad codeword, input yang terminator-nya jatuh tepat di batas codeword, dan input ECI, disimpan sebagai teks di `qrgen/testdata/thirdparty/rmqr_symbols.txt` di samping fixture QR dan Micro.
2. **Decoding.** `DecodeMatrix` membaca setiap fixture zint.
3. **Round trip** encoder kita sendiri; perlu, tetapi tidak pernah cukup dengan sendirinya.
4. **Decoder independen.** zxing-cpp membaca PNG yang kita render, dilaporkan di commit karena test Go tidak bisa memanggil Python.
5. **Format word.** Word BCH(18,6) hasil hitung dengan kedua mask sama dengan tabel 64 entri milik zint dan zxing-cpp.
6. **Pilihan otomatis.** Pilihan versi dan level kita cocok dengan pilihan zint untuk input di seluruh rentang kapasitas.

## 8. Usulan Delta Tata Letak File

```
qrgen/
├── rmqr.go                # baru — RMQRVersion, tabel versi dan lebar count
├── rmqr_encode.go         # baru — cost model, encodeRMQRText, blok, interleaving
├── rmqr_matrix.go         # baru — tata letak, format information, penempatan, mask
├── rmqr_decode.go         # baru — decodeRMQRMatrix
├── rmqr_test.go           # baru — tabel, BCH, bit stream, round trip
├── rmqr_golden_test.go    # baru — golden zint, decoding, pilihan otomatis
├── matrix.go, mask.go     # jumlah baris dan kolom; QR dan Micro tidak berubah
├── render_png.go, render_svg.go, render_terminal.go, api.go   # output persegi panjang
├── options.go, encode.go  # option rMQR, validasi, cabang buildMatrix
├── decode.go              # dispatch DecodeMatrix berdasarkan bentuk
└── testdata/thirdparty/rmqr_symbols.txt   # baru — fixture zint
cmd/qrgen/main.go          # flag rMQR
docs/
├── plan-rmqr.md / .id.md
└── theory/28-rmqr.md / .id.md
examples/encode/rmqr/main.go
```

## 9. Risiko & Catatan Teknis

- **Jebakan konsistensi diri, lagi.** Penempatan yang dimulai satu kolom meleset, atau salinan format yang ditulis dengan urutan salah, akan tetap round-trip lewat decoder kita sendiri dan gagal di tempat lain. Golden zint dan cek zxing-cpp adalah kriteria penerimaannya, dan kedua salinan format memakai tata letak yang berbeda, jadi masing-masing dicek sendiri.
- **Refactor persegi panjang menyentuh rendering QR dan Micro.** Fixture pihak ketiga QR dan Micro, serta setiap test renderer yang ada, harus tetap identik byte-for-byte.
- **Seri pada segmentasi.** Dua encoder optimal bisa memecah teks campuran secara berbeda; golden byte-exact memakai input single-mode, dan input campuran dicek lewat decoding.
- **Kebijakan otomatis punya langkah yang halus** (H kalau muat di versi yang dipilih untuk M); langkah itu dikunci lewat test terhadap pilihan zint, bukan lewat penalaran.
- **Dua kesalahan blok rmqrcode** mengingatkan bahwa transkripsi kedua adalah bukti, bukan kepastian; yang menopang tabel adalah kesepakatan zint dan zxing-cpp, sebuah encoder dan sebuah reader yang ditulis secara independen.

## 10. Referensi

- ISO/IEC 23941:2022 — spesifikasi rMQR; nomor tabelnya (Table 3 lebar count, Table 8 error correction, Table C.1 format information, Table D.1 kolom alignment) sebagaimana dikutip di source zint dan zxing-cpp.
- zint (backend/qr.c dan qr.h, BSD-3-Clause), lewat zint-bindings 1.2.2 — encoder golden.
- zxing-cpp (core/src/qrcode: QRVersion, QRFormatInformation, QRBitMatrixParser, QRCodecMode) — tabel, dan reader independen.
- rmqrcode 0.3.2 (MIT) — transkripsi ketiga, dengan dua penyimpangan blok di bagian 3.
- `docs/theory/26-micro-qr.md`, `05-matrix-construction.md`, `06-masking.md`, `07-format-version-info.md`, `17-optimal-segmentation.md`, `20-eci-segments.md` — padanan QR dan Micro yang diparalelkan milestone ini.

## 11. Pertanyaan Terbuka

- **API publik.** Usulannya adalah `WithRMQR`, `WithRMQRVersion` dengan konstanta yang dinamai sesuai standar (`R7x43` sampai `R17x139`), dan `WithRMQRHeight` untuk tinggi yang ditetapkan. Dikonfirmasi sebelum RQ4.
- **Salinan format yang tidak sepakat.** Usulannya mengambil yang lebih dekat dari dua word hasil decode; mensyaratkan keduanya sepakat akan lebih ketat tetapi membuang redundansi yang menjadi alasan salinan kedua ada. Diputuskan di RQ5 lewat test salinan yang rusak.
