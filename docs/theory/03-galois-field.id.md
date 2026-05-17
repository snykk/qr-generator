# Galois Field GF(256)

Error correction Reed–Solomon untuk QR berjalan di atas finite field **GF(2⁸)**, ditulis GF(256). Dokumen ini menyajikan aljabar secukupnya agar implementasi tetap benar.

> Versi English: [03-galois-field.md](03-galois-field.md).

## Apa itu finite field

Sebuah *field* adalah himpunan dengan operasi penjumlahan, pengurangan, perkalian, dan pembagian (oleh elemen non-nol) yang memenuhi aksioma aljabar standar. Finite field adalah field yang himpunan elemennya terhingga; untuk tiap prime power `pⁿ` terdapat satu field berorde `pⁿ` (unique up to isomorphism). Untuk QR kita pakai `p = 2`, `n = 8`, sehingga field memiliki 256 elemen yang pas dalam satu byte.

## Representasi

Elemen GF(2⁸) adalah polinomial di `x` berderajat ≤ 7 dengan koefisien di GF(2) (yaitu 0 atau 1). Byte `b₇b₆b₅b₄b₃b₂b₁b₀` merepresentasikan:

    b₇·x⁷ + b₆·x⁶ + … + b₁·x + b₀

Jadi `0x57` (`01010111`) adalah `x⁶ + x⁴ + x² + x + 1`.

## Penjumlahan dan pengurangan

Penjumlahan di GF(2) adalah XOR (1 + 1 = 0). Pengurangan sama dengan penjumlahan karena setiap elemen merupakan invers aditif dirinya sendiri.

```text
add(a, b) = a XOR b
sub(a, b) = a XOR b
```

## Perkalian

Perkalian adalah polynomial multiplication modulo sebuah *primitive polynomial*. QR memakai:

    p(x) = x⁸ + x⁴ + x³ + x² + 1   (biner 0x11D)

Implementasi yang benar tapi lambat menggunakan schoolbook multiply, mengurangi bit tinggi pada setiap shift:

```text
function mul(a, b):
    r = 0
    while b != 0:
        if b AND 1:
            r = r XOR a
        a = a SHL 1
        if a AND 0x100:
            a = a XOR 0x11D
        b = b SHR 1
    return r
```

Dalam praktik, dua tabel di-precompute agar perkalian dan pembagian menjadi lookup O(1).

## Tabel exp/log dan generator α

Elemen `α = 0x02` (polinomial `x`) adalah *generator* dari multiplicative group: pangkat berurutannya `α⁰, α¹, …, α²⁵⁴` menyebutkan seluruh 255 elemen non-nol.

Definisikan:

```text
exp[i] = αⁱ        untuk i di 0..254
log[αⁱ] = i        untuk i di 0..254
```

Maka untuk `a`, `b` non-nol:

```text
a · b = exp[(log[a] + log[b]) mod 255]
a / b = exp[(log[a] − log[b] + 255) mod 255]
```

`log[0]` tidak terdefinisi; implementasi harus menjaga input ini.

## Contoh: α³ · α⁵

- `log[α³] = 3`, `log[α⁵] = 5`.
- Jumlah = 8.
- Hasil = `α⁸ = exp[8]`.

Menghitung `exp[8]` langkah demi langkah dari `α¹ = 0x02`:

```text
α¹ = 0x02
α² = 0x04
α³ = 0x08
α⁴ = 0x10
α⁵ = 0x20
α⁶ = 0x40
α⁷ = 0x80
α⁸ = (0x80 SHL 1) = 0x100 → reduksi dengan 0x11D → 0x100 XOR 0x11D = 0x1D
```

Maka `α³ · α⁵ = 0x1D`. Verifikasi via perkalian polinomial langsung: `x³ · x⁵ = x⁸`, direduksi modulo `p(x)` menghasilkan `x⁴ + x³ + x² + 1 = 0x1D`. ✓

## Polinomial di atas GF(256)

Reed–Solomon membutuhkan polinomial yang koefisiennya juga elemen GF(256). Polinomial direpresentasikan sebagai `[]byte`, koefisien indeks `i` dipasangkan dengan `x^(deg − i)` (derajat tertinggi di depan). Perkalian polinomial adalah konvolusi biasa, hanya saja `+` diganti `XOR` dan `*` skalar diganti `mul` GF(256) di atas.

## Penunjuk implementasi

- `qrgen/gf256.go` akan memuat tabel `exp` dan `log` yang diinisialisasi di `init()`, helper skalar `mul`/`div`/`pow`, serta helper polinomial `polyMul` dan `polyMod` untuk dipakai `qrgen/reedsolomon.go`.
- Tabel dibangun sekali dan diperlakukan sebagai immutable.

## Referensi

- ISO/IEC 18004:2015, §8.5 (Error correction coding).
- Reed, I. S. and Solomon, G., "Polynomial Codes over Certain Finite Fields," *J. SIAM*, 8(2), 1960, pp. 300–304.
- Plank, J. S., "A Tutorial on Reed–Solomon Coding for Fault-Tolerance in RAID-like Systems," *Software – Practice and Experience*, 27(9), 1997 — pengantar dengan contoh-contoh terhitung.
