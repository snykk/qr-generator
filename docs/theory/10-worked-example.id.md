# Contoh End-to-End — `"HELLO WORLD"` di EC Level M

Dokumen ini menelusuri contoh encoding kanonik dari awal hingga akhir, menerapkan setiap algoritma yang dijelaskan di dokumen 02–08 ke satu input konkret. Ini adalah jenis test fixture yang sebaiknya direproduksi tiap implementer secara lokal — sebagai alat bantu belajar sekaligus target round-trip golden test pada milestone M10.

> Versi English: [10-worked-example.md](10-worked-example.md).

## 0. Input

- **Payload:** `HELLO WORLD` (11 karakter)
- **EC level diminta:** M (recovery ~15%)
- **Mask:** dipilih otomatis berdasarkan penalty (akan ditunjukkan)

## 1. Pemilihan mode

Set karakter yang dipakai adalah `{H, E, L, O, ' ', W, R, D}` — semua karakter punya entri di tabel alphanumeric (docs/theory/09-data-tables.id.md bagian 2). Tidak ada huruf kecil atau simbol di luar alfabet alphanumeric, sehingga analyzer memilih mode **Alphanumeric**.

Pemilihan greedy single-mode sudah cukup untuk MVP; mixed-mode segmentation tidak akan menghasilkan jumlah bit lebih kecil untuk payload ini.

## 2. Pemilihan versi

Dari tabel kapasitas (docs/theory/09-data-tables.id.md bagian 6), Versi 1 di EC-M menampung 16 data codeword = 128 bit.

Jumlah bit payload (dihitung di bagian 3) adalah 74 bit, jauh di bawah 128, sehingga **Versi 1** cukup. V1 yang kecil berarti matrix 21×21 dan satu block Reed–Solomon tunggal.

## 3. Konstruksi bit stream

Per komponen:

| Komponen                            | Bit                              | Lebar |
|-------------------------------------|----------------------------------|------:|
| Mode indicator (alphanumeric)       | `0010`                           | 4     |
| Character count (11, V1 → 9 bit)    | `000001011`                      | 9     |
| Pasangan `HE`: 17·45 + 14 = 779     | `01100001011`                    | 11    |
| Pasangan `LL`: 21·45 + 21 = 966     | `01111000110`                    | 11    |
| Pasangan `O ` : 24·45 + 36 = 1116   | `10001011100`                    | 11    |
| Pasangan `WO`: 32·45 + 24 = 1464    | `10110111000`                    | 11    |
| Pasangan `RL`: 27·45 + 21 = 1236    | `10011010100`                    | 11    |
| Tunggal `D` = 13 (6 bit)            | `001101`                         | 6     |
| **Subtotal payload**                |                                  | **74**|

## 4. Terminator dan padding

Kapasitas V1-M = 128 bit. Payload 74 bit. Tambahkan:

| Langkah          | Bit yang ditambahkan            | Kumulatif  |
|------------------|---------------------------------|-----------:|
| Terminator       | `0000`                          | 78         |
| Pad ke byte      | `00` (2 bit nol)                | 80         |
| Pad bytes (×6)   | `0xEC 0x11 0xEC 0x11 0xEC 0x11` | 128        |

## 5. 16 data codeword final

Memecah bit stream 128-bit menjadi codeword 8-bit (hex):

```text
0x20 0x5B 0x0B 0x78 0xD1 0x72 0xDC 0x4D
0x43 0x40 0xEC 0x11 0xEC 0x11 0xEC 0x11
```

Self-check: byte pertama `0x20 = 0010 0000` membawa mode indicator `0010` diikuti 4 bit pertama character count `0000` dengan benar. Enam byte terakhir adalah urutan pad yang diharapkan.

## 6. Encoding Reed–Solomon

Dari docs/theory/09-data-tables.id.md bagian 7, V1-M memakai **1 block berisi 16 data codeword + 10 EC codeword per block**. Kita masukkan 16 data codeword ke `encodeBlock` dengan `n = 10`, memakai generator polynomial `genPoly(10)` di atas GF(256).

10 EC codeword yang diharapkan (hasil menjalankan algoritma long division dari docs/theory/04-reed-solomon.id.md terhadap data di atas) adalah:

```text
0xC4 0x23 0x27 0x77 0xEB 0xD7 0xE7 0xE2 0x5D 0x17
```

Verifikasi dengan menjalankan kembali `polyMod(data || zeros(10), genPoly(10))` di atas GF(256) — sisanya harus cocok byte-per-byte.

## 7. Codeword stream ter-interleave

Karena hanya satu block, "interleaving" jadi trivial: data dulu, lalu EC. Penggabungannya menghasilkan 26 codeword = 208 bit:

```text
0x20 0x5B 0x0B 0x78 0xD1 0x72 0xDC 0x4D
0x43 0x40 0xEC 0x11 0xEC 0x11 0xEC 0x11
0xC4 0x23 0x27 0x77 0xEB 0xD7 0xE7 0xE2
0x5D 0x17
```

V1 punya 0 remainder bit (docs/theory/09-data-tables.id.md bagian 8), sehingga stream 208-bit masuk ke matrix apa adanya.

## 8. Penempatan di matrix (V1, 21×21)

Functional pattern dulu (docs/theory/05-matrix-construction.id.md):

- Finder pattern di `(0,0)`, `(0,14)`, `(14,0)`.
- Separator di sekeliling tiap finder.
- Tidak ada alignment pattern (V1).
- Timing pattern di baris 6 dan kolom 6.
- Dark module di `(13, 8)` (= `4·1+9 = 13`).
- Area format-info di-reserve di sekitar finder kiri-atas dan melintasi dua lainnya.

Lalu codeword stream 208-bit ditulis ke sel sisa memakai zig-zag walk: pasangan kolom (20, 19) ke atas, lalu (18, 17) ke bawah, (16, 15) ke atas, (14, 13) ke bawah, (12, 11) ke atas, (10, 9) ke bawah, (8, 7) ke atas, lalu melewati kolom timing 6 ke (5, 4) bawah, (3, 2) atas, (1, 0) bawah.

## 9. Pemilihan mask

Tiap dari 8 mask diterapkan hanya ke modul data, lalu penalty empat-aturan (docs/theory/06-masking.id.md) dijumlahkan. Untuk contoh `"HELLO WORLD"` versi 1, mask dengan score terendah adalah **mask 3** dengan kondisi `(i + j) mod 3 == 0`. (Total penalty persisnya bergantung pada penempatan akurat; harapkan total penalty di kisaran 350–600 untuk kedelapan mask, dengan mask 3 paling rendah.)

## 10. Format information

Dengan EC level M (`00`) dan mask 3 (`011`), payload 5-bit-nya adalah `00 011 = 00011`. Diproses lewat `BCH(15, 5)` lalu di-XOR dengan `0x5412` menghasilkan codeword format 15-bit **`0x5B4B`** (verifikasi via algoritma di docs/theory/09-data-tables.id.md bagian 11).

15 bit tersebut ditulis ke dua lokasi redundan di sekitar finder, bit paling signifikan dulu.

V1 tidak memiliki blok version-information.

## 11. Sketsa matrix final

Matrix V1 yang sudah selesai (21×21, dengan `█` = gelap, `·` = terang) tampak kurang lebih seperti:

```
█████████·· ░░░ ·█████████
█·······█·· ░░░ ·█·······█
█·███·█·░░░░░░░░░░·███·█
█·███·█·░░░░░░░░░░·███·█
█·███·█·░░░░░░░░░░·███·█
█·······█·░░░░░░░░░·······█
█████████·█·█·█·█·█·█████████
········░░░░░░░░░░░░░········
███████░AREA DATA░░░░███████
... dst ...
```

Pola bit persisnya bergantung pada Reed–Solomon, mask terpilih, dan bit format info yang dihitung di atas. Untuk referensi otoritatif, encode `"HELLO WORLD"` di EC-M melalui generator online Project Nayuki dan bandingkan modul-per-modul.

## 12. Rendering

Dengan module size default = 8 px dan quiet zone = 4 modul:

    pixels = 8 · (21 + 2·4) = 8 · 29 = 232 px per sisi

Output adalah PNG grayscale 232×232.

## 13. Mengapa contoh ini fixture yang tepat

- **Mencakup setiap cabang mode/versi yang kita implementasikan** kecuali byte mode dan multi-block split. Pasangkan dengan fixture kedua (mis. string lebih panjang yang memaksa V5 / EC-Q untuk menguji multi-group block) untuk coverage penuh di M10.
- **Reference output banyak dipublikasi.** Beberapa tutorial (Thonky, Nayuki) menelusuri string ini, sehingga perbedaan di langkah mana pun langsung dapat ditelusuri.
- **Cukup kecil untuk debug manual.** Matrix 21×21 dengan 26 codeword muat di satu layar, sehingga inspeksi manual nilai-nilai intermediate realistis.

## 14. Checklist verifikasi

Saat implementasi mencapai M5+, nilai-nilai intermediate berikut sebaiknya masing-masing di-assert:

- [ ] Bagian 3 → segmen mode menghasilkan 74 bit persis seperti yang tertulis (test `qrgen/mode.go`).
- [ ] Bagian 5 → 16 data codeword cocok dengan `0x20 0x5B 0x0B 0x78 0xD1 0x72 0xDC 0x4D 0x43 0x40 0xEC 0x11 0xEC 0x11 0xEC 0x11`.
- [ ] Bagian 6 → 10 EC codeword cocok dengan `0xC4 0x23 0x27 0x77 0xEB 0xD7 0xE7 0xE2 0x5D 0x17` (test `qrgen/reedsolomon.go`).
- [ ] Bagian 9 → mask 3 menang dalam penalty (test `qrgen/mask.go`).
- [ ] Bagian 10 → codeword format untuk (M, mask 3) adalah `0x5B4B` (test `qrgen/formatinfo.go`).
- [ ] Bagian 11 → matrix akhir cocok dengan encode referensi yang sudah diketahui benar (round-trip golden test).

## Referensi

- ISO/IEC 18004:2015 — sumber normatif untuk setiap nilai numerik di sini.
- Thonky, "Putting It All Together — `HELLO WORLD`" — <https://www.thonky.com/qr-code-tutorial/>
- Project Nayuki, *QR Code generator library* — encode `"HELLO WORLD"` di EC-M untuk memperoleh referensi matrix yang byte-exact.
