# rMQR Code

rMQR, Rectangular Micro QR Code dari ISO/IEC 23941:2022, adalah anggota ketiga keluarga QR. Ia mempertahankan mode data, kode Reed-Solomon, dan penempatan zigzag milik QR serta quiet zone kecil milik Micro QR, tetapi meninggalkan bentuk persegi: simbol rMQR setinggi 7 sampai 17 modul dan selebar 27 sampai 139 modul. Dokumen ini menelusuri tiap langkah seperti dokumen 26 untuk Micro QR, menyebutkan bedanya dengan kedua kerabatnya, dan ditutup dengan contoh kerja yang dicek terhadap zint.

> Versi bahasa Inggris: [28-rmqr.md](28-rmqr.md).

## 1. Kenapa rMQR ada

QR dan Micro QR berbentuk persegi, jadi di ruang yang sempit sebuah simbol persegi harus mengecil sampai tingginya tidak melebihi ruang itu, dan kapasitasnya ikut mengecil. rMQR mempertahankan tinggi yang kecil dan memperbesar lebarnya. R7x139 memuat 44 data codeword pada level M di sebuah strip setinggi 7 modul; simbol Micro QR terbesar memuat 16 di persegi 17 kali 17. Simbol yang panjang dan tipis butuh cara untuk menemukan kedua ujungnya, jadi rMQR menambahkan finder sub-pattern di sudut seberang dan corner pattern kecil di dua sudut lainnya, serta menyebarkan timing di sepanjang setiap tepi.

## 2. Versi dan kapasitas

Ada 32 versi, dinamai R*h*x*w* sesuai tinggi dan lebarnya dalam modul: tinggi 7, 9, 11, 13, 15, dan 17, lebar 43, 59, 77, 99, dan 139, serta lebar 27 untuk tinggi 11 dan 13. Versi diberi nomor 1 sampai 32 sesuai urutan di bawah, dan format information membawa nomor itu dikurangi satu. Setiap versi punya dua level error correction, M dan H; tidak ada L maupun Q.

| No. | Versi | Codeword | M: data / blok | H: data / blok | Remainder bit | Lebar count N / A / B / K |
|---|---|---|---|---|---|---|
| 1 | R7x43 | 13 | 6 / 1 | 3 / 1 | 0 | 4 / 3 / 3 / 2 |
| 2 | R7x59 | 21 | 12 / 1 | 7 / 1 | 3 | 5 / 5 / 4 / 3 |
| 3 | R7x77 | 32 | 20 / 1 | 10 / 1 | 5 | 6 / 5 / 5 / 4 |
| 4 | R7x99 | 44 | 28 / 1 | 14 / 1 | 6 | 7 / 6 / 5 / 5 |
| 5 | R7x139 | 68 | 44 / 1 | 24 / 2 | 1 | 7 / 6 / 6 / 5 |
| 6 | R9x43 | 21 | 12 / 1 | 7 / 1 | 2 | 5 / 5 / 4 / 3 |
| 7 | R9x59 | 33 | 21 / 1 | 11 / 1 | 3 | 6 / 5 / 5 / 4 |
| 8 | R9x77 | 49 | 31 / 1 | 17 / 2 | 1 | 7 / 6 / 5 / 5 |
| 9 | R9x99 | 66 | 42 / 1 | 22 / 2 | 4 | 7 / 6 / 6 / 5 |
| 10 | R9x139 | 99 | 63 / 2 | 33 / 3 | 5 | 8 / 7 / 6 / 6 |
| 11 | R11x27 | 15 | 7 / 1 | 5 / 1 | 2 | 4 / 4 / 3 / 2 |
| 12 | R11x43 | 31 | 19 / 1 | 11 / 1 | 1 | 6 / 5 / 5 / 4 |
| 13 | R11x59 | 47 | 31 / 1 | 15 / 2 | 0 | 7 / 6 / 5 / 5 |
| 14 | R11x77 | 67 | 43 / 1 | 23 / 2 | 2 | 7 / 6 / 6 / 5 |
| 15 | R11x99 | 89 | 57 / 2 | 29 / 2 | 7 | 8 / 7 / 6 / 6 |
| 16 | R11x139 | 132 | 84 / 2 | 42 / 3 | 6 | 8 / 7 / 7 / 6 |
| 17 | R13x27 | 21 | 12 / 1 | 7 / 1 | 4 | 5 / 5 / 4 / 3 |
| 18 | R13x43 | 41 | 27 / 1 | 13 / 1 | 1 | 6 / 6 / 5 / 5 |
| 19 | R13x59 | 60 | 38 / 1 | 20 / 2 | 6 | 7 / 6 / 6 / 5 |
| 20 | R13x77 | 85 | 53 / 2 | 29 / 2 | 4 | 7 / 7 / 6 / 6 |
| 21 | R13x99 | 113 | 73 / 2 | 35 / 3 | 3 | 8 / 7 / 7 / 6 |
| 22 | R13x139 | 166 | 106 / 3 | 54 / 4 | 0 | 8 / 8 / 7 / 7 |
| 23 | R15x43 | 51 | 33 / 1 | 15 / 2 | 1 | 7 / 6 / 6 / 5 |
| 24 | R15x59 | 74 | 48 / 1 | 26 / 2 | 4 | 7 / 7 / 6 / 5 |
| 25 | R15x77 | 103 | 67 / 2 | 31 / 3 | 6 | 8 / 7 / 7 / 6 |
| 26 | R15x99 | 136 | 88 / 2 | 48 / 4 | 7 | 8 / 7 / 7 / 6 |
| 27 | R15x139 | 199 | 127 / 3 | 69 / 5 | 2 | 9 / 8 / 7 / 7 |
| 28 | R17x43 | 61 | 39 / 1 | 21 / 2 | 1 | 7 / 6 / 6 / 5 |
| 29 | R17x59 | 88 | 56 / 2 | 28 / 2 | 2 | 8 / 7 / 6 / 6 |
| 30 | R17x77 | 122 | 78 / 2 | 38 / 3 | 0 | 8 / 7 / 7 / 6 |
| 31 | R17x99 | 160 | 100 / 3 | 56 / 4 | 3 | 8 / 8 / 7 / 6 |
| 32 | R17x139 | 232 | 152 / 4 | 76 / 6 | 4 | 9 / 8 / 8 / 7 |

Kolom codeword menghitung semua codeword di simbol; EC codeword adalah codeword dikurangi data codeword, dibagi rata ke semua blok. Remainder bit adalah modul data yang tersisa setelah codeword terakhir (bagian 8), dan lebar count adalah lebar field character count untuk numeric, alphanumeric, byte, dan Kanji (bagian 3).

## 3. Mode, indicator, dan character count

Setiap versi rMQR memakai mode indicator 3 bit:

| Indicator | Mode |
|---|---|
| `000` | terminator |
| `001` | numeric |
| `010` | alphanumeric |
| `011` | byte |
| `100` | Kanji |
| `101`, `110` | FNC1 di posisi pertama dan kedua |
| `111` | ECI |

Encoding payload-nya sama dengan QR: sepuluh bit per tiga digit, sebelas bit per dua karakter alphanumeric, delapan bit per byte, tiga belas bit per karakter Kanji (dokumen 02 dan 22). Header ECI adalah indicator `111` diikuti designator 8, 16, atau 24 bit yang sama seperti di QR (dokumen 20).

Lebar character count diberikan per versi di tabel bagian 2. Seperti Micro QR dan berbeda dari QR, lebarnya berubah di setiap versi, bukan per kelompok versi, dan mengikuti kapasitasnya: R7x43 menghitung digit dalam 4 bit, sedangkan R15x139 dan R17x139 memakai 9.

## 4. Dari bit ke codeword

Setelah segmen terakhir ada terminator 3 bit, yang dipotong kalau kapasitas habis lebih dulu. Bit nol lalu mengisi sampai batas codeword berikutnya, dan pad codeword `11101100` dan `00010001` bergantian mengisi data codeword yang tersisa, persis seperti QR. Setiap codeword berukuran penuh 8 bit; rMQR tidak punya padanan codeword akhir 4 bit milik M1 dan M3 pada Micro QR.

## 5. Error correction

Kode Reed-Solomon-nya milik QR: GF(256) dengan polinomial primitif `0x11D` (dokumen 03 dan 04). Simbol kecil memakai satu blok; yang terbesar, R17x139 pada level H, memakai enam. Kalau data codeword tidak terbagi rata, blok-blok pertama satu codeword lebih pendek, dan blok-blok itu di-interleave seperti QR: data codeword pertama dari setiap blok, lalu yang kedua, dan seterusnya, dengan blok yang lebih panjang menyumbang codeword terakhirnya di ujung, diikuti EC codeword dengan urutan kolom yang sama. R9x77 pada level H, misalnya, membagi 17 data codeword-nya menjadi blok berisi 8 dan 9 lalu mengirim D1, D9, D2, D10, …, D8, D16, D17.

## 6. Tata letak simbol

- **Finder pattern.** Finder 7 kali 7 milik QR di sudut kiri-atas, dengan separator terang di kolom 7 dan, mulai tinggi 9, di baris 7. Pada simbol R7, finder mengisi seluruh tinggi dan tidak ada separator di bawahnya.
- **Finder sub-pattern.** Pola 5 kali 5 di sudut kanan-bawah: cincin gelap, cincin terang, dan satu modul pusat yang gelap. Pola ini menandai ujung jauh simbol.
- **Corner finder pattern.** Di kanan-atas, modul (0, w - 2), (0, w - 1), dan (1, w - 1) gelap mengelilingi (1, w - 2) yang terang. Di kiri-bawah, modul (h - 1, 0), (h - 1, 1), dan (h - 1, 2) gelap, dan mulai tinggi 11, begitu juga (h - 2, 0), di samping (h - 2, 1) yang terang.
- **Timing pattern.** Baris 0 dan baris h - 1 gelap pada kolom genap, dan kolom 0 dan w - 1 gelap pada baris genap, di mana pun tidak ada pola lain.
- **Alignment pattern.** Pola 3 kali 3, cincin gelap mengelilingi pusat yang terang, di tepi atas (baris 0 sampai 2) dan tepi bawah (baris h - 3 sampai h - 1) setiap kolom alignment, dihubungkan timing pattern vertikal yang gelap pada baris genap. Kolomnya hanya bergantung pada lebar: 21 untuk lebar 43; 19 dan 39 untuk 59; 25 dan 51 untuk 77; 23, 49, dan 75 untuk 99; 27, 55, 83, dan 111 untuk 139; tidak ada untuk 27.
- **Format information.** Dua area 18 modul, di samping finder dan di samping sub-pattern (bagian 7).

R11x43, dengan F finder, s separatornya, S sub-pattern, C corner pattern, T timing, A alignment, f format information, dan `.` data:

```
FFFFFFFsTTTTTTTTTTTTAAATTTTTTTTTTTTTTTTTTCC
FFFFFFFsffff........AAA..................CC
FFFFFFFsffff........AAA...................T
FFFFFFFsffff.........T....................T
FFFFFFFsfff..........T....................T
FFFFFFFsfff..........T.............ffffff.T
FFFFFFFs.............T.............fffSSSSS
ssssssss.............T.............fffSSSSS
T...................AAA............fffSSSSS
CC..................AAA............fffSSSSS
CCCTTTTTTTTTTTTTTTTTAAATTTTTTTTTTTTTTTSSSSS
```

## 7. Format information

Format information terdiri dari 6 bit data: level di bit teratas (0 untuk M, 1 untuk H) di atas nomor versi dikurangi satu sepanjang 5 bit. Bit-bit itu dilindungi BCH(18,6) dengan generator `x¹² + x¹¹ + x¹⁰ + x⁹ + x⁸ + x⁵ + x² + 1` (`0x1F25`), kode yang sama yang melindungi version information QR (dokumen 07); 12 bit pemeriksa mengikuti 6 bit data. Jarak minimum kode ini 8, jadi sebuah salinan tetap bisa di-decode dengan sampai tiga bit error.

Ada dua salinan, masing-masing di-XOR dengan mask-nya sendiri: `0x1FAB2` untuk salinan di samping finder dan `0x20A7B` untuk salinan di samping sub-pattern. Dengan bit 0 sebagai yang paling tidak signifikan dan `i` sebagai indeks bit:

| Salinan | Bit 0–14 di `(baris, kolom)` | Bit 15, 16, 17 di |
|---|---|---|
| Di samping finder | `(1 + i mod 5, 8 + ⌊i / 5⌋)` | `(1, 11)`, `(2, 11)`, `(3, 11)` |
| Di samping sub-pattern | `(h − 6 + i mod 5, w − 8 + ⌊i / 5⌋)` | `(h − 6, w − 5)`, `(h − 6, w − 4)`, `(h − 6, w − 3)` |

Kedua salinan diisi kolom demi kolom, lima bit per kolom dari atas, dan tiga bit terakhir berjalan di sepanjang satu baris atau kolom: turun di kolom 11 di samping finder, dan menyusuri baris h - 6 di samping sub-pattern. Dibanding kerabatnya, format information rMQR membawa versi, yang di QR disimpan terpisah di version information dan di Micro QR dilebur ke nomor simbol, dan sama sekali tidak membawa mask (bagian 9).

## 8. Penempatan data

Penempatannya adalah zigzag milik QR: kolom selebar dua modul yang diisi dari bawah, ke atas lebih dulu, berbalik arah di setiap tepi, dan melompati modul fungsi. rMQR mengubah titik mulainya. Kolom paling kanan hanya berisi modul timing, corner, dan sub-pattern, jadi pasangan kolom pertama adalah w - 2 dan w - 3, dan pasangan berikutnya terus ke kiri sampai kolom 1 dan 0, tanpa ada kolom yang dilompati. Memulai dari kolom paling kanan, seperti QR, akan memasangkan setiap kolom dengan tetangga yang salah dan tetap round-trip lewat decoder yang membuat kesalahan yang sama.

Setelah codeword terakhir, 0 sampai 7 modul data yang tersisa adalah remainder bit, bernilai nol sebelum masking.

## 9. Masking

rMQR punya satu mask saja, mask 4 milik QR: dengan `i` sebagai baris dan `j` sebagai kolom, modul data atau remainder dibalik kalau `(⌊i / 2⌋ + ⌊j / 3⌋) mod 2 = 0`. Tidak ada pemilihan mask dan tidak ada nomor mask di format information.

## 10. rMQR, QR, dan Micro QR sekilas

| Langkah | QR Code | Micro QR Code | rMQR Code |
|---|---|---|---|
| Bentuk | persegi, 21 sampai 177 | persegi, 11 sampai 17 | persegi panjang, 7 sampai 17 kali 27 sampai 139 |
| Versi | 40 | 4 | 32 |
| Quiet zone | 4 modul | 2 modul | 2 modul |
| Pola penemu | 3 finder | 1 finder | 1 finder, 1 sub-pattern, 2 corner pattern |
| Timing | baris 6, kolom 6 | baris 0, kolom 0 | keempat tepi, dan di sepanjang setiap kolom alignment |
| Alignment | grid, mulai versi 2 | tidak ada | tepi atas dan bawah, di kolom tetap |
| Mode indicator | 4 bit | 0 sampai 3 bit | 3 bit |
| Lebar count | per kelompok versi | per versi | per versi |
| Terminator | 4 bit | 3, 5, 7, 9 bit | 3 bit |
| Level EC | L, M, Q, H | deteksi (M1); L, M; L, M, Q (M4) | M, H |
| Blok RS | 1 sampai 81 | 1 | 1 sampai 6 |
| Format information | 15 bit, 2 salinan: level, mask | 15 bit, 1 salinan: nomor simbol, mask | 18 bit, 2 salinan: level, versi |
| Version information | 18 bit mulai versi 7 | tidak ada | tidak ada |
| Mask | 8, penalti terendah | 4, skor tepi tertinggi | 1, tetap |
| ECI | ya | tidak | ya |
| Structured append | ya | tidak | tidak |

## 11. Contoh kerja

Setiap stream di bawah diturunkan dengan tangan dari aturan di atas dan cocok persis dengan simbol zint, termasuk EC codeword-nya. Spasi memisahkan field.

**R7x43 pada level H, `"123"`: simbol terkecil.**
Indicator `001`, count `0011` (3), `123` → `0001111011`, terminator `000`: 20 bit, lalu empat bit padding sampai kapasitas 24 bit, tepat tiga codeword.
Data `26 3D 80`; error correction `A9 7F E5 56 03 84 46 AC 4C DF`. Tidak ada remainder bit.

**R11x27 pada level M, `"HELLO"`: pad codeword dan remainder bit.**
Indicator `010`, count `0101` (5), `HE` → `01100001011`, `LL` → `01111000110`, `O` → `011000`, terminator `000`: 38 bit, padding `00` sampai 40, lalu dua pad codeword untuk mengisi tujuh data codeword.
Data `4A C2 DE 33 00 EC 11`; error correction `F8 99 00 E4 B0 CB 71 17`; dua remainder bit.

**R11x27 pada level M, `"12345678901234"`: terminator yang terpotong.**
Indicator `001`, count `1110` (14), `123`, `456`, `789`, `012` → empat grup 10 bit, `34` → `0100010`: 54 bit dari kapasitas 56 bit, jadi terminator menyusut menjadi `00`.
Data `3C 3D B9 18 A8 18 88`; error correction `43 4E CF F5 DB F2 66 E0`.

## 12. Bagaimana fakta-fakta ini dicek

ISO/IEC 23941:2022 tidak dirujuk secara langsung. Fakta-faktanya berasal dari tiga implementasi yang tidak berbagi kode, dan masing-masing dicek terhadap yang lain alih-alih dipercaya begitu saja. zint (lewat zint-bindings 1.2.2) dan zxing-cpp, sebuah encoder dan sebuah reader yang ditulis secara independen, sepakat di setiap ukuran, jumlah codeword, struktur blok, lebar count, dan kolom alignment. Yang ketiga, rmqrcode 0.3.2, sepakat dengan keduanya kecuali di dua definisi blok, R13x27 pada M dan R17x43 pada M, tempat ia keliru. Ke-64 format word di tiap salinan dihitung ulang dari generator BCH dan kedua mask, dan cocok dengan tabel milik zint maupun zxing-cpp.

Tata letak di bagian 6 dibangun sebagai model dan dibandingkan dengan simbol zint untuk ke-32 versi di kedua level: setiap modul fungsi dan kedua salinan format cocok. Membaca kembali ke-64 simbol itu lewat model, dengan penempatan di bagian 8 dan mask di bagian 9, menghasilkan data codeword yang identik dengan encoding menurut aturan di bagian 3 dan 4, dan EC codeword yang identik dengan implementasi Reed-Solomon yang independen. Sama seperti Micro QR, kesepakatan dengan implementasi yang tidak berbagi kode sedikit pun dengan kita adalah satu-satunya cek yang bermakna; round trip lewat encoder dan decoder kita sendiri akan tetap lolos meskipun penempatan dimulai meleset satu kolom.

## 13. Penunjuk implementasi

Direncanakan di RQ3 sampai RQ5 (lihat `docs/plan-rmqr.md`):

- `qrgen/rmqr.go`: `RMQRVersion` dan tabel di bagian 2.
- `qrgen/rmqr_encode.go`: cost model untuk DP segmentasi, `encodeRMQRText`, padding, dan interleaving blok.
- `qrgen/rmqr_matrix.go`: tata letak, kedua salinan format, penempatan, dan mask.
- `qrgen/rmqr_decode.go`: `decodeRMQRMatrix`, yang dicapai dari `DecodeMatrix` berdasarkan bentuk grid.
- `qrgen/testdata/thirdparty/rmqr_symbols.txt`: simbol zint untuk test byte-exact dan decoding.

## Referensi

- ISO/IEC 23941:2022 — spesifikasi simbologi rMQR Code; Table 3 (lebar count), Table 8 (error correction), Table C.1 (format information), dan Table D.1 (kolom alignment) sebagaimana dikutip di source zint dan zxing-cpp.
- zint, backend/qr.c dan qr.h (BSD-3-Clause), lewat zint-bindings 1.2.2 — encoder referensi untuk contoh kerja dan golden.
- zxing-cpp, core/src/qrcode (QRVersion, QRFormatInformation, QRBitMatrixParser, QRCodecMode) — transkripsi kedua dari tabel-tabelnya, dan reader independen.
- rmqrcode 0.3.2 — transkripsi ketiga, dengan dua penyimpangan blok di bagian 12.
- `docs/theory/02-data-encoding.md`, `04-reed-solomon.md`, `05-matrix-construction.md`, `06-masking.md`, `07-format-version-info.md`, `20-eci-segments.md`, `26-micro-qr.md` — padanan QR dan Micro QR untuk tiap langkah.
