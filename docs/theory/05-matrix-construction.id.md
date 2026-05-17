# Konstruksi Matrix

Tahap ini mengambil codeword bit stream yang sudah ter-interleave dan menyusunnya ke dalam grid modul QR, berdampingan dengan functional pattern yang sudah baku.

> Versi English: [05-matrix-construction.md](05-matrix-construction.md).

## Sistem koordinat

Kita pakai `(row, column)` dengan `(0, 0)` di pojok kiri-atas, sesuai orientasi saat render ke image. Matrix berbentuk persegi dengan sisi `n = 21 + 4·(v − 1)` modul.

## Functional pattern

### Finder pattern

Finder adalah pola 7×7:

```
1 1 1 1 1 1 1
1 0 0 0 0 0 1
1 0 1 1 1 0 1
1 0 1 1 1 0 1
1 0 1 1 1 0 1
1 0 0 0 0 0 1
1 1 1 1 1 1 1
```

Ditempatkan di:

- kiri-atas pada `(0, 0)`,
- kanan-atas pada `(0, n − 7)`,
- kiri-bawah pada `(n − 7, 0)`.

### Separator

Strip terang selebar satu modul mengelilingi tiap finder pada sisi menghadap area data. Bersama-sama, finder ditambah separator menempati area 8×8.

### Timing pattern

Baris 6 dan kolom 6 diisi modul gelap-terang berselang-seling, dimulai dari gelap, di area antara finder. Mereka ditulis *setelah* finder agar sudut tetap sesuai pola finder.

### Alignment pattern

Pola konsentris 5×5:

```
1 1 1 1 1
1 0 0 0 1
1 0 1 0 1
1 0 0 0 1
1 1 1 1 1
```

Ditempatkan dengan pusat pada koordinat yang tercantum di ISO/IEC 18004:2015 Annex E. Versi 2 hingga 40 memiliki 1 sampai 46 alignment pattern. Aturan praktisnya: alignment ditempatkan saat pusat baris dan pusat kolom berasal dari daftar koordinat per-versi, *kecuali* posisi yang akan bentrok dengan finder pattern.

### Dark module

Sebuah modul gelap tunggal di `(4·v + 9, 8)` untuk setiap versi. Modul ini termasuk area format-information tapi selalu di-set gelap oleh spec.

### Area reserved

Dua strip 15-bit di sekitar finder kiri-atas, dan (untuk v ≥ 7) dua blok 6×3 di samping finder kanan-atas dan kiri-bawah, di-reserve untuk format dan version information. Area ini harus di-mark sebelum penempatan data agar data bit stream tidak menumpukinya.

## Menempatkan bit data

Setelah functional pattern berada di tempatnya, data stream disisipkan ke modul sisa dengan aturan berikut (ISO/IEC 18004:2015 §8.7.3):

1. Berjalan **ke atas** dalam band dua-kolom mulai dari tepi kanan.
2. Di dalam tiap band, tulis kolom kanan dulu lalu kolom kiri untuk tiap baris.
3. Saat mencapai puncak, geser dua kolom ke kiri lalu berjalan **ke bawah**.
4. Ulangi sampai seluruh modul non-fungsional terjamah.
5. **Kolom 6** (timing pattern vertikal) dilewati seluruhnya — band yang seharusnya melaluinya digeser satu kolom ke kiri.
6. Lewati modul yang menjadi bagian functional atau reserved area.

Bit paling signifikan dari codeword stream ditulis pertama, ke modul pertama yang dijumpai walk.

Untuk matrix versi 1, walk mulai dari pasangan kolom (20, 19), mengunjungi baris 20 turun ke 0; lalu (18, 17); lalu (16, 15); lalu (14, 13); lalu (12, 11); lalu (10, 9); lalu (8, 7); lalu (5, 4) (karena kolom 6 dilewati); lalu (3, 2); lalu (1, 0).

## Mengapa skema ini

- Finder berukuran besar dan asimetris agar decoder bisa mendeteksi orientasi terlepas dari posisi simbol dipegang.
- Alignment pattern membatasi distorsi perspektif pada simbol yang lebih besar, dengan harga sekumpulan modul yang permanen off-limits.
- Timing pattern bersifat predictable, memberi referensi kalibrasi pitch modul bahkan ketika permukaan melengkung.
- Zig-zag walk mengunjungi tiap modul non-fungsional persis sekali, sehingga decoder dapat membaliknya secara deterministik.

## Penunjuk implementasi

- `qrgen/matrix.go`: struct `Matrix` yang membungkus `[][]bool` ditambah mask "reserved" `[][]bool` paralel yang menandai modul fungsional dan reserved.
- Tempatkan functional pattern dulu; jalankan data walk; baru kemudian terapkan masking dan tulis format/version info ke area reserved.

## Referensi

- ISO/IEC 18004:2015, §6.3 (Symbol structure), §8.7 (Codeword placement in matrix), Annex E (Alignment pattern positions).
- Thonky, "Module Placement" — <https://www.thonky.com/qr-code-tutorial/module-placement-matrix>
