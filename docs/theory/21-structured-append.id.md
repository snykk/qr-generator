# Structured Append

Encoder dan decoder v0.10 menambahkan **structured append**, cara spec memecah satu pesan ke set 2..16 simbol QR yang saling tertaut dan menyatukannya kembali menjadi teks asli. Ini mengangkat plafon ukuran satu simbol (simbol V40 mentok di ~2.953 byte dalam byte mode di EC-L): pesan yang terlalu besar untuk satu simbol bisa dibawa beberapa. Dokumen ini mencatat structured-append header, symbol sequence indicator, parity byte, dan model split-lalu-reassemble.

> Versi Inggris: [21-structured-append.md](21-structured-append.md).

## 1. Kenapa structured append

- **Satu simbol punya kapasitas keras.** Simbol terbesar, V40 di EC level L, memuat sekitar 2.953 byte dalam byte mode; EC level lebih tinggi memuat lebih sedikit. Pesan lebih panjang sekadar tak muat.
- **Structured append merantai simbol.** Hingga 16 simbol bisa ditautkan sehingga scanner yang membaca semuanya merekonstruksi pesan asli, berurutan, terlepas dari urutan pemindaiannya. Tiap simbol adalah QR code biasa yang bisa di-decode independen dan sekaligus menyatakan posisinya dalam set.
- **Ia murni algoritmik.** Tak seperti Kanji (yang butuh tabel Shift-JIS) atau charset ECI sembarang (yang butuh `golang.org/x/text`), structured append hanyalah bit header, sebuah pembagian pesan, dan parity XOR — tanpa tabel data, jadi mempertahankan aturan zero-dependency.

## 2. Structured-append header

Tiap simbol dalam set memulai data bit stream-nya dengan structured-append header:

```text
0011           symbol sequence indicator   parity
^mode (4 bit)  ^8 bit                      ^8 bit
```

- **Mode indicator adalah `0011`** (4 bit).
- **Symbol sequence indicator** (8 bit, bagian 3) menyatakan simbol keberapa ini dan ada berapa total.
- **Parity byte** (8 bit, bagian 4) dibagi ke seluruh set dan membiarkan decoder memastikan simbol-simbol memang satu kelompok.

Header tak punya character-count indicator; ia tetap 20 bit. Ia mendahului segalanya di simbol — kalau ECI header juga ada, urutannya structured-append header, lalu ECI header, lalu data mode segment. Setelah header, chunk pesan milik simbol itu di-encode dengan mixed-mode segmentation biasa, diikuti terminator dan padding milik simbol itu sendiri.

## 3. Symbol sequence indicator

Sequence indicator 8-bit mengemas dua nibble 4-bit:

| Bit | Field | Makna |
|-----|-------|-------|
| nibble atas | posisi simbol `m` | indeks 0-based simbol ini dalam set (0..15) |
| nibble bawah | count dikurangi satu `n-1` | total jumlah simbol dikurangi satu (jadi 0..15 meng-encode 1..16) |

Karena tiap field 4 bit, satu set memuat **maksimal 16 simbol**. Set structured-append selalu punya minimal dua, jadi `n` praktisnya 2..16.

Contoh kerja — simbol 2 dari 4: posisinya `m = 1` (0-based), count-nya `n = 4` jadi nibble count-nya `n - 1 = 3`. Byte indicator-nya `0001 0011` = `0x13`. Simbol 1 dari 4 adalah `0x03`, simbol 4 dari 4 adalah `0x33`.

## 4. Parity byte

Parity byte adalah XOR, byte demi byte, atas **semua nilai byte pesan asli sebelum dibagi** menjadi simbol. Tiap simbol dalam set membawa nilai yang sama.

Untuk library ini pesan aslinya adalah string Go UTF-8, jadi parity-nya adalah XOR `[]byte(text)` atas seluruh input. Misalnya pesan `"AB"` (`0x41`, `0x42`) punya parity `0x41 ^ 0x42 = 0x03`.

Tujuannya integritas, bukan error correction: saat reassemble, decoder menghitung ulang XOR dari teks gabungan dan mengecek ia sama dengan parity yang dilaporkan tiap simbol. Ketidakcocokan berarti ada simbol dari set berbeda yang tercampur, atau sebuah simbol salah-baca — apa pun itu, set tak bisa dipercaya. (Decoder yang cross-check terhadap ZXing sebaiknya menghitung parity atas basis byte yang sama; encoder dan decoder di sini sepakat by construction, dan cek gozxing empiris saat implementasi mengonfirmasi basis byte-nya interoperabel.)

## 5. Split dan reassembly

**Split.** Pesan dibagi menjadi `parts` chunk kontigu pada batas rune — rune multi-byte tak pernah dipotong, jadi tak ada chunk yang memegang UTF-8 invalid. Tiap chunk lalu di-encode sebagai simbolnya sendiri: structured-append header, lalu chunk dijalankan lewat mixed-mode segmenter biasa, lalu terminator dan padding simbol itu. Semua simbol dalam set diukur ke satu version umum (dipilih untuk memuat chunk terbesar plus header-nya) sehingga set tampak seragam.

**Reassembly.** Decoder membaca tiap simbol, mengekstrak posisi, count, dan parity-nya dari header, lalu men-decode sisanya sebagai chunk simbol itu. Ia lalu mengurutkan simbol berdasarkan posisi dan menggabungkan chunk-nya untuk memulihkan pesan asli. Hanya data chunk hasil decode yang digabung: terminator dan padding milik tiap simbol dikonsumsi saat decode-nya dan tak pernah muncul di hasil gabungan. Satu simbol dari sebuah set tetap di-decode sendiri — header di-parse dan dilewati, dan chunk simbol itu dikembalikan — jadi `DecodeBytes` atas satu bagian menghasilkan bagian itu, bukan error.

## 6. Penempatan dan interaksi dengan fitur lain

- **Urutan header.** Structured-append header adalah yang pertama di tiap simbol. ECI header, bila ada, mengikutinya; lalu data segment. v0.10 meng-encode tiap chunk sebagai mixed-mode UTF-8 default dan tak menggabung structured append dengan ECI eksplisit.
- **Independensi per-simbol.** Tiap simbol adalah QR code lengkap yang valid-spec dengan version, mask, error correction, terminator, dan padding sendiri. Structured append hanya menambah header bersama dan koordinasi yang tersirat; tak ada yang berubah pada simbol dasarnya.
- **Version selection per chunk.** Data tiap chunk adalah structured-append header 20-bit-nya plus segment-nya, jadi perhitungan bit version-selection menyertakan header; version umum-nya adalah yang terbesar dibutuhkan di antara chunk-chunk.

## 7. Penunjuk implementasi

- `qrgen/structured_append.go` akan memuat pack/unpack sequence-indicator, parity XOR, dan pembagian batas-rune. `ModeStructuredAppend` (`0011`) bergabung dengan mode indicator di `mode.go`.
- `EncodeStructuredAppend(text, parts, opts...)` di `qrgen/api.go` memvalidasi `parts` (2..16), membagi teks, memilih version umum, lalu mengeluarkan header plus chunk tiap simbol lewat pipeline yang ada, mengembalikan satu PNG per simbol.
- `decodeText` mendapat `case 0b0011`: baca sequence indicator dan parity, lalu decode segment sisanya sebagai chunk. `DecodeStructuredAppend(symbols)` di `qrgen/decode.go` men-decode tiap simbol, mengurutkan berdasarkan posisi, menggabungkan, dan memverifikasi parity bersama serta count yang konsisten.
- Validasi berupa round-trip split-lalu-reassemble (termasuk simbol yang diberikan tak berurut) plus kasus `TestRoundTripWithThirdPartyDecoder` yang memastikan gozxing membaca chunk tiap simbol dan metadata sequence-nya.

## Referensi

- ISO/IEC 18004:2015 — mode indicator Structured Append `0011`, symbol sequence indicator (nibble posisi dan count), parity-data byte serta derivasi XOR-nya, dan aturan reassembly.
- `docs/theory/02-data-encoding.md` dan `docs/theory/20-eci-segments.md` — catatan mode-indicator dan header yang structured append perluas; structured-append header mendahului ECI header saat keduanya ada.
- ZXing / gozxing — penanganan structured-append di decoder independen, dipakai sebagai oracle cross-check dan untuk mengonfirmasi basis byte parity interoperabel.
