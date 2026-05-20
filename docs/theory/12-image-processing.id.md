# Image Processing untuk Decoding QR

Tahap image di decoder mengubah `image.Image` apa pun (foto kamera, halaman scan, atau PNG yang kita generate sendiri) menjadi grid modul `[][]bool` yang bersih yang bisa dikonsumsi decoder matrix-level. Dokumen ini mencakup empat sub-tahap yang akan dipakai `qrgen/decode_image.go` (di milestone D8..D12): konversi grayscale, binarisasi Otsu, deteksi finder-pattern, perspective transform, dan sampling modul.

> Versi English: [12-image-processing.md](12-image-processing.md).

## 1. Konversi grayscale

Image sumber bisa `image.Gray`, `image.RGBA`, `image.NRGBA`, atau tipe lain. Kita collapse menjadi satu channel luminance memakai bobot standar ITU-R BT.601:

```text
Y = 0.299 R + 0.587 G + 0.114 B
```

Standard library Go `image/color.GrayModel` sudah mengimplementasikan konversi ini; kita tinggal menarik byte `Y` untuk tiap pixel ke dalam satu buffer `[]uint8` panjang `width * height`. Bekerja dalam buffer flat row-major membuat pass binarisasi dan scanning lebih cepat daripada lewat interface `image.Image`.

## 2. Binarisasi Otsu

Setelah punya grayscale, kita butuh satu threshold `t` agar pixel dengan `Y ≥ t` adalah background dan `Y < t` adalah foreground. **Metode Otsu** memilih `t` yang memaksimalkan *between-class variance*, yaitu threshold yang paling baik memisahkan histogram menjadi dua cluster.

Algoritma:

```text
function otsuThreshold(pixels []uint8) uint8:
    hist[256] = histogram pixels
    total = len(pixels)
    sumAll = Σ_{i=0..255} i * hist[i]

    sumBg = 0
    wBg   = 0
    best  = 0
    bestT = 0
    for t = 0..255:
        wBg += hist[t]
        if wBg == 0: continue
        wFg = total - wBg
        if wFg == 0: break
        sumBg += t * hist[t]
        meanBg = sumBg / wBg
        meanFg = (sumAll - sumBg) / wFg
        between = wBg * wFg * (meanBg - meanFg)^2
        if between > best:
            best, bestT = between, t
    return bestT
```

Otsu bersifat global — satu threshold untuk seluruh image. Bekerja sempurna untuk output encoder kita sendiri (PNG monokrom) dan untuk sebagian besar foto dengan pencahayaan baik. Untuk foto dengan gradien kuat atau shadowing tidak merata, fallback ke skema thresholding lokal (Sauvola, block-mean, dst.) adalah kandidat enhancement post-v0.2.

## 3. Deteksi finder-pattern

Finder pattern QR di ruang pixel terlihat sebagai urutan run gelap/terang/gelap/terang/gelap dengan **rasio 1:1:3:1:1**. Kita scan tiap baris dari kiri ke kanan, melacak panjang lima run konsekutif terakhir berwarna konstan. Kapanpun lima run cocok dengan rasio tersebut (dalam toleransi) dan pixel pusat berwarna gelap, kolom pusat dicatat sebagai kandidat.

```text
function scanRowFinders(row []bool, y int) []candidate:
    runs = [0]*5
    color = row[0]
    candidates = []
    for x = 1..len(row)-1:
        if row[x] == color:
            runs[4] += 1
        else:
            shift runs ke kiri 1; runs[4] = 1; color = row[x]
            if warna pusat gelap dan runs ≈ k:k:3k:k:k (toleransi ±50%):
                centerX = x − runs[4] − runs[3] − runs[2]/2
                append candidate(centerX, y, k)
    return candidates
```

Setelah row scan kita lakukan **konfirmasi vertikal** di tiap kolom pusat kandidat: walk ke atas dan ke bawah dari `(centerX, y)`, mengecek bahwa run length vertikal melalui kolom tersebut juga mematuhi rasio 1:1:3:1:1. Kandidat yang gagal cek vertikal dibuang.

Kandidat yang lolos di-cluster (satu finder biasanya menghasilkan beberapa centre kandidat di baris-baris konsekutif). Tiga centre cluster dengan bukti terbanyak menjadi finder kiri-atas, kanan-atas, dan kiri-bawah, terurut berdasar geometri: vertex siku-siku yang paling dekat ke finder kiri-atas simbol.

Bila kurang dari tiga finder valid yang lolos, decoder mengembalikan `ErrFinderNotFound`.

## 4. Validasi geometri

Sebelum mempercayai tiga centre finder kita sanity-check:

- Segitiga yang mereka bentuk harus mendekati siku-siku.
- Dua kaki (kiri-atas → kanan-atas dan kiri-atas → kiri-bawah) harus punya panjang yang mirip (simbol berbentuk persegi).
- Ukuran modul estimasi dari tiap finder (`runs[2] / 3` dari fit 1:1:3:1:1) harus setuju antar ketiga finder dalam toleransi.

Validasi gagal juga mengembalikan `ErrFinderNotFound`. Ini yang menangkap mode kegagalan umum: background ramai yang secara tidak sengaja mengandung pola 1:1:3:1:1.

## 5. Perspective transform

Image yang kita terima bisa dirotasi, diskew secara perspektif, atau di-scale non-uniform. Kita butuh fungsi `srcPixel(col, row)` yang menerima koordinat grid modul `(col, row)` dan mengembalikan pixel image sumber `(u, v)` tempat centre modul itu berada.

Diberikan empat korespondensi titik `(col_i, row_i) ↔ (u_i, v_i)` kita bisa menghitung **matriks homography 3×3** `H` sehingga:

```text
[u_i]       [col_i]
[v_i] = H * [row_i]
[ 1 ]       [  1  ]
```

(Dalam koordinat homogen: `[u, v, w] = H · [col, row, 1]`, lalu bagi dengan `w`.)

Empat korespondensi memberi 8 persamaan; `H` punya 9 entri tapi scale-invariant sehingga 8 unknown — solvable lewat sistem linear kecil.

Empat korespondensi untuk QR adalah:

- **Centre finder kiri-atas** → `(3, 3)` (centre dari finder pattern 7×7 menempati modul 0..6, di-centre pada modul 3).
- **Centre finder kanan-atas** → `(n − 4, 3)` di mana `n = 21 + 4·(v − 1)`.
- **Centre finder kiri-bawah** → `(3, n − 4)`.
- **Centre alignment-pattern kanan-bawah** → untuk V2+ ini datang dari menemukan alignment pattern di image sumber (D11); untuk V1 kita ekstrapolasi dari tiga yang lain (parallelogram adalah persegi panjang).

Versi `v` diestimasikan dari spasi finder dulu; lihat "Estimasi pitch modul" di bawah.

## 6. Estimasi pitch modul & versi

Dari centre finder kiri-atas dan kanan-atas kita tahu extent horizontal simbol dalam pixel. Dibagi dengan `(n − 7)` (jumlah modul antara dua centre finder) memberi kita pitch modul dalam pixel. Karena `n` bergantung `v` dan kita belum tahu `v`, trik standarnya:

1. Estimasikan ukuran modul dari fit 1:1:3:1:1 di tiap finder (run "3" tepat 3 modul lebarnya; bagi 3 dapat pitch).
2. Hitung `v = round( (jarak / pitch − 7) / 4 + 1 )`.
3. Validasi bahwa `n` yang dihasilkan ada di `[21, 177]` dan konsisten dengan finder horizontal dan vertikal.

Untuk V7+, codeword BCH version-information di kemudian pipeline (D6 di dalam decoder matrix) bisa double-check estimasi ini.

## 7. Refinement alignment-pattern

Untuk V2 ke atas, spec QR mendefinisikan satu atau lebih alignment pattern. Setelah menghitung homography awal dari tiga finder, kita refine dengan mencari alignment pattern kanan-bawah di image sumber (yang terdekat ke sudut `(col, row) = (n-7, n-7)`). Mengrefine sudut kanan-bawah saja biasanya cukup; simbol dengan distorsi lokal parah bisa mengrefine tiap alignment pattern secara individu dengan ongkos lebih banyak langkah solve.

V1 tidak punya alignment pattern dan melewati langkah ini.

## 8. Sampling modul

Dengan `H` di tangan, sampling jadi straightforward:

```text
function sample(col, row) -> bool:
    (u, v) = H · (col + 0.5, row + 0.5, 1)   # centre dari modul
    return grayscale[v * width + u] < threshold
```

`+ 0.5` menggeser ke centre modul dalam koordinat grid. Untuk robustness kita bisa merata-ratakan patch 3×3 di sekitar `(u, v)` dan threshold meannya, tapi sample single-pixel adalah pendekatan textbook.

Hasilnya `[][]bool` dengan sisi `n` × `n` — persis yang dikonsumsi decoder matrix.

## Penunjuk implementasi

- `qrgen/decode_image.go` menampung struct bitmap, konversi grayscale, Otsu, scanning finder, validasi geometri, homography, dan sampling.
- Kerja numerik pakai `float64` di seluruh; kita promote koordinat pixel `uint8` dan `int` hanya di boundary.
- Untuk testing, generate PNG dengan encoder kita sendiri lalu jalankan kembali melalui decoder — matrix yang keluar harus cocok persis dengan `[][]bool` asli. Varian sintetis dirotasi/di-scale menguji path homography.

## Referensi

- ISO/IEC 18004:2015, §11 — algoritma decode referensi.
- Otsu, N. — "A Threshold Selection Method from Gray-Level Histograms," IEEE Trans. Systems, Man, and Cybernetics, 1979.
- Hartley & Zisserman — *Multiple View Geometry in Computer Vision*, edisi 2, §4 (estimasi homography).
- ZXing — *referensi decoder open-source*: <https://github.com/zxing/zxing>, khususnya kelas `FinderPatternFinder` dan `PerspectiveTransform`.
