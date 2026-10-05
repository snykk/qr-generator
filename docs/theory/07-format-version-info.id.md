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

15 bit ditulis dua kali agar kerusakan pada satu area tidak menghancurkan metadata. Bit 0 adalah bit paling tidak signifikan dari codeword yang sudah di-mask dan bit 14 yang paling signifikan; posisi ditulis `(baris, kolom)` dengan `(0, 0)` di kiri-atas dan `n` adalah sisi simbol.

- **Salinan pertama, mengelilingi finder kiri-atas.** Bit 0–5 turun di kolom 8 pada baris 0–5, bit 6 di `(7, 8)` tepat setelah baris timing, bit 7 di sudut `(8, 8)`, bit 8 di `(8, 7)` tepat setelah kolom timing, dan bit 9–14 bergerak ke kiri di baris 8 sehingga bit 14 jatuh di `(8, 0)`.
- **Salinan kedua, dipecah di antara dua finder lainnya.** Bit 0–7 bergerak dari kanan ke kiri di baris 8 dari `(8, n−1)` sampai `(8, n−8)`, dan bit 8–14 turun di kolom 8 dari `(n−7, 8)` sampai `(n−1, 8)`. Modul di `(n−8, 8)` tepat di atasnya adalah dark module yang selalu gelap, bukan bagian dari salinan ini.

| Bit | 0–5 | 6 | 7 | 8 | 9–14 |
|---|---|---|---|---|---|
| Salinan pertama | `(i, 8)` | `(7, 8)` | `(8, 8)` | `(8, 7)` | `(8, 14−i)` |
| Salinan kedua | `(8, n−1−i)` | `(8, n−7)` | `(8, n−8)` | `(n−7, 8)` | `(n−15+i, 8)` |

Jadi kedua salinan membawa word 15-bit yang sama, dan itulah yang memungkinkan reader memulihkan format dari salah satunya. Posisi ini mengikuti ISO/IEC 18004:2015 section 7.9.1 dan cocok dengan `TYPE_INFO_COORDINATES` milik ZXing, segno, serta output zint; `qrgen/testdata/thirdparty` mengunci posisi ini lewat simbol dari encoder independen.

Sebelum v0.14.1, library ini menulis dan membaca salinan pertama secara terbalik urutan bitnya, menaruh bit `i` di tempat yang menurut tabel milik bit `14−i`. Karena encoder dan decoder berbagi kesalahan yang sama, setiap round trip internal lolos, dan reader lain tetap bisa memindai simbol kita lewat salinan kedua yang utuh, sehingga tak ada yang menyadarinya; tetapi redundansinya hilang, dan decoder tidak bisa membaca 14 dari 32 format word yang ditulis encoder lain. Pelajarannya berlaku umum: uji decoder dengan simbol yang tidak ia hasilkan sendiri.

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

- ISO/IEC 18004:2015, section 7.9 (Format information), section 7.10 (Version information), Annex C dan D. Referensi lama, termasuk JIS X 0510:2004 dan source ZXing, menomori kedua section ini 8.9 dan 8.10.
- Bose, R. C. and Ray-Chaudhuri, D. K., "On a Class of Error Correcting Binary Group Codes," *Information and Control*, 3(1), 1960, pp. 68–79.
