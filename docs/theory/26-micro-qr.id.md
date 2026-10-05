# Micro QR Code

Micro QR Code adalah saudara kecil QR Code yang didefinisikan di standar yang sama, ISO/IEC 18004. Ia mempertahankan blok bangunan QR (mode data yang sama, Galois field dan kode Reed-Solomon yang sama, format word yang sama-sama dilindungi BCH, penempatan zigzag yang sama) tetapi mengubah parameternya di hampir setiap langkah, dan membuang bagian yang hanya bermanfaat pada simbol besar. Dokumen ini menelusuri tiap langkah, menyebutkan persis bedanya dengan QR, dan ditutup dengan contoh yang mengunci encoding-nya bit demi bit.

> Versi bahasa Inggris: [26-micro-qr.md](26-micro-qr.md).

## 1. Kenapa Micro QR ada

Simbol QR terkecil, version 1, berukuran 21 kali 21 modul dan butuh quiet zone 4 modul: total 29 kali 29. Itu luas yang besar untuk kode lima digit. Simbol Micro QR terkecil, M1, berukuran 11 kali 11 dengan quiet zone 2 modul, total 15 kali 15, sekitar seperempat luasnya, dan tetap membawa lima digit.

Penghematannya berasal dari membuang redundansi yang tidak dibutuhkan simbol kecil. Hanya ada satu finder pattern alih-alih tiga, jadi orientasi ditentukan dari finder itu plus timing pattern. Hanya ada satu salinan format information alih-alih dua, dan satu blok Reed-Solomon. Kompromi-kompromi berikutnya mengikuti pilihan yang sama: lebih sedikit level error correction (tidak ada H sama sekali, dan M1 hanya untuk *deteksi* error), kapasitas lebih kecil, serta tidak ada ECI, structured append, maupun FNC1.

## 2. Tipe simbol dan kapasitas

Simbol Micro version M*k* berukuran `2k + 9` modul persegi. Setiap kombinasi version dan level error correction punya **nomor simbol** 3 bit yang dibawa format information, menggantikan field version dan EC yang terpisah di QR.

| Nomor simbol | Tipe | Ukuran | Data codeword | EC codeword | Bit data | Numeric | Alphanumeric | Byte | Kanji |
|---|---|---|---|---|---|---|---|---|---|
| 0 | M1 | 11 | 3 (yang terakhir 4 bit) | 2 (hanya deteksi) | 20 | 5 | — | — | — |
| 1 | M2-L | 13 | 5 | 5 | 40 | 10 | 6 | — | — |
| 2 | M2-M | 13 | 4 | 6 | 32 | 8 | 5 | — | — |
| 3 | M3-L | 15 | 11 (yang terakhir 4 bit) | 6 | 84 | 23 | 14 | 9 | 6 |
| 4 | M3-M | 15 | 9 (yang terakhir 4 bit) | 8 | 68 | 18 | 11 | 7 | 4 |
| 5 | M4-L | 17 | 16 | 8 | 128 | 35 | 21 | 15 | 9 |
| 6 | M4-M | 17 | 14 | 10 | 112 | 30 | 18 | 13 | 8 |
| 7 | M4-Q | 17 | 10 | 14 | 80 | 21 | 13 | 9 | 5 |

Kapasitas karakter diturunkan dari bit data dan lebar header di bagian 3; kedua puluh lima nilainya sudah dicek terhadap batas zint.

## 3. Mode, indicator, dan character count

Micro QR mendukung keempat mode data milik QR, tetapi mode indicator-nya lebih pendek, dan tidak setiap version mendukung setiap mode.

| | M1 | M2 | M3 | M4 |
|---|---|---|---|---|
| Lebar mode indicator | 0 bit | 1 bit | 2 bit | 3 bit |
| Lebar count numeric | 3 | 4 | 5 | 6 |
| Lebar count alphanumeric | — | 3 | 4 | 5 |
| Lebar count byte | — | — | 4 | 5 |
| Lebar count Kanji | — | — | 3 | 4 |
| Terminator | 3 bit | 5 bit | 7 bit | 9 bit |

Nilai indicator-nya adalah numeric `0`, alphanumeric `1`, byte `2`, Kanji `3`, ditulis sesuai lebar indicator: satu bit milik M2 hanya bisa menyatakan numeric atau alphanumeric, persis himpunan mode yang didukung M2. M1 tidak punya indicator sama sekali, karena numeric adalah satu-satunya mode-nya. Encoding payload-nya sendiri tidak berubah dari QR: sepuluh bit per tiga digit, sebelas bit per dua karakter alphanumeric, delapan bit per byte, tiga belas bit per karakter Kanji (dokumen 02 dan 22).

Terminator adalah deretan bit nol yang panjangnya persis sama dengan mode indicator plus field count numeric (3, 5, 7, 9 = 0+3, 1+4, 2+5, 3+6). Karena nilai indicator numeric adalah `0`, reader yang menemuinya mem-parse "mode numeric, nol karakter" lalu berhenti. Seperti milik QR, terminator dipotong kalau kapasitas habis lebih dulu.

## 4. Dari bit ke codeword

Setelah segmen terakhir dan terminator, bit stream diperpanjang untuk mengisi kapasitas data dalam dua langkah, seperti di QR:

1. **Padding bit.** Bit nol sampai batas codeword berikutnya. Kalau stream sudah berakhir tepat di batas, tidak ada yang ditambahkan.
2. **Pad codeword.** `11101100` dan `00010001`, bergantian, mengisi data codeword yang tersisa.

Bedanya ada pada **half codeword**. Di M1 dan M3, kapasitas data (20, 84, atau 68 bit) bukan kelipatan 8, dan data codeword terakhir hanya sepanjang 4 bit. Batas codeword-nya karena itu berada 4 bit setelah batas sebelumnya, dan ketika posisi itu adalah posisi pad, ia diisi `0000`, bukan pad codeword.

Dua encoder independen berbeda pendapat persis pada aturan ini, dan itu peringatan yang berguna. segno mengisi seluruh padding M1 dan M3 dengan nol, dan menambahkan delapan bit nol bahkan ketika terminator sudah berakhir tepat di batas codeword. zint mengikuti aturan di atas. Decoder mengabaikan bit pad, jadi keduanya menghasilkan simbol yang bisa dipindai; tetapi hanya satu yang sesuai standar, dan encoder yang diuji terhadap yang lain akan tampak salah padahal benar.

## 5. Error correction

Setiap tipe Micro memakai satu blok Reed-Solomon di atas GF(256) yang sama dengan QR (primitive polynomial `0x11D`, dokumen 03 dan 04), dengan jumlah EC codeword sesuai tabel di bagian 2. Dengan satu blok, tidak ada interleaving.

Untuk half codeword, Reed-Solomon melihat satu byte penuh: 4 bit data di nibble atas dan nol di nibble bawah. Hanya 4 bit data yang ditempatkan di simbol. Bit stream akhirnya adalah data codeword (half codeword dihitung 4 bit) diikuti EC codeword: 36, 80, 132, atau 192 bit untuk M1 sampai M4, persis sejumlah modul data, jadi simbol Micro tidak pernah butuh remainder bit.

Dua EC codeword milik M1 hanya untuk **deteksi error**. Secara prinsip dua check symbol bisa mengoreksi satu error, tetapi setelah itu tidak ada sisa untuk mendeteksi koreksi yang keliru, jadi standar menetapkan M1 hanya untuk deteksi: decoder menolak simbol M1 yang syndrome-nya tidak semua nol, alih-alih mengoreksinya.

## 6. Tata letak simbol

- **Finder.** Satu finder pattern 7 kali 7 di sudut kiri-atas, dengan separator hanya di sisi kanan (kolom 7) dan bawah (baris 7), karena dua sisi lainnya adalah tepi simbol: total blok 8 kali 8.
- **Timing.** Di baris 0 dan kolom 0, dari indeks 8 sampai tepi, gelap pada indeks genap. Di QR, timing pattern berjalan di baris dan kolom 6, di antara finder; Micro tidak punya finder kedua untuk diapit, jadi timing pindah ke tepi luar.
- **Format information.** 15 modul berbentuk L tepat di luar separator: kolom 8 baris 1 sampai 8, dan baris 8 kolom 1 sampai 7 (bagian 7).
- **Tidak ada** alignment pattern, version information, maupun dark module.

Function pattern memakan `63 + 2n` modul dari simbol berukuran `n` modul (64 untuk blok finder, `2(n − 8)` untuk timing, 15 untuk format), sehingga tersisa `n² − 2n − 63` modul data: 36, 80, 132, dan 192.

## 7. Format information

Payload 5 bit-nya adalah nomor simbol 3 bit diikuti nomor mask 2 bit. Ia dilindungi persis seperti di QR, kode BCH(15,5) dengan generator `x¹⁰ + x⁸ + x⁵ + x⁴ + x² + x + 1` (`0x537`), lalu di-XOR dengan `0x4445` alih-alih `0x5412` milik QR.

| Simbol | Mask 0 | Mask 1 | Mask 2 | Mask 3 |
|---|---|---|---|---|
| M1 | `4445` | `4172` | `4E2B` | `4B1C` |
| M2-L | `55AE` | `5099` | `5FC0` | `5AF7` |
| M2-M | `6793` | `62A4` | `6DFD` | `68CA` |
| M3-L | `7678` | `734F` | `7C16` | `7921` |
| M3-M | `06DE` | `03E9` | `0CB0` | `0987` |
| M4-L | `1735` | `1202` | `1D5B` | `186C` |
| M4-M | `2508` | `203F` | `2F66` | `2A51` |
| M4-Q | `34E3` | `31D4` | `3E8D` | `3BBA` |

Hanya ada satu salinan. Dengan bit 0 sebagai yang paling tidak signifikan:

| Bit | 0–7 | 8–14 |
|---|---|---|
| Modul `(baris, kolom)` | `(i + 1, 8)` | `(8, 15 − i)` |

Bit 0 sampai 7 turun di kolom 8 dari baris 1 sampai baris 8, dan bit 8 sampai 14 bergerak ke kiri di baris 8 sehingga bit 14 jatuh di `(8, 1)`. Ini adalah bentuk L yang sama dengan salinan pertama QR (dokumen 07), dengan sudut yang sama: bit 7 di `(8, 8)` dan bit 8 di `(8, 7)`. Bedanya ada dua. Ujung luarnya berhenti satu modul lebih awal, di baris 1 dan kolom 1, karena baris 0 dan kolom 0 berisi timing pattern Micro. Dan tidak ada celah yang perlu dilompati, karena tidak ada timing pattern di baris atau kolom 6. Orientasi ini mudah terbalik dan tetap lolos setiap round trip buatan sendiri, seperti yang terjadi pada kode QR library ini sampai v0.14.1; dengan hanya satu salinan, simbol Micro yang ditulis seperti itu tak akan terbaca oleh reader lain mana pun.

Decoder tidak punya salinan kedua sebagai cadangan, jadi ia bisa memakai seluruh budget BCH sebesar 3 error untuk salinan yang satu ini. Budget itu membuat satu cek tambahan menjadi wajib: word dengan bobot terendah di tabel, M4-L dengan mask 1 (`1202`), hanya punya 3 bit bernilai satu, sehingga strip format yang terbaca serba terang masih dalam budget dan ter-decode sebagai M4-L. Decoder harus mensyaratkan nomor simbol cocok dengan ukuran grid sebelum mempercayainya.

## 8. Penempatan data

Penempatan data mengikuti zigzag QR: bit stream mengisi kolom selebar dua modul, mulai dari kanan-bawah dan bergerak ke atas, berbalik arah di setiap tepi, dan melewati function module. Micro membuang satu detail: QR melompati kolom 6 karena timing pattern vertikalnya ada di sana, tetapi kolom 6 Micro di bawah finder berisi data, dan pasangan kolom cukup berjalan dari tepi kanan sampai kolom 1, menyisakan kolom 0 untuk timing pattern. Half codeword tidak butuh penanganan khusus di tahap ini; stream ditempatkan bit demi bit, apa pun batas codeword-nya.

## 9. Masking

Micro memakai empat dari delapan pola mask QR, diterapkan hanya pada modul data. Dengan `i` sebagai baris dan `j` sebagai kolom:

| Mask Micro | Mask QR | Modul dibalik ketika |
|---|---|---|
| 0 | 1 | `i mod 2 = 0` |
| 1 | 4 | `(⌊i / 2⌋ + ⌊j / 3⌋) mod 2 = 0` |
| 2 | 6 | `((i·j) mod 2 + (i·j) mod 3) mod 2 = 0` |
| 3 | 7 | `((i + j) mod 2 + (i·j) mod 3) mod 2 = 0` |

Aturan pemilihannya sama sekali berbeda dari empat aturan penalti QR (dokumen 06). Hitung modul gelap di kolom terakhir (SUM1) dan di baris terakhir (SUM2), masing-masing tanpa modul timing di indeks 0. Skornya `SUM1 × 16 + SUM2` ketika `SUM1 ≤ SUM2`, dan `SUM2 × 16 + SUM1` selain itu, dan skor **tertinggi** yang menang. Dua tepi yang dinilai adalah tepi yang tidak punya finder maupun timing pattern, dan mengalikan jumlah yang lebih kecil dengan 16 membuat tepi dengan modul gelap lebih sedikit mendominasi skor; tafsiran yang masuk akal adalah aturan ini menjaga batas jauh simbol tetap terlihat oleh reader. Kalau seri, segno mempertahankan mask dengan nomor terendah. Tidak seperti di QR, format information tidak pernah menyentuh tepi yang dinilai, jadi ia tidak ikut menentukan pilihan.

## 10. Micro versus QR sekilas

| Langkah | QR Code | Micro QR Code |
|---|---|---|
| Ukuran | 21 sampai 177, langkah 4 | 11, 13, 15, 17 |
| Quiet zone | 4 modul | 2 modul |
| Finder pattern | 3 | 1 |
| Timing | baris 6, kolom 6 | baris 0, kolom 0 |
| Mode indicator | 4 bit | 0 sampai 3 bit |
| Terminator | 4 bit | 3, 5, 7, 9 bit |
| Codeword | semua 8 bit | data codeword terakhir 4 bit di M1, M3 |
| Blok RS | 1 sampai 81, di-interleave | 1 |
| Level EC | L, M, Q, H | M1 hanya deteksi; L, M (M2, M3); L, M, Q (M4) |
| Format information | 2 salinan, mask `0x5412` | 1 salinan, mask `0x4445`, nomor simbol |
| Version information | 18 bit untuk v7+ | tidak ada |
| Mask | 8, penalti terendah | 4, skor tepi tertinggi |
| ECI, structured append, FNC1 | ya | tidak |

## 11. Contoh kerja

Setiap bit stream di bawah diturunkan dengan tangan dari aturan di atas dan cocok persis dengan simbol zint. Spasi memisahkan field; `|` menandai batas codeword di baris terakhir.

**M1, `"12345"`: kapasitas penuh, jadi tanpa terminator.**
Count `101` (5), digit `123` → `0001111011`, digit `45` → `0101101`: 3 + 10 + 7 = 20 bit, persis kapasitas M1.
`10100011 | 11011010 | 1101` (yang terakhir adalah half codeword).

**M1, `"1"`: padding bit berlanjut sampai half codeword.**
Count `001`, digit `1` → `0001`, terminator `000`: 10 bit; padding bit sampai batas codeword di bit 16; lalu half codeword menjadi posisi pad, `0000`.
`00100010 | 00000000 | 0000`.

**M2-L, `"0123456789"`: terminator dipotong jadi satu bit.**
Indicator `0`, count `1010` (10), `012` → `0000001100`, `345` → `0101011001`, `678` → `1010100110`, `9` → `1001`: 39 bit, kurang satu dari kapasitas 40 bit, jadi terminator 5 bit menyusut menjadi `0`.
`01010000 | 00011000 | 10101100 | 11010100 | 11010010`.

**M4-Q, `"1234"`: terminator berakhir tepat di batas codeword.**
Indicator `000`, count `000100` (4), `123` → `0001111011`, `4` → `0100`, terminator `000000000`: 32 bit, empat codeword utuh, jadi tanpa padding bit; enam pad codeword menyusul.
`00000010 | 00001111 | 01101000 | 00000000 | 11101100 | 00010001 | 11101100 | 00010001 | 11101100 | 00010001`. segno menyisipkan `00000000` tambahan sebelum pad codeword di sini.

**M3-M, `"HELLO WORLD"`: berakhir di half codeword yang membawa data.**
Indicator `01`, count `1011` (11), pasangan `HE`, `LL`, `O ` (O dan spasi), `WO`, `RL` sebagai nilai 11 bit dan `D` terakhir sebagai 6 bit: 67 dari kapasitas 68 bit, jadi terminatornya satu `0`.
`01101101 | 10000101 | 10111100 | 01101000 | 10111001 | 01101110 | 00100110 | 10100001 | 1010`.

**M2-L, `"01234567"`, contoh dari standar itu sendiri, lengkap dengan error correction.**
Indicator `0`, count `1000` (8), `012`, `345`, `67` → 32 bit, terminator `00000`, padding `000` sampai kapasitas 40 bit.
Data codeword `40 18 AC C3 00`; Reed-Solomon di atas GF(256) memberi lima EC codeword `86 0D 22 AE 30`.

## 12. Bagaimana fakta-fakta ini dicek

Tabel berasal dari standar sebagaimana ditranskripsikan di segno 1.6.6, dan dicek silang terhadap zint (diakses lewat zxing-cpp 2.3.0) pada 69 simbol: 57 identik byte-for-byte, 12 sisanya hanya berbeda di padding, tempat segno menyimpang dari standar seperti dijelaskan di bagian 4. Implementasi Reed-Solomon yang independen mereproduksi EC codeword zint untuk 20 simbol yang mencakup kedelapan tipe, dan itu mengunci konvensi half codeword. Kapasitas karakter di bagian 2 cocok dengan batas zint di kedua puluh lima kasus, dan tabel format di bagian 7 cocok baik dengan perhitungan BCH maupun tabel precomputed milik segno. Pelajaran dari bug format information yang diperbaiki di v0.14.1 berlaku di sini dengan bobot lebih besar: dengan satu salinan format, satu-satunya cek yang bermakna adalah kesesuaian dengan implementasi yang tidak berbagi satu baris kode pun dengan kita.

## 13. Penunjuk implementasi

Direncanakan di MQ3 sampai MQ5 (lihat `docs/plan-micro-qr.md`):

- `qrgen/micro.go`: `MicroVersion`, tabel tipe simbol, dan tabel lebar bit.
- `qrgen/micro_encode.go`: cost model Micro untuk DP segmentasi, `encodeMicroText`, padding dengan half codeword, dan stream data-plus-EC yang bit-exact.
- `qrgen/micro_matrix.go`: tata letak, writer format satu salinan, dan pemilihan empat mask.
- `qrgen/micro_decode.go`: `decodeMicroMatrix`, dijangkau dari `DecodeMatrix` berdasarkan ukuran grid.
- `qrgen/testdata/thirdparty/micro_symbols.txt`: simbol zint untuk test byte-exact dan decoding.

## Referensi

- ISO/IEC 18004:2015 — karakteristik simbol Micro QR Code, encoding data, error correction, tata letak, format information, dan masking.
- segno 1.6.6, `consts.py` dan `encoder.py` — transkripsi tabel, dengan penyimpangan padding yang dicatat di bagian 4.
- zint, lewat zxing-cpp 2.3.0 — encoder referensi untuk contoh kerja dan golden; reader zxing-cpp sebagai decoder independen.
- `docs/theory/02-data-encoding.md`, `04-reed-solomon.md`, `05-matrix-construction.md`, `06-masking.md`, `07-format-version-info.md`, `22-kanji-mode.md` — padanan QR untuk tiap langkah.
