# Error Correction Reed–Solomon

Kode Reed–Solomon (RS) adalah mekanisme redundansi yang membuat QR code tetap terbaca meski sebagian modulnya hilang atau rusak. Dokumen ini menjelaskan bagaimana QR menerapkan RS — encoder saja, karena library ini tidak mengimplementasikan decoder.

> Versi English: [04-reed-solomon.md](04-reed-solomon.md).

## RS dalam satu paragraf

Kode RS di atas GF(q) memperlakukan `k` simbol sumber sebagai koefisien polinomial pesan `M(x)`, mengalikannya dengan `xⁿ` (di mana `n` adalah jumlah simbol EC), membagi hasilnya dengan *generator polynomial* `g(x)`, dan menambahkan sisa pembagian. Polinomial hasil berderajat kurang dari `k + n` dan menjadi codeword. Karena akar generator adalah pangkat berurutan dari `α`, kode mampu mengoreksi hingga `n / 2` symbol error per blok. Untuk QR `q = 256`, sehingga tiap simbol adalah satu byte (disebut *codeword*).

## Generator polynomial

Untuk `n` EC codewords, generator-nya adalah:

    g(x) = (x − α⁰)(x − α¹)(x − α²) … (x − α^(n−1))

dikalikan habis di atas GF(256). Contoh (koefisien ditulis sebagai pangkat α, dari leading term ke bawah):

- `n = 7`:  `x⁷ + α⁸⁷x⁶ + α²²⁹x⁵ + α¹⁴⁶x⁴ + α¹⁴⁹x³ + α²³⁸x² + α¹⁰²x + α²¹`
- `n = 10`: `x¹⁰ + α²¹⁵x⁹ + α¹⁹⁴x⁸ + …`

Dalam kode, `g` dibangun iteratif: mulai dari `[1]`, lalu untuk tiap `i` di `0..n−1` kalikan polinomial saat ini dengan `(x − αⁱ)`. Karena pengurangan = penjumlahan di GF(2⁸), bentuknya menjadi `(x + αⁱ)`.

## Mengencode satu blok

Diberikan satu blok berisi `k` data codewords:

1. Padding `n` zero codeword di kanan → polinomial berderajat `k + n − 1`.
2. Bagi dengan `g(x)` menggunakan polynomial long division di atas GF(256).
3. Sisa pembagian, berderajat `< n`, adalah EC codewords blok tersebut.

Pseudo-code:

```text
function encodeBlock(data[0..k-1], g[0..n]):
    result = data ++ zeros(n)
    for i in 0..k-1:
        coef = result[i]
        if coef != 0:
            for j in 0..n:
                result[i + j] = result[i + j] XOR mul(g[j], coef)
    return result[k..k+n-1]   # sisa pembagian
```

## Struktur block khusus QR

Untuk sebagian besar kombinasi (versi, EC level), codeword dibagi ke dalam dua kelompok block dengan dua ukuran data berbeda. ISO/IEC 18004:2015 Table 9 menyebutkan untuk tiap (versi, EC level):

- jumlah EC codewords per block (sama untuk semua block dalam pasangan (versi, EC level)),
- group 1: jumlah block dan data codewords per block,
- group 2: jumlah block dan data codewords per block (nol jika tidak dipakai).

Contoh — versi 5 / EC level Q:

| Group | Blocks | Data codewords / block | EC codewords / block |
|:-----:|:------:|:----------------------:|:--------------------:|
|   1   |   2    |          15            |          18          |
|   2   |   2    |          16            |          18          |

Total data = `2·15 + 2·16 = 62` codeword. EC budget = `4 · 18 = 72` codeword. Encoder harus memecah byte stream encoded sesuai pembagian ini lalu menjalankan encoder per-block pada setiap potongan.

## Interleaving

Setelah encoding per-block kita memiliki, misalnya, empat data block D₁..D₄ dan empat EC block E₁..E₄. Urutan transmisi final adalah *column-major* atas block:

```text
D₁[0], D₂[0], D₃[0], D₄[0],
D₁[1], D₂[1], D₃[1], D₄[1],
...
D₁[max], D₂[max], D₃[max], D₄[max],
E₁[0], E₂[0], E₃[0], E₄[0],
...
```

Bila block punya panjang data berbeda, block yang lebih pendek tidak menyumbang apa pun pada posisi melebihi panjangnya — slot tersebut dilewati saja. EC block selalu memiliki panjang sama untuk (versi, EC level) tertentu, sehingga interleaving EC straightforward.

Urutan byte yang sudah ter-interleave adalah codeword stream yang akan ditempatkan ke matrix.

## Remainder bits

Untuk versi tertentu, sejumlah ekstra 0, 3, 4, atau 7 zero bit di-append ke codeword stream agar panjangnya pas dengan jumlah sel area data. Jumlah per versi ada di ISO/IEC 18004:2015 Table 1.

## Penunjuk implementasi

- `qrgen/reedsolomon.go`: `genPoly(n) []byte`, `encodeBlock(data, n) []byte`.
- Pembagian block dan interleaving diletakkan di samping driver encoder, berbagi tabel codeword dengan `qrgen/version.go`.

## Referensi

- ISO/IEC 18004:2015, §8.5–§8.6 dan Table 9 serta 13–22.
- Wicker, S. B., *Error Control Systems for Digital Communication and Storage*, Prentice Hall, 1995 — bab tentang kode RS.
- Project Nayuki, "Creating a QR Code step by step" — contoh numerik melalui encoding dan interleaving.
