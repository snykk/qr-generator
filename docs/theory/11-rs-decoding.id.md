# Decoding Reed–Solomon

Decoding Reed–Solomon membalik tahap EC dari [04-reed-solomon.id.md](04-reed-solomon.id.md), tapi toolkit algoritmiknya sama sekali berbeda. Encoding mengalikan dengan generator polynomial; decoding harus mengidentifikasi *di mana* error berada dan *apa* simbol asli, hanya dari codeword yang diterima yang sudah korup. Dokumen ini menjelaskan empat tahap decoder yang akan diimplementasikan `qrgen/rs_decode.go` (di milestone D3).

> Versi English: [11-rs-decoding.md](11-rs-decoding.md).

## Setup

Codeword yang diterima `R = (r_0, r_1, …, r_{n+k-1})` adalah jumlah codeword yang ditransmisikan `C` dan vektor error tak diketahui `E`:

```text
R(x) = C(x) + E(x)
```

`C(x)` habis dibagi generator polynomial `g(x)` karena encoder yang memaksanya begitu. `E(x)` memiliki paling banyak `t = floor(n/2)` koefisien non-nol (janji spec bahwa kita bisa mengoreksi sampai `t` error per block ketika ada `n` EC codeword). Tugas decoder adalah memulihkan `E(x)` sehingga kita bisa menghitung `C(x) = R(x) − E(x)` di atas GF(256), lalu strip ekor EC dan baca data codeword.

Decoder tidak tahu di mana error berada. Prosedur recovery punya empat tahap yang terdefinisi dengan baik.

## Tahap 1 — Syndrome

*Syndrome* adalah `R(α^i)` untuk `i = 0, 1, …, n-1`. Karena `C(α^i) = 0` untuk `i`-`i` tersebut (mereka adalah akar dari `g(x)`), tiap syndrome `S_i` sama dengan `E(α^i)`:

```text
S_i = R(α^i) = C(α^i) + E(α^i) = 0 + E(α^i) = E(α^i)
```

Jika seluruh `n` syndrome adalah nol, `E(x)` tidak punya kontribusi di akar-akar tersebut — decoder menyimpulkan codeword bebas error dan mengembalikan bagian datanya langsung. Bila tidak, kita punya `n` persamaan non-trivial dalam posisi dan magnitude error yang tidak diketahui.

Implementasi: hitung tiap `S_i` lewat evaluasi Horner. Inilah peran `polyEval` dari D2.

## Tahap 2 — Polynomial error-locator

Misalkan ada `v` error sesungguhnya pada posisi `j_1, …, j_v` dengan magnitude `Y_1, …, Y_v`. Definisikan `X_k = α^{j_k}` — "error locator" untuk posisi `j_k`. Decoder membangun polynomial yang akarnya adalah `X_k^{-1}`:

```text
Λ(x) = ∏_{k=1}^{v} (1 − X_k · x)
```

Bila kita tahu `Λ(x)`, kita tahu posisi error (terapkan Chien search; lihat tahap 3). Tantangannya: `v`, semua `X_k`, dan semua `Y_k` tidak diketahui — kita hanya punya `n` syndrome.

Tautan antara syndrome dan `Λ(x)` adalah **persamaan kunci**:

```text
S_i · Λ_0 + S_{i-1} · Λ_1 + … + S_{i-v} · Λ_v = 0   untuk i ≥ v
```

Artinya, `Λ` adalah linear recurrence terpendek yang menghasilkan urutan syndrome. Mencari recurrence terpendek seperti itu adalah persis yang dilakukan Berlekamp–Massey.

## Tahap 3 — Berlekamp–Massey

Berlekamp–Massey (BM) adalah algoritma iteratif yang melewati syndrome, menumbuhkan `Λ(x)` secara minimal saat informasi baru menuntutnya. Pseudo-code:

```text
function berlekampMassey(S [n]GF):
    Λ      = [1]              # estimasi locator saat ini (panjang L+1)
    B      = [1]              # locator "bagus" terakhir
    L      = 0                # panjang saat ini
    m      = 1                # langkah sejak L update terakhir
    b      = 1                # discrepancy saat L tumbuh terakhir

    for i = 0..n-1:
        δ = S[i] + Σ_{j=1..L} Λ[j] · S[i-j]   # discrepancy
        if δ == 0:
            m += 1
        else if 2*L <= i:
            T = Λ
            Λ = Λ − (δ/b) · x^m · B
            L = i + 1 − L
            B = T
            b = δ
            m = 1
        else:
            Λ = Λ − (δ/b) · x^m · B
            m += 1
    return Λ, L
```

Dua catatan khusus QR:

- Semua aritmatika di atas GF(256). `δ/b` pakai `gf256Inverse` dari D2.
- "Shift dengan `x^m`" berarti prepend `m` nol ke `B` sebelum dikurangkan.

Setelah BM berhenti, kita tahu `L` (jumlah error) dan `Λ(x)`. Bila `L > t = n/2`, codeword punya error lebih banyak dari yang bisa dikoreksi kode, dan decoder harus menyerah dengan `ErrTooManyErrors`.

## Tahap 4 — Chien search

Chien search menemukan akar `Λ(x)` dengan mengevaluasinya di tiap elemen non-nol GF(256):

```text
for i = 0..254:
    if polyEval(Λ, α^{-i}) == 0:
        catat posisi error i
```

Untuk QR kita hanya peduli posisi `0..n+k-1`, jadi bisa break lebih awal. Akar `α^{-i}` berkorespondensi dengan posisi error `i` (sehingga `X_k = α^{i_k} = (α^{-i_k})^{-1}`). Implementasi berada di `qrgen/rs_decode.go` bersama BM.

## Tahap 5 — Algoritma Forney

Begitu kita tahu *di mana* error berada, Forney memberi kita magnitude-nya. Definisikan polynomial evaluator error:

```text
Ω(x) = S(x) · Λ(x) mod x^n
```

di mana `S(x) = Σ_{i=0..n-1} S_i · x^i`. Lalu untuk tiap posisi error `j_k` dengan locator `X_k`:

```text
Y_k = − Ω(X_k^{-1}) / Λ'(X_k^{-1})
```

`Λ'` adalah turunan formal dari `Λ` — diimplementasikan di `polyDeriv` dari D2. Di atas GF(256) tanda "minus" hilang (tiap elemen adalah additive inverse dirinya sendiri), jadi:

```text
Y_k = Ω(X_k^{-1}) / Λ'(X_k^{-1})
```

Koreksi finalnya: `c_{j_k} = r_{j_k} XOR Y_k`.

## Menyatukan semuanya

```text
function rsDecode(received []byte, n int) []byte:
    S = syndromes(received, n)
    if S semua nol: return received[:k]              # tidak ada error
    Λ, L = berlekampMassey(S)
    if L > n/2: error "too many errors"
    positions = chienSearch(Λ, len(received))
    if len(positions) != L: error "Λ inkonsisten"    # cek algoritmik
    Ω = polyMul(S, Λ) dipotong ke degree < n
    Λ' = polyDeriv(Λ)
    for k in 0..L-1:
        X_k = α^{positions[k]}
        Y_k = polyEval(Ω, X_k^{-1}) / polyEval(Λ', X_k^{-1})
        received[positions[k]] ^= Y_k
    return received[:k]
```

`k = len(received) − n` adalah jumlah data codeword untuk block tersebut.

## Kapasitas error & mode kegagalan

| Jumlah error | Hasil |
|:------------:|:------|
| 0            | Semua syndrome nol, return cepat. |
| 1..t         | Direcover persis. |
| t+1..n−1     | BM mungkin atau mungkin tidak konvergen ke solusi konsisten; decoder melaporkan `ErrTooManyErrors` setelah cek konsistensi. |
| ≥ n          | Perilaku tidak terdefinisi — codeword punya begitu banyak error sehingga tidak ada koreksi unik. Kita mengandalkan cek konsistensi di Chien search untuk menangkap ini. |

Decoder harus selalu melakukan cek konsistensi bahwa `len(chienSearch(Λ)) == L`; kalau tidak, bisa silently mengembalikan data salah, yang lebih buruk dari gagal terang-terangan.

## Penunjuk implementasi

- `qrgen/rs_decode.go` akan menampung `syndromes`, `berlekampMassey`, `chienSearch`, `forneyMagnitudes`, dan `rsDecode`.
- `polyEval`, `polyDeriv`, `polyDivQR`, dan `gf256Inverse` datang dari D2 (`qrgen/gf256.go`).
- Tests memakai block encoded HELLO WORLD dari `docs/theory/10-worked-example.id.md` sebagai fixture: korup 1..5 byte (kapasitas untuk n=10) dan assert pemulihan persis.

## Referensi

- ISO/IEC 18004:2015, §9 — algoritma decode referensi.
- Berlekamp, E. R. — *Algebraic Coding Theory* (1968).
- Massey, J. L. — "Shift-Register Synthesis and BCH Decoding," IEEE Trans. Information Theory, IT-15(1), 1969.
- Forney, G. D. — "On Decoding BCH Codes," IEEE Trans. Information Theory, IT-11, 1965.
- Plank, J. S. — "A Tutorial on Reed–Solomon Coding for Fault-Tolerance in RAID-like Systems," 1997.
