# Format & Version Information

Dua pesan singkat — satu tentang EC level dan mask, satu tentang versi — ditulis ke modul reserved agar decoder dapat mengkonfigurasi diri sebelum membaca payload. Keduanya kode BCH, dan codeword format-info dikenakan XOR mask tetap di akhir.

> Versi English: [07-format-version-info.md](07-format-version-info.md).

## Format information — BCH(15, 5)

### Payload

5 bit, urutan:

- 2 bit: EC level (`L=01`, `M=00`, `Q=11`, `H=10`).
- 3 bit: indeks mask pattern `0..7`.

Jadi untuk EC level `M` dan mask `5`, payload-nya adalah `00 101` = `00101`.

### Encoding BCH

Hitung `payload · x¹⁰` modulo `g(x) = x¹⁰ + x⁸ + x⁵ + x⁴ + x² + x + 1` (biner `10100110111`, hex `0x537`). Gabungkan `payload (5 bit) || sisa (10 bit)` menjadi codeword 15-bit.

### Mask

XOR-kan codeword 15-bit dengan `101010000010010` (`0x5412`). XOR ini mencegah payload semua-nol menghasilkan format string semua-nol, yang akan melanggar persyaratan spec bahwa area format tidak boleh uniform.

### Penempatan

15 bit ditulis di dua lokasi:

- Mengelilingi **finder kiri-atas** dalam bentuk L.
- Melintasi finder **kanan-atas** (baris 8) dan finder **kiri-bawah** (kolom 8), dipecah antara keduanya agar kerusakan satu sudut tidak menghancurkan metadata.

Urutan: bit 14 (paling signifikan) lebih dulu. Posisi modul per bit secara detail ada di ISO/IEC 18004:2015 §8.9.

## Version information — BCH(18, 6)

Hadir hanya untuk **versi 7 ke atas**.

### Payload

6 bit yang mengencode nomor versi `7..40`, dalam binary unsigned, bit paling signifikan di depan.

### Encoding BCH

Hitung `payload · x¹²` modulo `g(x) = x¹² + x¹¹ + x¹⁰ + x⁹ + x⁸ + x⁵ + x² + 1` (biner `1111100100101`, hex `0x1F25`). Gabungkan `payload (6) || sisa (12)` menjadi codeword 18-bit.

Tidak ada XOR mask diterapkan.

### Penempatan

Dua blok 6×3:

- Blok `6 × 3` tepat di atas finder kiri-bawah, baris `n − 11..n − 9` dan kolom `0..5`.
- Blok `3 × 6` tepat di sebelah kiri finder kanan-atas, baris `0..5` dan kolom `n − 11..n − 9`.

18 bit yang sama ditulis ke dua blok tersebut tapi dengan orientasi transposed. Pemetaan bit-ke-modul yang persis ada di ISO/IEC 18004:2015 Annex D.

## Tabel lookup yang di-precompute

Total enumerasinya kecil: 32 codeword format-info (4 EC level × 8 mask) dan 34 codeword version-info (versi 7 sampai 40). Kita precompute kedua tabel sekali dan tinggal lookup saat runtime; ini membuat encode path tidak perlu melakukan aritmatika polinomial untuk kode-kode kecil ini.

## Penunjuk implementasi

- `qrgen/formatinfo.go`: tabel `formatInfo[ecLevel][mask] uint16` dan `versionInfo[version] uint32` yang di-precompute, plus fungsi placement yang menulis bit-bit tersebut ke matrix.

## Referensi

- ISO/IEC 18004:2015, §8.9 (Format information), §8.10 (Version information), Annex C dan D.
- Bose, R. C. and Ray-Chaudhuri, D. K., "On a Class of Error Correcting Binary Group Codes," *Information and Control*, 3(1), 1960, pp. 68–79.
