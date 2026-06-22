# ECI Segments

Encoder dan decoder v0.9 menambahkan **Extended Channel Interpretation (ECI)**, mekanisme spec untuk mendeklarasikan character set dari data dalam sebuah simbol. Sampai sekarang byte-mode data dikeluarkan sebagai UTF-8 mentah tanpa deklarasi, dan decoder membaca byte segment sebagai UTF-8 secara implisit — praktis, dan itu yang diasumsikan kebanyakan scanner modern, tetapi sebuah non-conformance yang diketahui karena penafsiran byte-mode default menurut standar adalah ISO-8859-1, bukan UTF-8. ECI menutup celah itu dengan membiarkan pemanggil menyatakan character set secara eksplisit. Dokumen ini mencatat apa itu ECI header, bagaimana designator-nya di-encode, nomor assignment dan nuansa default-charset, scope sebuah ECI, serta batas transcoding zero-dependency yang membatasi character set mana yang didukung library ini.

> Versi Inggris: [20-eci-segments.md](20-eci-segments.md).

> Catatan istilah di depan: standar menyebut konstruksi `0111`-indicator-plus-designator sebagai **ECI header**, dan mencadangkan kata "segment" untuk mode segment (mode indicator + character-count indicator + data). Roadmap dan milestone ini memakai nama longgar "ECI segments"; di dokumen ini istilah presisinya *ECI header*.

## 1. Kenapa ECI

- **Byte mode tak membawa character set sendiri.** Sebuah byte segment hanyalah nilai 8-bit. Bagaimana byte itu memetakan ke karakter adalah keputusan terpisah, dan ISO/IEC 18004 menyatakan bahwa tanpa ECI default-nya adalah **ISO-8859-1** (Latin-1) pada edisi 2006/2015 yang berlaku. (Edisi 2000 yang asli menetapkan JIS X 0201; perubahan inilah yang membuat perilaku decoder di dunia nyata tak konsisten.)
- **Library ini justru mengasumsikan UTF-8.** String Go itu UTF-8, scanner modern mayoritas mengasumsikan UTF-8, dan encoder/decoder selama ini selalu memperlakukan byte mode sebagai UTF-8. Itu pragmatis tetapi sebuah penyimpangan disengaja dari default spec — item pertama di limitations README.
- **ECI membuat charset eksplisit dan conformant.** Mendeklarasikan ECI 26 (UTF-8) di depan berarti decoder ketat tak perlu lagi menebak; mendeklarasikan ECI 3 (ISO-8859-1) membiarkan pemanggil mengeluarkan Latin-1 sungguhan. Byte data tak berubah; hanya label eksplisit yang ditambahkan.
- **Opt-in, tak pernah otomatis.** Encoder mengeluarkan ECI header hanya saat diminta (`WithECI`). Tanpa ECI, bit stream-nya byte-for-byte seperti sebelumnya, sehingga fixture yang ada dan round-trip gozxing tak tersentuh dan library tetap membaca byte data tanpa-ECI sebagai UTF-8.

## 2. ECI header

Sebuah ECI header adalah **mode indicator ECI `0111`** 4-bit diikuti langsung oleh **designator** ECI. Tak seperti mode segment, ia **tak punya character-count indicator** — designator-nya self-delimiting (bagian 3). Pada kasus single-ECI yang umum, header duduk di **awal data bit stream**, sebelum mode segment pertama:

```text
0111  <designator>  <mode segment> <mode segment> ...  0000(terminator)
^ECI  ^charset      ^mis. 0100 byte: indicator + count + data
```

Urutan kerja menurut standar persis ini: `0111 / designator / 0100 / count / data`.

## 3. ECI designator

Designator meng-encode nomor assignment ECI dalam **1, 2, atau 3 codeword** (satu codeword = 8 bit). Panjangnya self-describing: yakni jumlah bit `1` di depan sebelum bit `0` pertama.

| Codeword | Bit | Template | Bit nilai | Range bentuk terpendek |
|----------|-----|----------|-----------|------------------------|
| 1 | 8  | `0bbbbbbb`                     | 7  | 0 – 127        |
| 2 | 16 | `10bbbbbb bbbbbbbb`            | 14 | 128 – 16383    |
| 3 | 24 | `110bbbbb bbbbbbbb bbbbbbbb`   | 21 | 16384 – 999999 |

`b…b` adalah nilai biner polos nomor assignment ECI, rata-kanan di bit nilai.

Dua kehalusan penting untuk implementasi yang benar:

1. **Encoder mengeluarkan bentuk terpendek; decoder wajib menerima bentuk apa pun.** Range di atas adalah output *preferred/kanonis* yang semestinya dihasilkan encoder. Range milik spec sendiri **overlap dan semua mulai dari nol** (1cw = 0–127, 2cw = 0–16383, 3cw = 0–999999): klausul 8.4.1.1 menyatakan "the lower numbered ECI assignments may be encoded in multiple ways, but the shortest way is preferred." Jadi decoder conformant harus membaca panjang dari bit prefix dan menerima encoding non-minimal (misalnya ECI 5 yang sah ditulis dalam bentuk 2- atau 3-codeword), walau encoder kita tak akan pernah memproduksinya.
2. **Maksimum 999999 itu cap desimal, bukan cap lebar-bit.** 21 bit nilai secara fisik bisa menampung hingga 2.097.151; standar membatasi nomor assignment ke enam digit desimal, jadi 999999 adalah plafonnya.

Contoh kerja (kedua nomor assignment yang didukung library ini muat dalam satu codeword):

```text
ECI 26 (UTF-8):       0111  00011010              -> mode 0111, lalu 0 + 0011010 (=26)
ECI 3  (ISO-8859-1):  0111  00000011              -> mode 0111, lalu 0 + 0000011 (=3)
ECI 128 (2-codeword): 0111  10000000 10000000     -> prefix 10 + 14-bit 128
```

## 4. Nomor assignment dan default charset

Nomor assignment berasal dari registry ECI AIM ITS/04-001. Tiga yang relevan di sini:

| ECI | Character set |
|-----|---------------|
| 000003 | ISO-8859-1 (Latin-1) |
| 000026 | UTF-8 |
| 000020 | Shift-JIS (JIS8 / JIS X 0208) |

Penafsiran **default** byte mode saat tak ada ECI adalah **ISO-8859-1** (setara ECI 3) pada standar yang berlaku — eksplisit *bukan* UTF-8. Agar conformant, UTF-8 harus dideklarasikan dengan ECI 26.

Library ini mengambil pilihan yang disengaja dan didokumentasikan yang berbeda dari default spec: tanpa ECI ia tetap memperlakukan byte mode sebagai **UTF-8** baik saat encode maupun decode, karena itu cocok dengan string Go, QR code dunia nyata, dan perilaku library sebelumnya, dan karena mengadopsi default Latin-1 akan diam-diam merusak setiap simbol UTF-8 yang sudah beredar. ECI lalu membiarkan pemanggil eksplisit ke arah mana pun: `ECIUTF8` mendeklarasikan asumsi yang sudah dipakai library, dan `ECILatin1` memilih Latin-1 sungguhan. Karena decode tanpa-ECI dan decode ECI-26 sama-sama UTF-8, simbol ECI-26 round-trip secara trivial; simbol ECI-3 di-decode lewat Latin-1.

## 5. Scope sebuah ECI

Per klausul 15.2 sebuah ECI menafsir ulang nilai byte dari **semua data berikutnya**, sampai akhir data atau header `0111` baru — ia tidak di-scope ke byte mode, dan klausul 8.4.1 bahkan membolehkan ECI data dibawa dalam numeric, alphanumeric, byte, atau kanji mode (mana pun yang meng-encode nilai byte paling padat). Jadi "ECI hanya memengaruhi byte segment" adalah observasi praktis, bukan aturan spec: numeric (digit) dan alphanumeric (subset ASCII 45-karakter tetap) meng-encode karakter yang sama di bawah setiap code-page ECI umum, sehingga efek *terlihat* dari ECI charset jatuh pada byte yang berbeda dari ASCII, yang dalam praktik melaju di byte mode. v0.9 mengeluarkan satu ECI di kepala dan menerapkan charset yang dideklarasikan saat merekonstruksi teks byte-mode, yang benar untuk penggunaan charset ini; pergantian ECI per-segment di tengah payload valid secara spec tetapi ditunda.

## 6. Transcoding zero-dependency

Library ini menjaga zero runtime dependency, yang membatasi character set yang didukung ke dua yang bisa di-transcode dengan standard library saja:

- **UTF-8 (ECI 26) — passthrough.** String Go sudah berupa urutan byte UTF-8, jadi payload byte adalah byte string itu tanpa transcoding. Satu catatan: string Go secara formal adalah urutan byte arbitrer yang immutable dan tak *dijamin* valid UTF-8 (`string([]byte{0xFF})` sah dan invalid), jadi encoder ketat boleh memvalidasi dengan `unicode/utf8.ValidString` sebelum mendeklarasikan ECI 26.
- **ISO-8859-1 (ECI 3) — bijection eksak, tanpa import.** Latin-1 memetakan byte `0xXX` ke Unicode code point `U+00XX` di seluruh range `0x00..0xFF` tanpa celah (inilah kenapa 256 code point pertama Unicode disemai dari Latin-1). Encoding-nya per-rune: keluarkan `byte(r)` saat `r <= 0xFF`, selain itu error pemanggil (misalnya `é` U+00E9 ter-encode ke `0xE9`, tetapi simbol euro `€` U+20AC — yang ada di Windows-1252/ISO-8859-15, bukan Latin-1 — dan rune astral apa pun benar-benar gagal). Decoding-nya `rune(b)` untuk tiap byte.
- **Selain itu di luar scope.** Shift-JIS (ECI 20) dan code page lain butuh tabel pemetaan, yang di Go berarti `golang.org/x/text/encoding` — modul eksternal, bukan bagian standard library (paket `golang.org/x/text` yang ikut toolchain adalah salinan vendored internal yang tak bisa di-import kode user). Menariknya masuk akan melanggar aturan zero-dependency, jadi mekanisme ECI di sini mem-parse designator apa pun tetapi hanya membawa transcoder untuk 3 dan 26.

## 7. Penunjuk implementasi

- `qrgen/eci.go` akan memuat tipe `ECI` (`ECIUTF8` = 26, `ECILatin1` = 3), encode designator (bentuk terpendek) dan decode (prefix-driven, menerima panjang non-minimal), serta kedua transcoder. `ModeECI` (`0111`) bergabung dengan mode indicator yang ada di `mode.go`.
- Encoder (`encodeText`) mengeluarkan ECI header di kepala stream saat `WithECI` di-set dan men-transcode payload byte ke charset yang dideklarasikan; `selectVersion` dan cek kapasitas force-version menambah overhead header `4 + 8/16/24` bit supaya payload yang pas-pasan tak bisa overflow begitu designator ditambahkan.
- Decoder (`decodeText`) mendapat `case 0b0111`: membaca designator lewat prefix-nya, menetapkan charset aktif, lalu men-decode byte segment berikutnya lewatnya (3 → Latin-1, 26 dan default tanpa-ECI → UTF-8). Sebuah ECI yang nomornya tak punya transcoder di-parse dan dilewati, dengan byte data berikutnya dibaca sebagai UTF-8 best-effort alih-alih menggagalkan simbol yang sebetulnya terbaca.
- Validasi berupa round-trip (`Encode` dengan ECI → `DecodeBytes` → teks persis) untuk UTF-8 dan Latin-1, plus cross-check gozxing pada simbol ECI-26, dan guard test yang memastikan output tanpa-ECI byte-identical dengan encoder pra-perubahan.

## Referensi

- ISO/IEC 18004:2015 (dan :2000 / :2006) — mode indicator ECI `0111` (Table 2), encoding designator 1/2/3-codeword (Table 4, klausul 8.4.1.1), struktur dan penempatan ECI header (klausul 8.4), serta scope ECI saat decode (klausul 15.2). Default byte-mode Latin-1 (edisi berlaku; JIS X 0201 pada 2000) ditetapkan di sini.
- AIM ITS/04-001 — registry assignment *Extended Channel Interpretations*: 3 = ISO-8859-1, 20 = Shift-JIS, 26 = UTF-8.
- `docs/theory/02-data-encoding.md` dan `docs/theory/09-data-tables.md` — catatan mode-indicator dan character-count yang ECI header perluas.
- Go standard library — `unicode/utf8` untuk validitas UTF-8 dan penyempitan Latin-1 yang trivial; sengaja bukan `golang.org/x/text`, yang merupakan modul eksternal.
