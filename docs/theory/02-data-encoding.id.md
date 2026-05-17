# Encoding Data

Dokumen ini menjelaskan bagaimana sebuah string input diubah menjadi bit stream yang siap masuk tahap Reed–Solomon. Dokumen ini mengacu pada ISO/IEC 18004:2015 bagian 7.3.

> Versi English: [02-data-encoding.md](02-data-encoding.md).

## Mode encoding

ISO/IEC 18004 mendefinisikan empat mode standar; kita mengimplementasikan tiga yang pertama:

| Mode         | Mode indicator | Karakter yang didukung                         |
|--------------|:--------------:|------------------------------------------------|
| Numeric      | `0001`         | `0–9` (10 karakter)                            |
| Alphanumeric | `0010`         | `0–9 A–Z $ % * + - . / : space` (45 karakter)  |
| Byte         | `0100`         | Sembarang nilai 8-bit (kita kirim raw UTF-8)   |
| Kanji        | `1000`         | Shift-JIS double-byte (out of scope v0.1)      |

ECI (mode indicator `0111`) dan structured-append (`0011`) juga out of scope untuk v0.1.

### Encoding numeric

Kelompokkan digit per tiga, lalu encode tiap kelompok sebagai integer 10-bit. Kelompok ekor dua digit di-encode sebagai 7 bit; satu digit sebagai 4 bit.

Contoh `"01234567"`:

- Kelompok: `012 | 345 | 67`.
- Hasil: `0000001100 | 0101011001 | 1000011` → total 27 bit.

### Encoding alphanumeric

Tiap karakter dipetakan ke integer `0..44` (`A`=10, `B`=11, …, `space`=36, `$`=37, dst.). Proses dalam pasangan; tiap pasangan dipadatkan ke 11 bit sebagai `value(a) · 45 + value(b)`. Sisa satu karakter dipadatkan ke 6 bit.

Contoh `"HELLO WORLD"` (11 karakter): 5 pasangan + 1 sisa → 5 × 11 + 6 = **61 bit** untuk payload.

### Encoding byte

Tiap byte input (UTF-8) menjadi 8 bit, ditulis MSB lebih dulu. Kita mengasumsikan UTF-8 dan **tidak** mengirim segmen ECI untuk mendeklarasikannya; decoder modern umumnya menebak UTF-8 dengan benar, tapi ini adalah non-conformance yang diketahui dan dicatat di README.

## Character count indicator

Setelah 4-bit mode indicator, kita kirim *character count indicator* yang lebarnya bergantung pada rentang versi dan mode:

| Versi   | Numeric | Alphanumeric | Byte |
|:-------:|:-------:|:------------:|:----:|
|  1–9    |   10    |      9       |   8  |
| 10–26   |   12    |     11       |  16  |
| 27–40   |   14    |     13       |  16  |

Sumber: ISO/IEC 18004:2015, Table 3.

## Memilih versi terkecil

Diberikan payload dan EC level yang diminta, versi terkecil adalah `v` minimum yang kapasitas data-codeword-nya (dikalikan 8 menjadi bit) setidaknya sama dengan panjang encoded. Panjang encoded sendiri bergantung pada `v` melalui lebar character count indicator, sehingga pencariannya iteratif: mulai dari `v = 1`, hitung panjang encoded memakai indicator versi tersebut, maju selama kapasitas belum cukup.

## Terminator dan padding

Setelah bit stream payload terbentuk, ada tiga langkah penutup:

1. **Terminator** — tambahkan hingga 4 bit nol, dipotong jika sisa kapasitas lebih kecil sebelum batas byte.
2. **Bit padding** — pad dengan nol hingga batas byte berikutnya.
3. **Pad bytes** — isi sisa kapasitas dengan dua byte bergantian `0xEC 0x11` (`11101100`, `00010001`).

## Mengapa pilihan-pilihan ini

- Encoder per-mode memanfaatkan alfabet yang terbatas: numeric rata-rata 3,33 bit/karakter, alphanumeric rata-rata 5,5 bit/karakter, byte 8 bit.
- Lebar character count indicator dipilih agar count maksimum per rentang versi pas dengan lebar bit indicator — spec menyeimbangkan overhead payload dengan rentang kapasitas.
- Pad byte `0xEC 0x11` ditentukan spec karena pola bit-nya (`11101100` dan `00010001`) cukup berimbang antara modul gelap-terang, sehingga mengurangi peluang penalty mask yang tinggi karena ekor monoton.

## Penunjuk implementasi

- `qrgen/mode.go` akan memuat encoder per-mode dan mode analyzer.
- `qrgen/version.go` akan memuat tabel kapasitas dan pencarian versi terkecil.
- Untuk v0.1 mode analyzer cukup greedy single-segment: pilih mode paling ketat yang menutup semua karakter input. Segmentasi mode campuran (DP kecil) ditunda.

## Referensi

- ISO/IEC 18004:2015, §7.3 dan Table 2–7.
- Thonky, "Data Encoding" — <https://www.thonky.com/qr-code-tutorial/data-encoding>
- Project Nayuki, *QR Code generator library* — pseudo-code mode analyzer.
