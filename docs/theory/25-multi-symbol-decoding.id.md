# Multi-Symbol Decoding

Decoder v0.14 membaca setiap simbol QR yang ada dalam satu gambar, bukan hanya yang pertama. Satu halaman berisi banyak kode, satu strip tiket, atau satu set structured-append yang difoto bersama, terdekode dalam satu panggilan `DecodeAll`. Dokumen ini menjelaskan kenapa detektor sudah melihat semua simbol, apa masalah yang benar-benar baru (mengelompokkan finder jadi simbol), dan kenapa keberhasilan decode — bukan geometri — harus menjadi penentu akhir.

> Versi bahasa Inggris: [25-multi-symbol-decoding.md](25-multi-symbol-decoding.md).

## 1. Detektor sudah melihat setiap simbol

Menemukan sebuah simbol QR dimulai dari tiga pola findernya. Detektor connected-component v0.13 (dokumen 24) melabeli gambar terbinarisasi dan mencocokkan setiap penyarangan cincin-gelap / cincin-terang / pusat-gelap dengan sisi bounding box 7:5:3. Pencocok itu tak terbatas pada satu simbol: `finderNestings` mengembalikan **setiap** cincin finder di gambar. Foto berisi empat kode QR menghasilkan (sekitar) dua belas kandidat finder.

Pipeline single-symbol lalu membuang sebagian besarnya. `bestFinderQuadTriple` menyimpan kandidat, dan `bestFinderTripleIdx` memilih satu triple dengan cost geometri terendah — satu susunan right-isosceles terbaik — lalu men-decode-nya. Semua yang dibutuhkan untuk membaca keempat kode sudah terhitung; pipeline hanya menciut ke satu di langkah seleksi terakhir. Maka multi-symbol decoding bukan detektor baru. Ia adalah *seleksi* yang berbeda: alih-alih satu triple terbaik, temukan **semua triple valid yang disjoint**, dan decode masing-masing lewat pipeline per-simbol yang tak berubah.

## 2. Masalah grouping

Diberi N kandidat finder, partisikan menjadi nol atau lebih simbol, di mana tiap simbol adalah triple berisi tiga finder yang

- berbagi pitch modul dalam toleransi (tiga finder sebuah simbol berukuran sama),
- membentuk susunan right-isosceles — dua leg yang kira-kira sama bertemu pada sudut siku di finder top-left, bentuk sebuah simbol persegi yang dilihat lewat similarity transform apa pun, dan
- tidak memakai ulang finder: tiap finder milik paling banyak satu simbol.

Dua oracle untuk ini sudah ada dan dipakai ulang tanpa perubahan. `finderTripleCost` memberi skor sebuah triple — jumlah residu module-spread dan dua residu geometri, nol untuk simbol sempurna, makin rendah makin baik. `orderFinderTriple` memvalidasi sebuah triple — ia menetapkan top-left/top-right/bottom-left secara invarian rotasi dan menolak susunan implausibel (rasio leg di atas 1.5, residu hipotenusa di atas 0.15). Jalur single-symbol memakai keduanya untuk mengambil minimum global. Grouping multi-symbol meng-enumerasi triple kandidat (ceiling O(N^3) yang sama, dengan N dibatasi), menyimpan yang lolos `orderFinderTriple` dan sebuah ceiling cost, mengurutkannya terbaik-dulu, dan meng-klaim triple disjoint secara greedy.

## 3. Keberhasilan decode adalah penentunya

Geometri saja tak cukup untuk mengelompokkan dengan benar, dan inilah jantung halus milestone ini. Bayangkan grid teratur berisi kode same-size. Ambil satu finder dari simbol A dan dua dari simbol B: ketiga pusat itu bisa membentuk segitiga right-isosceles sempurna pada pitch modul bersama. **Triple cross-symbol** semacam itu lolos setiap gerbang geometri. Kalau grouping mempercayai geometri dan meng-klaim ketiga finder itu, ia akan merusak A dan B — mencuri finder nyata dari masing-masing dan tak men-decode keduanya.

Pertahanannya adalah membuat **sebuah finder di-klaim hanya ketika triple yang memuatnya benar-benar Reed-Solomon decode**. Triple cross-symbol menyampel grid yang bukan simbol nyata, jadi `DecodeMatrix` gagal di cek format-information dan RS-nya, dan triple itu dibuang tanpa satu pun findernya dikonsumsi — mereka tetap di pool untuk triple nyata yang memang men-decode. Keberhasilan decode, bukan plausibilitas geometri, adalah penentu grouping akhir. Ini juga memberi backtracking implisit tanpa pembukuan: triple yang gagal meninggalkan findernya bebas, jadi triple tumpang-tindih terbaik berikutnya tetap dapat giliran, dibatasi hanya oleh cap attempt.

Cap attempt itu harus **berskala dengan jumlah finder**. Grid berisi banyak kode same-size menghasilkan banyak triple cross-symbol plausibel yang tersortir di antara yang nyata; cap tetap yang kecil bisa habis di sampah sebelum simbol nyata tercapai, diam-diam men-drop-nya. Batasnya tumbuh dengan N agar simbol nyata selalu terjangkau.

## 4. Memakai ulang pipeline per-simbol

Begitu sebuah triple terkelompok dan terurut, men-decode-nya persis seperti yang sudah dilakukan jalur projektif single-symbol: estimasi version dari panjang sisi quad finder, trial kandidat version, fit homografi projektif dua-belas-sudut, sampel grid modul, dan Reed-Solomon decode (dokumen 24). Milestone ini memfaktorkan body itu keluar dari jalur projektif single-symbol menjadi helper, `decodeOneQuadTriple`, sehingga baik jalur single-symbol maupun loop multi-symbol memanggil kode per-simbol yang identik. Helper itu memunculkan version dan homography yang dimenangkan, yang dibuang jalur single-symbol tetapi dibutuhkan hasil publik (bagian 5).

Karena tiap simbol membawa pitch findernya sendiri ke homografi dan trial version-nya sendiri, simbol beda-ukuran dan beda-tilt hidup berdampingan dalam satu gambar tanpa saling ganggu: grouping menjaga tiga finder tiap triple pada satu pitch, dan semua di hilir bersifat per-triple.

`Decode` single-symbol sengaja dibiarkan pada seleksi argmin-global, bukan hasil pertama grouper. Greedy-di-bawah-threshold bisa memilih triple berbeda dari minimum ketat pada gambar satu-simbol yang noisy, jadi menjaga `Decode` pada `bestFinderTripleIdx` mempertahankan perilaku persisnya; sebuah test meng-assert `DecodeAll` pada gambar satu-simbol mengembalikan tepat yang dikembalikan `Decode`.

## 5. Melokasi tiap simbol

Hasil multi-symbol hanya berguna kalau pemanggil tahu di mana tiap simbol berada. Dua bagian geometri sudah di tangan begitu sebuah simbol men-decode:

- **Empat sudut luar simbol**, diturunkan dari homography yang dimenangkan. Homography memetakan koordinat modul ke piksel, jadi menerapkannya ke sudut grid-modul `(-0.5, -0.5)`, `(n-0.5, -0.5)`, `(n-0.5, n-0.5)`, `(-0.5, n-0.5)` — tepi luar sebuah simbol berukuran `n = version.Size()` modul — memberi garis luar simbol sejati dalam piksel, berotasi dan ter-shear bersama simbolnya. Ini *bukan* sudut cincin finder, yang hanya membatasi finder 7x7; sudut simbol berasal dari homography dan mencakup seluruh kode.
- **Tiga pusat finder**, dari triple terurut, yang memaku orientasi (sisi mana yang atas).

Persegi panjang pembatas sejajar-sumbu ditawarkan hanya sebagai kemudahan turunan (`Bounds()`), bukan geometri utama: untuk simbol miring, garis luar sejatinya adalah quad sudut, dan kotak sejajar-sumbu akan kira-kira menggandakan luasnya dan tumpang-tindih dengan tetangga — persis salah dalam scene berisi beberapa kode pada sudut aneh.

Hasil dikembalikan dalam reading order deterministik: simbol dikelompokkan jadi baris berdasarkan posisi vertikal sudut top-left-nya dalam toleransi, baris berjalan atas ke bawah dan simbol kiri ke kanan dalam satu baris, dengan tie-break akhir eksak agar urutannya reprodusibel. Ini best-effort untuk layout grid dan dokumen; penempatan tersebar tetap mendapat urutan yang stabil dan deterministik.

## 6. Semantik error dan hasil

Multi-symbol decoding ada untuk membaca yang bisa dibaca dari scene berantakan, jadi satu simbol yang tak terbaca tak boleh menggagalkan batch. Simbol yang finder, format info, atau blok Reed-Solomon-nya tak bisa dipulihkan cukup dilewati, dan simbol yang memang men-decode tetap dikembalikan. `DecodeAll` mengembalikan error hanya untuk kegagalan seluruh-gambar: byte gambar yang cacat, atau **nol** decode sukses — entah dari kurang dari tiga finder atau dari finder yang tak pernah men-decode — yang dipetakan ke `ErrFinderNotFound` demi paritas dengan `Decode` tunggal. Ia tak pernah mengembalikan slice hasil parsial berikut error.

Menyusun ulang set structured-append dari satu gambar komposit di luar cakupan: `DecodeAll` mengembalikan teks per-simbol mentah, dan `DecodeStructuredAppend` tetap jalur yang menjahit gambar terpisah jadi satu pesan.

## 7. Batas yang jujur

- **Connected-component saja.** `DecodeAll` memakai detektor CC, bukan line scanner afin, karena hanya jalur CC yang meng-enumerasi semua finder. Simbol tegak yang blur atau low-resolution yang akan didekode line scan 1:1:3:1:1, tetapi cincinnya tak melabel bersih jadi nesting, mungkin dibaca `Decode` tetapi terlewat `DecodeAll`.
- **Finder harus penuh dalam frame.** Cincin finder yang menyentuh tepi gambar tak melabel jadi nesting berongga, jadi simbol yang keluar tepi di-drop alih-alih dipulihkan sebagian. Tak ada crash: sampling out-of-bounds dijaga.
- **Simbol yang bersentuhan bisa kehilangan finder tepi.** Dua simbol berdampingan yang quiet zone terangnya menyatu jadi satu component bisa merusak uji cincin-terang untuk finder di tepi bersama.
- **Biaya trial version.** Tiap triple terkelompok bisa mencoba beberapa version, masing-masing sampel-dan-decode penuh, jadi batas jujur pada gambar noisy adalah cap attempt kali window version; version terurut-kedekatan menjaga kasus umum tetap murah.

## 8. Penunjuk implementasi

Direncanakan di MS3/MS4 (lihat `docs/plan-multi-symbol-decoding.md`):

- `qrgen/decode_finder_cc.go`: `groupFinderTriples` (enumerasi, gerbang oleh `orderFinderTriple` dan ceiling cost terukur, sort, klaim disjoint berbasis indeks) dan `allFinderQuadTriples`.
- `qrgen/decode_image.go`: `decodeOneQuadTriple(bm, tq) (text, Version, homography, error)`, diekstrak dari jalur projektif single-symbol tanpa perubahan perilaku.
- `qrgen/decode_multi.go`: tipe `Result` dengan `Bounds()`, penurunan sudut simbol dari homography, dan `DecodeAll` / `DecodeAllBytes` publik. Sejak v0.16 loop ini juga mencoba setiap finder yang belum diklaim sebagai simbol Micro QR (dokumen 27), yang dilaporkan lewat `Result.MicroVersion`.
- Test mengomposit beberapa simbol yang di-render ke satu kanvas dan cross-check reader `multi/qrcode` milik `gozxing`.

## Referensi

- ISO/IEC 18004:2015 — struktur pola finder dan quiet zone yang diandalkan detektor.
- `docs/theory/24-arbitrary-angle-detection.md` — detektor finder connected-component dan pipeline projektif per-simbol yang dipakai ulang di sini tanpa perubahan.
- `docs/theory/15-rotation-handling.md` — pengurutan finder invarian rotasi (`orderFinderTriple`) yang dipakai ulang sebagai gerbang validitas grouping.
- ZXing `MultiDetector` / `MultiFinderPatternFinder.selectMultipleBestPatterns` — prior art untuk mengelompokkan finder jadi beberapa simbol; `multi/qrcode.QRCodeMultiReader` milik `gozxing` adalah oracle cross-check.
