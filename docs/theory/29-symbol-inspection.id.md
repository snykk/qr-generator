# Inspeksi Simbol

Dokumen 5 sampai 7, 26, dan 28 menjelaskan cara sebuah simbol dibangun: function pattern lebih dulu, lalu codeword ditempatkan di sekelilingnya, lalu mask, lalu format information. Grid akhirnya menyembunyikan langkah-langkah itu; sebuah modul gelap bisa jadi bagian dari finder, bit format, atau bit data yang dibalik oleh mask. Dokumen ini membahas cara membedah lagi simbol yang sudah jadi: apa yang dilaporkan `qrgen.Inspect`, bagaimana encoder dibuat melaporkannya tanpa mengubah apa yang digambarnya, dan bagaimana web demo menjalankan library ini di browser untuk menampilkannya.

> Versi bahasa Inggris: [29-symbol-inspection.md](29-symbol-inspection.md).

## 1. Lapisan-lapisan sebuah simbol

Setiap modul termasuk salah satu dari dua jenis. **Modul fungsi** ditentukan oleh tata letak: finder pattern, separator, timing pattern, alignment pattern, dark module, dan pada rMQR, finder sub-pattern serta corner pattern. Format information dan version information juga menempati posisi yang tetap, meskipun nilainya bergantung pada simbolnya. **Modul pesan** membawa codeword stream, yaitu data codeword dan error-correction codeword yang di-interleave, ditambah beberapa remainder bit di tempat stream habis lebih dulu daripada modulnya.

Mask hanya menyentuh modul pesan. Jadi sebuah simbol adalah tiga lapisan yang saling bertumpuk: modul fungsi, pesan sebagaimana ditempatkan, dan pola mask, yang membalik modul pesan yang dipilihnya. `Matrix` mengembalikan gabungannya. `Inspect` mengembalikan tiap lapisan, dan untuk setiap modul pesan, bit codeword yang dibawanya.

## 2. Peran modul

`ModuleRole` menamai sebelas peran: data, remainder, finder, separator, timing, alignment, format, version, dark module, finder sub-pattern, dan corner pattern. Data dan remainder adalah modul pesan; sisanya modul fungsi (`IsFunction`). Menghitung peran dalam sebuah simbol adalah cek yang bagus untuk tata letaknya, karena setiap jumlahnya mengikuti standar:

| Simbol | Finder | Separator | Timing | Alignment | Format | Version | Lainnya | Data | Remainder |
|---|---|---|---|---|---|---|---|---|---|
| QR V1 (21 × 21) | 147 | 45 | 10 | 0 | 30 | 0 | dark module 1 | 208 | 0 |
| QR V7 (45 × 45) | 147 | 45 | 48 | 150 | 30 | 36 | dark module 1 | 1568 | 0 |
| QR V40 (177 × 177) | 147 | 45 | 272 | 1150 | 30 | 36 | dark module 1 | 29648 | 0 |
| Micro M1 (11 × 11) | 49 | 15 | 6 | 0 | 15 | 0 | | 36 | 0 |
| Micro M4 (17 × 17) | 49 | 15 | 18 | 0 | 15 | 0 | | 192 | 0 |
| rMQR R7x43 | 49 | 7 | 58 | 18 | 36 | 0 | sub-pattern 25, corner 4 | 104 | 0 |
| rMQR R11x27 | 49 | 15 | 41 | 0 | 36 | 0 | sub-pattern 25, corner 9 | 120 | 2 |
| rMQR R17x139 | 49 | 15 | 297 | 72 | 36 | 0 | sub-pattern 25, corner 9 | 1856 | 4 |

Tiga tumpang-tindih menentukan sebagian angka ini, dan masing-masing adalah tempat hitungan yang naif bisa keliru.

- **Alignment di atas timing.** Mulai versi 7, QR menempatkan alignment pattern yang berpusat di baris 6 dan kolom 6, di atas timing pattern, dan masing-masing mengambil alih lima modul timing. Dengan n sebagai sisi dan k sebagai jumlah pusat alignment, jumlah timing QR adalah 2(n − 16) − 10(k − 2) untuk versi 7 ke atas: 48 di V7 (k = 3) dan 272 di V40 (k = 7). Jumlah alignment-nya 25(k² − 3).
- **Area format QR dan modul timing.** Area yang di-reserve QR untuk format information, yaitu baris 8 dan kolom 8 di dekat finder kiri-atas, ikut mencakup (6, 8) dan (8, 6), yang merupakan bagian timing pattern. Keduanya tetap modul timing; format information sendiri terdiri dari 2 × 15 = 30 modul.
- **Simbol R7.** Pada tinggi 7, finder mengisi seluruh tinggi simbol, sehingga separator-nya hanya kolom 7 (7 modul, dibanding 15 mulai tinggi 9), dan corner pattern kiri-bawah, yaitu tiga modul gelap (h − 1, 0) sampai (h − 1, 2), jatuh di tepi bawah finder. Modul-modul itu dihitung sebagai modul finder, sehingga tersisa 4 modul corner pattern di kanan-atas. Mulai tinggi 11, corner pattern kiri-bawah punya 5 modulnya sendiri, maka jumlahnya 9.

Jumlah modul data adalah 8 kali jumlah codeword (26 codeword di V1-M, 13 di R7x43), kecuali pada Micro QR M1 dan M3, yang data codeword terakhirnya hanya 4 bit (bagian 4).

## 3. Mencatat peran selama membangun

`Inspect` tidak mendeskripsikan tata letak untuk kedua kalinya. Deskripsi kedua bisa melenceng dari kode yang menggambar simbolnya, dan gambar yang benar dengan deskripsi yang salah akan mengajarkan hal yang keliru. Sebagai gantinya, `Inspect` menjalankan encoder itu sendiri (`buildMatrix`) dengan trace yang menempel pada matriksnya. Setiap fungsi penempatan memberi label peran pada modul yang digambarnya: `placeSingleFinder` melabeli modul finder dan separator, fungsi timing dan alignment melabeli modulnya masing-masing, dan seterusnya. Pola yang digambar di atas pola sebelumnya mengambil alih modulnya, seperti alignment di atas timing. Langkah yang hanya me-reserve modul, seperti area format QR, hanya melabeli modul yang belum dipegang pola sebelumnya, dan itulah yang membuat (6, 8) dan (8, 6) tetap menjadi modul timing.

Penempatan zigzag (`placeBits`) mencatat, di setiap modul pesan, indeks bit stream yang ditulisnya. Setelah mask dan format information, `Inspect` membaca trace-nya dan menyalin modul akhirnya. Karena trace menumpang pada kode yang benar-benar meng-encode, `Modules` dengan sendirinya sama dengan yang dikembalikan `Matrix`, dan sebuah test memeriksanya di ketiga simbologi.

Trace adalah pointer yang hanya diisi oleh `Inspect`. Setiap encoder lain membangun matriks tanpa trace, sehingga biaya tracing bagi mereka hanyalah satu cek nil per modul.

## 4. Dari bit stream ke codeword

Untuk QR dan rMQR, stream terdiri dari codeword utuh, bit paling signifikan lebih dulu. Bit stream ke-b adalah bit 7 − (b mod 8) dari codeword ⌊b / 8⌋, dengan bit 7 sebagai yang paling signifikan. Bit sesudah codeword terakhir adalah remainder bit: 0, 3, 4, atau 7 di QR tergantung versinya, dan 0 sampai 7 di rMQR. Nilainya nol sebelum masking dan tidak membawa codeword.

Micro QR M1 dan M3 keluar dari pola ini. Kapasitas datanya bukan kelipatan 8 (20 dan 84 bit), sehingga data codeword terakhirnya hanya 4 bit, dan error-correction codeword langsung menyusul sesudahnya di stream (dokumen 26, bagian 4). Dengan f data codeword utuh sebelum codeword setengah itu:

- bit 0 sampai 8f − 1 milik codeword 0 sampai f − 1 seperti biasa;
- bit 8f sampai 8f + 3 adalah bit 7 sampai 4 dari codeword f, sang half codeword;
- bit 8f + 4 + j milik codeword f + 1 + ⌊j / 8⌋, bit 7 − (j mod 8).

M1, misalnya, punya dua data codeword utuh, satu half codeword, dan dua error-correction codeword: 16 + 4 + 16 = 36 bit stream, satu untuk tiap modul pesan. `Inspection.Codewords` menyimpan half codeword di nibble atas byte-nya, dengan nol di bawahnya, yang juga merupakan cara Reed-Solomon melihatnya.

Sebuah test memeriksa pemetaan ini dari dua arah: setiap modul pesan, sebelum masking, memuat bit `Codewords` yang ditunjuk `ModuleCodeword` dan `ModuleBit`, dan tidak ada bit codeword yang muncul dua kali.

## 5. Sebelum mask, dan skor mask

`MaskPattern` menandai setiap modul yang dipilih oleh rumus mask, di seluruh grid. Rumusnya adalah mask QR ke-k untuk QR, mask QR 1, 4, 6, atau 7 untuk mask Micro QR 0 sampai 3, dan mask QR 4 untuk rMQR (dokumen 6, 26, dan 28). `Unmasked` adalah simbol dengan mask yang dilepas lagi dari modul pesan: modul pesan di `Unmasked` adalah nilainya di `Modules` di-XOR dengan polanya, dan modul fungsi tidak berubah. Di tampilan tanpa mask, deretan panjang dan blok padat dari data menjadi terlihat, dan justru itulah yang ingin dipecah oleh masking.

`MaskScores` menilai setiap kandidat mask pada simbol tanpa mask, seperti yang dilakukan encoder saat memilih:

- **QR.** Setiap mask diterapkan, format information untuk level dan mask itu ditulis, lalu keempat aturan penalty dijumlahkan (dokumen 6): N1 untuk deretan lima modul atau lebih, N2 untuk blok 2 × 2, N3 untuk urutan yang mirip finder, N4 untuk rasio gelap yang tidak seimbang. `MaskPenalties` menyimpan keempat bagiannya. Total terendah menang, dan nomor mask yang lebih kecil menang kalau seri.
- **Micro QR.** Setiap mask diterapkan lalu modul gelap dihitung di sepanjang tepi kanan dan tepi bawah, tanpa modul timing; dengan SUM1 sebagai hitungan yang lebih kecil dan SUM2 yang lebih besar, skornya 16 × SUM1 + SUM2 (dokumen 26, bagian 9). Skor tertinggi menang.
- **rMQR.** Hanya ada satu mask tetap, jadi tidak ada yang perlu dinilai, dan `MaskScores` bernilai nil.

Skor tetap dihitung meskipun `WithMask` memaksakan sebuah mask: encoder melewati perbandingannya dalam kasus itu, tetapi skornya menunjukkan apa yang akan dipilihnya dan kenapa.

## 6. Anggaran bit pesan

`Segments` mendaftar bagian-bagian pesan secara berurutan, masing-masing dengan lebar ketiga field-nya: mode indicator, character count, dan payload. Lebarnya bergantung pada simbologi: indicator QR 4 bit dan lebar count-nya bergantung pada kelompok versi (dokumen 2); indicator Micro QR 0 sampai 3 bit dan lebar count-nya punya aturan sendiri (dokumen 26); indicator rMQR 3 bit dan lebar count-nya bergantung pada versi (dokumen 28). Header ECI muncul sebagai segmen dengan mode ECI, field count-nya kosong dan payload-nya adalah designator: 8 bit untuk ECI 3 (ISO-8859-1) dan ECI 26 (UTF-8). Di bawah ECI, segmen byte dihitung dalam byte hasil transcode, sehingga `é` bernilai satu byte di ISO-8859-1 dan dua byte di UTF-8. Segmen Kanji menghitung karakter, masing-masing 13 bit.

Sesudah segmen-segmen itu, yang totalnya `DataBits`, datang terminator: 4 bit nol di QR, 3, 5, 7, atau 9 di Micro QR M1 sampai M4, dan 3 di rMQR, dipendekkan kalau kapasitasnya habis lebih dulu. `TerminatorBits` adalah panjang sebenarnya. Sisa `CapacityBits` adalah padding: bit nol sampai batas codeword berikutnya, lalu pad codeword 0xEC dan 0x11 bergantian.

`HELLO WORLD` di V1-M (dokumen 10) memakai anggarannya seperti ini: satu segmen alphanumeric sebesar 4 + 9 + 61 = 74 bit, terminator 4 bit, 2 bit nol untuk mencapai 80 bit, dan enam pad codeword untuk mengisi 16 data codeword, menghasilkan `20 5B 0B 78 D1 72 DC 4D 43 40 EC 11 EC 11 EC 11`. Simbol R11x27-M untuk 14 digit (dokumen 28, bagian 11) menunjukkan sisi ekstrem yang lain: segmennya memakan 54 dari 56 bit, sehingga terminatornya menyusut menjadi 2 bit dan tidak ada padding sama sekali.

## 7. Di mana tiap codeword berada

`Blocks` mendaftar data codeword tiap blok Reed-Solomon dan error-correction codeword yang dihitung darinya; `Codewords` adalah stream dalam urutan penempatan. Keduanya terhubung lewat interleaving (dokumen 4): data codeword pertama dari setiap blok, lalu yang kedua dari setiap blok, dan seterusnya, melewati blok yang lebih pendek begitu isinya habis, lalu error-correction codeword dengan cara yang sama.

Demo memutar ulang urutan itu untuk memberi label pada setiap codeword di stream: bloknya, posisinya di blok, dan apakah ia data atau error correction. V5-Q punya dua blok berisi 15 data codeword dan dua blok berisi 16, masing-masing dengan 18 error-correction codeword. 60 codeword pertama di stream-nya mengambil posisi 0 sampai 14 dari keempat blok secara bergiliran; codeword 60 dan 61 adalah posisi 15 dari dua blok yang lebih panjang; lalu 72 error-correction codeword menyusul, lagi-lagi blok demi blok. Mengarahkan kursor ke sebuah codeword di demo menyorot delapan modulnya, yang menunjukkan bagaimana interleaving menyebar setiap blok ke seluruh simbol, sehingga kerusakan di satu tempat hanya merugikan setiap blok sedikit, bukan satu blok banyak.

## 8. Menjalankan library di browser

Demo menjalankan library itu sendiri. Target `js/wasm` milik Go meng-compile-nya, bersama sebuah bridge kecil, menjadi satu modul WebAssembly berukuran sekitar 4,7 MB (1,3 MB setelah dikompresi). Modul itu mengekspos satu fungsi JavaScript, `qrgenCall(method, request, ...blobs)`, yang menerima request JSON dan input biner seperti file gambar, lalu mengembalikan response JSON. Operasi di baliknya (encode, inspect, masks, payload, structuredAppend, decode, decodePixels, dan decodeStructuredAppend) adalah kode Go biasa di `web/internal/bridge`, sehingga dites dengan toolchain normal, dan test yang sama juga dijalankan setelah di-compile ke WebAssembly di Node.

Runtime WebAssembly milik Go berjalan di satu thread, dan sebagian panggilan butuh waktu yang terasa: simbol versi 40, atau satu frame kamera. Karena itu modulnya ditempatkan di Web Worker, yang menangani panggilan satu per satu sementara halaman tetap responsif. Glue `wasm_exec.js` yang menyalakan runtime harus berasal dari rilis Go yang sama dengan compiler-nya, jadi build script menyalinnya dari toolchain yang mem-build modul.

Gambar melewati jalur milik library sendiri: byte sebuah file di-decode oleh package image milik Go dan dibaca oleh `DecodeAll`, dengan `Decode` sebagai cadangan untuk simbol buram yang terlewat oleh detektor multi-simbol. Frame kamera datang sebagai piksel RGBA dari sebuah canvas. Halaman mengirim kotak tengah dari tiap frame, dengan lebar paling besar 480 piksel, dan bridge mengubahnya ke abu-abu dengan bobot `color.GrayModel`, Y = (19595 R + 38470 G + 7471 B + 2¹⁵) >> 24 pada channel 16 bit, sehingga decoder melihat tingkat abu-abu yang sama seperti dari gambar RGBA-nya.

Biaya per frame bergantung pada isi frame-nya. Frame 640 × 480 yang berisi simbol ter-decode dalam sekitar 6 ms secara native dan 20 ms di WebAssembly; frame realistis tanpa simbol butuh sekitar 30 ms secara native. Frame berisi noise acak yang halus adalah kasus terburuk, sekitar 0,7 detik secara native dan 2,6 detik di WebAssembly: setiap bintik menjadi satu connected component, dan pencarian finder membandingkan komponen bersarang secara berpasangan (dokumen 24). Kotak yang lebih kecil membatasi biaya itu, dan worker menjaga halaman tetap responsif selama biaya itu dibayar.

## 9. Cara pengecekannya

`Inspect` melaporkan tentang encoder, jadi risikonya adalah laporan yang cocok dengan dirinya sendiri tetapi tidak dengan simbolnya. Karena itu pengecekannya bersandar pada fakta dari luar `Inspect`:

- `Modules` sama dengan keluaran `Matrix` untuk setiap simbologi dan option yang mengubah encoding: versi, level, mask, ECI, Kanji, dan logo.
- Lapisan-lapisannya cocok dengan codeword-nya: setiap modul pesan tanpa mask memuat bit yang diklaimnya, setiap bit codeword muncul sekali, modul remainder terang sebelum masking, dan modul fungsi tidak pernah di-mask.
- Sensus peran di bagian 2 dihitung manual dari tata letak di dokumen 5, 26, dan 28. Sensus ini menangkap satu kesalahan label saat dokumen ini ditulis: modul corner kiri-bawah pada R7 awalnya dilabeli corner pattern di atas finder; simbolnya benar, labelnya yang keliru.
- Codeword di dokumen 10 dan dua stream di dokumen 28, yang berasal dari zint, cocok persis.
- Mask otomatis adalah penalty QR terendah atau skor Micro tertinggi, dan memaksakan sebuah mask tidak mengubah skornya.

## 10. Penunjuk implementasi

- `qrgen/inspect.go`: `ModuleRole`, `Inspection`, `InspectSegment`, `InspectBlock`, `Inspect`, trace-nya, pemetaan bit stream (`byteLocator`, `microLocator`), dan skor mask.
- `qrgen/matrix.go`, `qrgen/micro_matrix.go`, `qrgen/rmqr_matrix.go`: label peran di kode penempatan (`tag`, `reserveAs`), dan bit stream di `placeBits`.
- `web/internal/bridge/bridge.go`: operasi JSON, termasuk lokasi codeword dan konversi abu-abu untuk frame kamera.
- `web/wasm/main.go`: entry point WebAssembly.
- `web/site/`: halamannya, dengan `inspect.js` yang menggambar lapisan-lapisan yang dijelaskan di sini.

## Referensi

- ISO/IEC 18004:2015 dan ISO/IEC 23941:2022 — tata letak, mask, dan evaluasi mask yang diikuti oleh peran dan skornya.
- `docs/theory/04-reed-solomon.md`, `05-matrix-construction.md`, `06-masking.md`, `07-format-version-info.md`, `10-worked-example.md`, `24-arbitrary-angle-detection.md`, `26-micro-qr.md`, `28-rmqr.md` — langkah-langkah yang dibedah lagi oleh dokumen ini.
- Go, `syscall/js` dan port `js/wasm` — target WebAssembly beserta glue `wasm_exec.js`-nya.
