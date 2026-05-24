# Adaptive Thresholding untuk Decoder

Decoder v0.2 hanya memakai **metode Otsu** sebagai binariser, memilih satu threshold global untuk seluruh image. Itu bekerja sempurna untuk output encoder kita sendiri dan untuk foto dengan pencahayaan rata, tapi runtuh pada capture dunia nyata yang punya gradient, soft shadow, vignette, atau kontras keseluruhan yang rendah. Dokumen ini menjelaskan kenapa Otsu gagal pada input-input tersebut, apa yang dikerjakan local adaptive thresholding sebagai gantinya, kenapa kita memilih **algoritma Sauvola** di atas Niblack / Bernsen / Adaptive Gaussian sebagai fallback di v0.3, bagaimana integral image menjaga Sauvola tetap linear terhadap jumlah pixel, dan bagaimana decoder memutuskan secara runtime binariser mana yang dijalankan.

> Versi Inggris: [14-adaptive-thresholding.md](14-adaptive-thresholding.md).

## 1. Kenapa Otsu gagal pada pencahayaan tidak rata

Metode Otsu memaksimumkan *between-class variance* dari histogram grayscale. Konkretnya ia memilih threshold `t` yang memaksimumkan:

```text
σ²_B(t) = w_0(t) * w_1(t) * (μ_0(t) - μ_1(t))²
```

di mana `w_0, w_1` adalah bobot kumulatif kedua class dan `μ_0, μ_1` adalah mean masing-masing. Pemilihan ini memiliki satu asumsi krusial: histogram-nya kira-kira **bimodal**, dengan lembah bersih di antara puncak dark ink dan puncak bright paper. Tiga kondisi dunia nyata mematahkan asumsi tersebut.

- **Linear gradient** (satu sisi image terang, satu sisi gelap) mencampur dua puncak menjadi sebuah kontinum. Histogram menjadi lebar dan datar. Otsu tetap mengembalikan threshold, tapi pixel di sisi gelap yang *seharusnya* paper berakhir di bawah threshold, dan pixel di sisi terang yang *seharusnya* ink berakhir di atas threshold.
- **Vignette dan drop shadow** menciptakan offset frekuensi rendah ke seluruh image. Otsu melihat histogram yang puncak-puncaknya tersebar dan memilih threshold yang memuaskan rata-rata global namun mengorbankan satu sudut simbol.
- **Kontras keseluruhan yang rendah** (print pudar, pencahayaan redup, scan tercuci) hanya menyisakan satu puncak. Between-class variance Otsu tidak punya maksimum yang jelas dan `t` yang dipilih jatuh hampir sembarangan, sering menghasilkan binarisasi di mana salah satu class kosong.

Pada ketiga kasus tersebut, gejalanya adalah image yang sudah dibinarisasi punya finder pattern yang rusak di setidaknya satu sudut, dan `findFinders` mengembalikan kurang dari tiga kandidat atau mengembalikan kandidat yang geometrinya invalid. Itulah failure mode yang ditargetkan oleh milestone ini.

## 2. Ide Niblack: lokalkan threshold-nya

Solusinya adalah berhenti memilih satu threshold untuk seluruh image. **Niblack (1986)** mengusulkan menghitung threshold terpisah untuk tiap pixel berdasarkan statistik dari window kecil yang berpusat pada pixel tersebut:

```text
T_Niblack(x, y) = m(x, y) + k * s(x, y)
```

di mana `m(x, y)` adalah mean lokal dan `s(x, y)` adalah standard deviation lokal intensitas pixel di dalam window `w x w` di sekitar `(x, y)`, dan `k` adalah konstanta yang dapat di-tune (biasanya `k = -0.2` untuk teks gelap di latar terang). Threshold mengikuti pencahayaan: di region gelap mean-nya rendah, jadi threshold-nya rendah; di region terang mean-nya tinggi, jadi threshold-nya tinggi. Gradient tidak lagi jadi masalah karena threshold ikut mengalir bersama gradient.

Niblack bekerja untuk dokumen tapi punya dua kelemahan yang dikenal luas. Di region uniform (tanpa ink, hanya paper atau hanya shadow) standard deviation-nya kecil dan threshold runtuh ke mean, lalu mengklasifikasi noise acak sebagai ink. Dan parameter `k` rapuh: nilai yang tepat untuk satu dokumen gagal pada dokumen berikutnya.

## 3. Penyempurnaan Sauvola

**Sauvola dan Pietikainen (2000)** memperkenalkan normalisasi yang menekan masalah noise di region uniform:

```text
T_Sauvola(x, y) = m(x, y) * (1 + k * (s(x, y) / R - 1))
```

Perubahan kunci adalah term `(s / R - 1)`, di mana `R` adalah range normalisasi — untuk image 8-bit pilihan standar adalah `R = 128`, separuh dynamic range. Di dalam term ini:

- Di window dengan **variance tinggi** (tepi teks nyata), `s` mendekati `R`, kurung menjadi `1 + k * 0 = 1`, dan threshold duduk dekat mean lokal. Ink dan paper terpisah dengan bersih.
- Di window dengan **variance rendah** (paper uniform, shadow uniform), `s` jauh di bawah `R`, kurung menjadi negatif, dan threshold turun jauh di bawah mean lokal. Noise acak tidak lagi melewati threshold dan region tetap satu class.

Pasangan parameter klasik adalah `k = 0.2` (paper) atau `k ≈ 0.2..0.5`, dengan `w = 25` untuk dokumen yang dicetak pada DPI khas. Kita mengadopsi default textbook dan memperlakukannya sebagai konstanta internal di v0.3; jika fixture dunia nyata menuntut tuning, public option dapat menyusul kemudian.

## 4. Integral image: menjaga Sauvola O(width * height)

Perhitungan naif untuk mean dan standard deviation lokal memakan biaya `O(w²)` per pixel, sehingga image penuh memakan `O(width * height * w²)` — untuk `w = 25` itu 625 read ekstra per pixel. **Integral image** (Shafait, Keysers, Breuel 2008) memangkasnya menjadi `O(1)` per pixel terlepas dari ukuran window.

Integral image `S` adalah running sum nilai pixel di sepanjang kedua axis:

```text
S[y][x] = Σ_{i ≤ y, j ≤ x} pixel(i, j)
```

Ia dapat diisi dalam satu pass linear via:

```text
S[y][x] = pixel(y, x) + S[y-1][x] + S[y][x-1] - S[y-1][x-1]
```

Diberikan `S`, jumlah pixel di dalam rectangle apa pun `[(x0, y0), (x1, y1)]` menjadi query corner-arithmetic constant-time:

```text
sum_rect = S[y1][x1] - S[y0-1][x1] - S[y1][x0-1] + S[y0-1][x0-1]
```

Dibagi dengan luas rectangle memberi mean lokal.

Untuk Sauvola kita juga butuh **variance lokal**, yang membutuhkan running sum dari pixel kuadrat. Kita membangun integral image kedua `S²` dengan `S²[y][x] = Σ pixel²` dan mengembalikan variance via identitas standar:

```text
mean   = sum_rect  / area
mean²  = sum²_rect / area
var    = mean² - mean²
std    = sqrt(var)
```

Kedua integral image diisi dalam satu pass linear ke atas buffer grayscale, dan tiap threshold Sauvola kemudian memakan empat load dari tiap integral image — delapan load plus segelintir multiplication dan satu `sqrt`. Total biaya binarisasi Sauvola adalah `O(width * height)`, kompleksitas asimtotik yang sama dengan Otsu.

Satu caveat praktis: image 4096x4096 dengan nilai 8-bit menjumlahkan sekitar `4.3 * 10^9` untuk `S` dan `1.1 * 10^12` untuk `S²`. Keduanya nyaman muat di `uint64`, meledak di `uint32`, jadi kita pakai `uint64` di seluruh jalur.

## 5. Kenapa Sauvola di atas alternatifnya

| Algoritma | Ide | Kelebihan | Kekurangan |
| --- | --- | --- | --- |
| Otsu | Satu threshold global yang memaksimumkan between-class variance | Cepat, parameter-free, sempurna di histogram bimodal yang bersih | Gagal pada gradient, shadow, kontras rendah |
| Niblack | Mean lokal + `k * std` | Mengikuti pencahayaan | Noise di region uniform; `k` rapuh |
| **Sauvola** | Niblack dengan normalisasi `(s/R - 1)` | Mengikuti pencahayaan DAN menekan noise di region uniform; standar untuk binarisasi dokumen | Sedikit lebih lambat dari Niblack; butuh integral image agar cepat |
| Wolf | Sauvola dengan normalisasi global min/max | Sedikit lebih baik pada kontras sangat rendah | Kehilangan keunggulan lokalitas Sauvola; pass global tambahan |
| Bernsen | Pakai midrange lokal antara min dan max, bukan mean+std | Mengikuti kontras lokal secara langsung | Sangat berisik di region uniform; sensitif terhadap outlier |
| Adaptive Gaussian (OpenCV) | Threshold = weighted Gaussian mean dari window dikurangi konstanta `C` | Halus; tersedia di library image | Setara dengan varian Niblack di bawah kapnya; `C` dan window punya kerapuhan yang sama |

Sauvola adalah pilihan standar untuk binarisasi dokumen justru karena mewarisi lokalitas Niblack dan memperbaiki noise region uniform milik Niblack. Decoder qr-generator membutuhkan itu persis: simbol QR adalah dokumen yang kebetulan berbentuk persegi, dan failure mode yang ingin kita perbaiki (gradient, shadow, kontras rendah) adalah teritori binarisasi dokumen.

## 6. Dispatch runtime: kapan masing-masing kita pakai?

Sekalipun Sauvola tersedia, kita tidak ingin menanggung biaya integral-image-nya di setiap decode. PNG sintetis dan foto pencahayaan rata adalah hal yang umum, dan Otsu terbukti lebih cepat pada keduanya. Decoder karena itu memakai **gate dua-stage** untuk memutuskan binariser mana yang dijalankan.

**Stage 1 — gate bimodality proaktif.** Otsu sudah menghitung between-class variance maksimum `σ²_B` dan total variance `σ²_T` dari histogram. Rasionya:

```text
η = σ²_B / σ²_T   ∈ [0, 1]
```

adalah *ukuran separability* standar yang didefinisikan paper asli Otsu. `η` dekat 1 menandakan histogram bimodal yang baik dan threshold Otsu bermakna; `η` dekat 0 menandakan histogram pada dasarnya unimodal dan Otsu tidak dapat melakukan tugasnya. Kita memilih `η_min = 0.5` sebagai cutoff default — ketika `η < η_min`, kita tahu binarisasi Otsu akan tidak sehat *sebelum* kita menjalankannya, jadi kita lewati langkah binarisasi Otsu sepenuhnya dan langsung ke Sauvola. Ini hanya memakan satu histogram pass (yang Otsu lakukan toh untuk menghitung threshold) dan menghemat satu pass finder detection yang akan terbuang pada output Otsu yang tidak sehat.

**Stage 2 — post-check reaktif.** Kalau `η ≥ η_min` kita jalankan binarisasi Otsu dan finder detection seperti biasa. Kalau `findFinders` berhasil, kita percayai hasilnya dan lanjutkan. Selain itu, kita me-rebinarise image grayscale dengan Sauvola lalu menjalankan ulang finder detection; baru ketika pass Sauvola juga gagal decoder mengembalikan `ErrFinderNotFound`. Plan asli v0.3 juga mempertimbangkan post-check rasio foreground (menolak binarisasi yang rasionya di bawah 5% atau di atas 95%), tapi benchmark T5 mengkonfirmasi observasi empiris yang memensiunkan check itu: binarisasi single-class yang degenerate tidak akan pernah menghasilkan triple 1:1:3:1:1 yang valid, jadi kegagalan `findFinders` sudah merupakan strict superset dari "rasio tidak sehat", dan melewati pass tambahan `O(width * height)` di happy path menjaga Otsu fast path tetap dalam ~1% dari baseline v0.2.

Kombinasi ini menjaga kasus umum (PNG bersih, foto pencahayaan rata) di Otsu fast path, membiarkan histogram yang jelas-jelas buruk langsung melompat ke Sauvola, dan menjaga Sauvola sebagai safety net untuk kasus di mana histogram terlihat baik tapi struktur spasial tetap mengalahkan Otsu (misal satu sudut tertutup shadow tapi histogram global masih bimodal).

Tiga state runtime tersebut terlihat oleh test via hook `binariserUsed` yang tidak di-export: `binariserOtsu`, `binariserSauvolaProactive` (Stage 1 menyala), dan `binariserSauvolaReactive` (Stage 2 menyala). Hook ini tidak pernah muncul di API publik.

## 7. Pointer Implementasi

- `qrgen/decode_image.go` menampung `otsuThreshold` dan `binarise` yang sudah ada. Fungsi Otsu mendapat return value tambahan untuk `η` (ukuran separability) tanpa mengubah output threshold-nya, sehingga caller dapat membaca keduanya dengan satu panggilan.
- `qrgen/decode_image_sauvola.go` (baru di v0.3) menampung `sauvolaBinarise`, kedua integral image, dan helper `windowMeanStd`. Fungsi tersebut mengembalikan `bitmap` dengan konvensi `p <= t` yang sama dengan Otsu sehingga finder detection di hilir tidak melihat perbedaan.
- Logika dispatch berupa blok `if η < ηMin { ... } else { ... }` kecil di dalam `decodeImage`, plus fallback reaktif `else if findFinders gagal { ... }`. Kita sengaja menjaganya sebagai kode lurus alih-alih strategy interface — hanya ada dua algoritma dan YAGNI berlaku.
- Default berupa konstanta tidak di-export di `decode_image_sauvola.go`: `sauvolaWindow = 25`, `sauvolaK = 0.2`, `sauvolaR = 128.0`, `etaMin = 0.5`. Theory doc menjelaskan angkanya; konstanta-konstanta tersebut mendokumentasikan tempatnya tinggal.

## Referensi

- Otsu, N. — "A threshold selection method from gray-level histograms," *IEEE Trans. Systems, Man, and Cybernetics*, 9(1):62–66, 1979. Mendefinisikan ukuran separability `η` yang kita pakai ulang di gate proaktif.
- Niblack, W. — *An Introduction to Digital Image Processing*, Prentice-Hall, 1986. Pendahulu Sauvola.
- Sauvola, J., Pietikainen, M. — "Adaptive document image binarization," *Pattern Recognition*, 33(2):225–236, 2000. Formula beserta default `k = 0.2`, `R = 128`.
- Shafait, F., Keysers, D., Breuel, T. M. — "Efficient implementation of local adaptive thresholding techniques using integral images," *Document Recognition and Retrieval XV*, SPIE, 2008. Trik integral image yang menjaga Sauvola tetap linear.
- Wolf, C., Jolion, J.-M. — "Extraction and recognition of artificial text in multimedia documents," *Pattern Analysis and Applications*, 6(4):309–326, 2003. Varian Wolf di atas Sauvola.
- Viola, P., Jones, M. — "Rapid object detection using a boosted cascade of simple features," *CVPR*, 2001. Mempopulerkan integral image di computer vision (Section 2).
