# Kanji Mode

Encoder dan decoder v0.11 menambahkan **Kanji mode**, mode data terakhir dari empat yang didefinisikan ISO/IEC 18004 dan satu-satunya yang meng-encode teks Jepang secara padat. Kalau byte mode menghabiskan tiga byte UTF-8 (24 bit) untuk satu kanji umum, Kanji mode menghabiskan 13 bit — kira-kira 1,8x lebih padat. Dokumen ini mencatat layout bit mode-nya, derivasi Shift-JIS ke 13-bit, cakupan JIS X 0208, bagaimana pemetaan Unicode tetap zero-dependency, dan bagaimana Kanji cocok dengan mixed-mode segmenter.

> Versi Inggris: [22-kanji-mode.md](22-kanji-mode.md).

## 1. Kenapa Kanji mode

- **Byte mode boros untuk bahasa Jepang.** Sebuah kanji BMP adalah tiga byte di UTF-8, jadi byte-mode segment menghabiskan 24 bit per karakter. Kanji mode meng-encode karakter yang sama dalam 13 bit, sedikit di bawah separuh ukurannya.
- **Ia melengkapi himpunan mode.** Numeric, alphanumeric, byte, dan Kanji adalah empat mode data di standar; dengan Kanji, encoder mencakup semuanya (ECI dan structured append adalah header, bukan mode data).
- **Ia pilihan segmentation, bukan saklar.** Kanji bergabung dengan DP segmenter sebagai mode keempat, jadi payload yang mencampur Jepang dengan ASCII terpecah menjadi run Kanji, alphanumeric, dan byte pada batas mana pun yang meminimalkan total — sebuah run kanji menjadi Kanji segment, ASCII di sekitarnya tetap numeric/alphanumeric/byte.

## 2. Layout bit

Kanji segment berbentuk mode-segment biasa: mode indicator 4-bit, character-count indicator, lalu payload.

- **Mode indicator: `1000`** (4 bit).
- **Character-count indicator:** 8 bit untuk version 1-9, 10 bit untuk 10-26, 12 bit untuk 27-40. Ini count tersempit di antara semua mode, karena simbol Kanji memuat lebih sedikit karakter dibanding simbol numeric seukuran.
- **Payload:** 13 bit per karakter (bagian 3). Kanji segment version-1 sebanyak `n` karakter karena itu `4 + 8 + 13n` bit.

## 3. Encoding 13-bit

Tiap karakter di-encode dari kode dua-byte Shift-JIS-nya `C`:

1. Jika `0x8140 <= C <= 0x9FFC`, kurangi `0x8140`: `D = C - 0x8140`.
2. Jika `0xE040 <= C <= 0xEBBF`, kurangi `0xC140`: `D = C - 0xC140`.
3. Nilai 13-bit adalah `(D >> 8) * 0xC0 + (D & 0xFF)`.

Decoding membalikkannya: dari nilai 13-bit `V`, `D = ((V / 0xC0) << 8) | (V % 0xC0)`, lalu tambahkan kembali `0x8140` saat `D` di bawah rentang range pertama dan `0xC140` selain itu, memulihkan `C`.

Contoh kerja — karakter 亜 (U+4E9C), Shift-JIS `0x889F`:

```text
C = 0x889F                         (di range 0x8140..0x9FFC)
D = 0x889F - 0x8140 = 0x075F
V = (0x07 * 0xC0) + 0x5F = 1344 + 95 = 1439 = 0x59F
  -> 13 bit: 0 0101 1001 1111
```

Library memegang rune Unicode, bukan byte Shift-JIS, jadi pipeline sebenarnya adalah rune -> Shift-JIS (via tabel, bagian 4) -> 13-bit saat encode, dan 13-bit -> Shift-JIS -> rune saat decode.

## 4. JIS X 0208 dan tabel yang di-generate

QR Kanji mode menyasar persis satu character set, **JIS X 0208**, diserialisasi sebagai Shift-JIS. Sebuah rune Kanji-eligible hanya jika ia punya kode JIS X 0208; selain itu (JIS X 0212, emoji, kebanyakan Latin) tak eligible dan tetap di byte mode. Tak ada error untuk rune tak terpetakan — ia sekadar pilihan mode berbeda.

Memetakan sebuah rune Unicode ke kode Shift-JIS-nya butuh tabel kira-kira 6.900 entry. Untuk menjaga library di zero runtime dependency, tabel itu **di-generate saat development dan di-commit** sebagai file Go source murni (`kanji_table.go`) tanpa import. Generator-nya (`gen_kanji_table.go`) membawa tag `//go:build ignore` dan dijalankan dengan `go generate`; ia satu-satunya kode yang meng-import `golang.org/x/text/encoding/japanese`, yang sudah menjadi modul indirect (test-only). Membangun atau meng-test library tak pernah meng-compile generator dan tak pernah meng-import `x/text`. Regenerasi tabel adalah langkah jarang yang terdokumentasi, dan CI memastikan file yang di-commit persis byte-for-byte dengan yang diproduksi `go generate`. Codec selalu round-trip lewat tabel, tak pernah lewat aritmetika buta, jadi kode Shift-JIS tak ter-assign tak pernah diproduksi atau diterima.

## 5. Interaksi segmentation

Kanji ditambahkan ke dynamic-programming segmenter (doc 17) sebagai mode keempat di samping numeric, alphanumeric, dan byte. Sebuah rune eligible untuk Kanji segment hanya saat tabel memetakannya, persis seperti sebuah rune eligible untuk numeric mode hanya saat ia digit. DP membandingkan cost meng-encode sebuah run kanji eligible sebagai Kanji segment (13 bit tiap, plus header) terhadap memecahnya secara berbeda, lalu memilih yang minimum.

Karena ASCII tak pernah Kanji-eligible dan payload non-Jepang tak memuat rune eligible, menambah Kanji tak bisa mengubah segmentasi input mana pun yang tak punya karakter JIS X 0208 — tiap golden fixture yang ada ter-encode byte-for-byte seperti sebelumnya. Kanji hanya menang saat ia memang lebih murah, yang untuk run kanji selalu begitu (13 bit versus 24 di byte mode).

## 6. Densitas, secara jujur

Kanji mode adalah **13 bit per karakter**; kanji BMP yang sama di byte mode UTF-8 adalah tiga byte, **24 bit** — sekitar 1,8x. (Limitation README sebelumnya mengklaim "4x"; itu keliru dan dikoreksi dengan milestone ini, sejalan dengan kebiasaan proyek mengukur alih-alih menebak, dengan catatan ukuran-file doc 16 sebagai preseden.) Perolehannya nyata dan sepadan dengan mode-nya, tetapi ini perolehan densitas payload di bawah separuh, bukan empat kali lipat.

## 7. Penunjuk implementasi

- `qrgen/kanji.go` memuat `kanjiEligible(rune) bool`, `kanjiValue(rune) (uint16, bool)` (rune -> 13-bit), dan `kanjiRune(uint16) (rune, bool)` (13-bit -> rune), semuanya didukung tabel yang di-generate. Aritmetika offset Shift-JIS tinggal di sini dan di-unit-test di kedua batas range.
- `ModeKanji` (`1000`) bergabung ke `mode.go` dengan lebar character-count 8/10/12-nya; DP segmenter di `segment.go` mendapat prefix Kanji-eligibility dan cost 13-bit; `writeKanji` di `encode.go` mengeluarkan nilai dan version selection memperhitungkan Kanji segment.
- `decodeText` mendapat `case 0b1000`, membaca count Kanji lalu tiap nilai 13-bit kembali ke rune via tabel.
- Validasi berupa round-trip tabel rune-demi-rune atas tiap code point terpetakan, round-trip encode-decode untuk payload pure-Kanji dan campuran Jepang/ASCII, dan cross-check gozxing pada simbol Kanji.

## Referensi

- ISO/IEC 18004:2015 — mode indicator Kanji `1000`, lebar character-count 8/10/12 (Table 3), dan derivasi dua-byte-Shift-JIS-ke-13-bit dengan dua range offset-nya.
- JIS X 0208 — character set double-byte yang di-encode QR Kanji mode; Shift-JIS adalah serialisasi byte-nya.
- `golang.org/x/text/encoding/japanese` — codec Shift-JIS yang dipakai hanya oleh generator tabel yang diabaikan.
- `docs/theory/09-data-tables.md` dan `docs/theory/17-optimal-segmentation.md` — tabel mode/count dan DP segmenter yang Kanji perluas.
