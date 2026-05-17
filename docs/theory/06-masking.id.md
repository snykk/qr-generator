# Masking

Setelah penempatan data, matrix bisa mengandung run modul sewarna yang panjang atau pola mirip-finder di area data, dan kedua hal ini dapat membingungkan decoder. *Masking* mengacak bit data untuk memecah pola tersebut sambil tetap reversibel sempurna.

> Versi English: [06-masking.md](06-masking.md).

## 8 mask pattern

Sebuah *mask* adalah fungsi dari `(row, column)`. Untuk tiap modul area data, fungsi mask mengembalikan boolean; jika `true`, modul tersebut diinversi (di-toggle). ISO/IEC 18004:2015 §8.8.1 mendefinisikan 8 mask:

| Mask | Kondisi (`i = row`, `j = column`)            |
|:----:|:---------------------------------------------|
|  0   | `(i + j) mod 2 == 0`                         |
|  1   | `i mod 2 == 0`                               |
|  2   | `j mod 3 == 0`                               |
|  3   | `(i + j) mod 3 == 0`                         |
|  4   | `(floor(i/2) + floor(j/3)) mod 2 == 0`       |
|  5   | `((i·j) mod 2) + ((i·j) mod 3) == 0`         |
|  6   | `(((i·j) mod 2) + ((i·j) mod 3)) mod 2 == 0` |
|  7   | `(((i+j) mod 2) + ((i·j) mod 3)) mod 2 == 0` |

Mask di-XOR-kan **hanya** ke modul area data — tidak pernah ke finder, alignment, timing, format, atau area version.

## Kenapa XOR aman

XOR adalah invers dirinya sendiri. Decoder membaca bit format-information yang menunjuk mask mana yang dipakai, menerapkan fungsi mask yang sama, lalu memulihkan bit data asli. Tidak ada informasi yang hilang karena masking adalah operasi simetris dan bijektif.

## Evaluasi penalty

Kita coba kedelapan mask dan pilih yang menghasilkan *penalty score* terendah. Score adalah jumlah dari empat kontribusi (ISO/IEC 18004:2015 §8.8.2).

### Aturan 1 — run modul sewarna

Untuk tiap baris dan tiap kolom, cari run berisi 5 modul sewarna atau lebih. Sebuah run panjang `L ≥ 5` menyumbang `L − 2` ke score. Jadi run panjang 5 menambah 3, panjang 6 menambah 4, dan seterusnya.

### Aturan 2 — block 2×2

Setiap block 2×2 modul sewarna menambah **3** poin. Overlap dihitung: area 3×3 yang seluruhnya gelap mengandung empat sub-block 2×2, menambah 12 poin.

### Aturan 3 — pola mirip-finder

Setiap kemunculan pola `1 0 1 1 1 0 1 0 0 0 0` — atau kebalikannya `0 0 0 0 1 0 1 1 1 0 1` — di baris atau kolom mana pun menambah **40** poin. Pola 11-modul ini menyerupai finder + separator dan akan membingungkan decoder jika dibiarkan di area data.

### Aturan 4 — rasio modul gelap

Misalkan `r` adalah persentase modul gelap di seluruh simbol. Hitung:

    k = floor( |r − 50| / 5 )

Penalty-nya adalah `10 · k`. Matrix yang seimbang sempurna mendapat 0 poin dari aturan ini; setiap deviasi 5% dari 50/50 menambah 10 poin.

## Memilih mask terbaik

Untuk tiap mask `k` di `0..7`:

1. Clone matrix.
2. Terapkan mask `k` hanya ke modul data.
3. Tulis bit format-info yang dihitung untuk `(EC level, k)` (lihat [07-format-version-info.id.md](07-format-version-info.id.md)).
4. Jumlahkan keempat kontribusi penalty.

Mask dengan total score minimum menang. Tie diputuskan dengan index mask terkecil.

## Tips praktis

- Untuk v0.1 cukup pass O(n²) per aturan; matrix cukup kecil sehingga incremental evaluation tidak sebanding kompleksitasnya.
- Bangun masked matrix in place tapi simpan salinan matrix asli bila ingin mengevaluasi beberapa mask berdampingan, atau ingat untuk membatalkan XOR sebelum mencoba mask berikutnya.

## Penunjuk implementasi

- `qrgen/mask.go`: `applyMask(m *Matrix, k int)`, `penalty(m *Matrix) int`, `selectMask(m *Matrix) (k int, masked *Matrix)`.

## Referensi

- ISO/IEC 18004:2015, §8.8 (Masking).
- Thonky, "Data Masking" — <https://www.thonky.com/qr-code-tutorial/data-masking>
