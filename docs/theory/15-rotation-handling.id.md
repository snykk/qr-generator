# Rotation Handling di Decoder

Decoder v0.3 mengasumsikan image sumber kira-kira right-side-up: finder top-left duduk di top-left image, top-right ada di sebelah kanannya, dan bottom-left ada di bawahnya. Asumsi itu patah pada saat user memegang HP-nya menyamping ketika men-scan halaman cetak atau ketika sebuah QR ditempel pada label yang lalu di-frame terbalik oleh kamera. Dokumen ini menjelaskan tepatnya di mana asumsi tegak ter-encode di pipeline v0.3, kenapa setiap tahap lain dari image pipeline sebenarnya sudah rotation-invariant, dan bagaimana sebuah test handedness cross-product tunggal membuka rotation handling untuk kasus axis-aligned (90 / 180 / 270 derajat) plus soft tilt hingga sekitar 30 derajat.

> Versi Inggris: [15-rotation-handling.md](15-rotation-handling.md).

## 1. Di mana asumsi tegak itu tinggal

Image pipeline v0.3 punya lima tahap: konversi grayscale, binarisasi (Otsu dengan fallback Sauvola mulai v0.3), scan finder-pattern, finder ordering, dan module sampling yang digerakkan homography. Menelusurinya satu per satu:

- **Grayscale dan binarisasi** hanya bergantung pada intensitas per pixel. Rotasi tidak berpengaruh.
- **Scan finder-pattern** memakai scan baris-dan-kolom `1:1:3:1:1` di `scanRowForFinders` dan `crossCheckVertical`. Finder pattern QR berbentuk kotak konsentris (`XXXXXXX / X.....X / X.XXX.X / X.XXX.X / X.XXX.X / X.....X / XXXXXXX`), sehingga garis lurus mana pun yang melewati pusatnya akan melewati dark-light-dark-light-dark dengan rasio `1:1:3:1:1` yang sama tanpa peduli orientasinya. Scanner menangkap finder yang ter-rotasi pada sudut apa pun yang berada dalam toleransi `±50%` per modul milik `fitsFinderRatio`.
- **Finder ordering** tinggal di `orderFinderTriple`. Di sinilah asumsi tegak bersembunyi. Fungsi tersebut mengidentifikasi vertex sudut-siku-siku berdasarkan "finder yang paling jauh dari sisi terpanjang" (rotation-invariant), lalu mendisambiguasi dua sisanya menjadi top-right vs bottom-left dengan `if tr.y > bl.y { swap }` — shortcut tegak. Untuk image yang ter-rotasi, label-nya keluar tertukar, yang membuat tahap-tahap hilir membaca matrix dengan salah.
- **Homography** di `homographyFromFinders` membangun transform projektif 3x3 umum dari empat korespondensi anchor-point. Transform projektif sudah menyerap rotasi, skala, translasi, dan perspektif. Diberikan label yang benar dari `orderFinderTriple`, homography mereproduksi rotasi secara otomatis — tanpa perubahan kode.
- **Module sampling** mengevaluasi homography di tiap pusat grid dan membaca pixel yang sudah dibinarisasi. Ini juga rotation-agnostic.

Jadi seluruh fix tinggal di `orderFinderTriple`. Semuanya sudah bekerja.

## 2. Vertex sudut-siku-siku sudah rotation-invariant

Simbol QR menempatkan tiga finder pattern-nya di tiga dari empat sudut sebuah persegi: top-left, top-right, dan bottom-left. Hubungkan ketiga pusatnya dan hasilnya adalah segitiga siku-siku yang sudut siku-sikunya duduk di sudut top-left. Hipotenusa segitiga itu adalah segmen antara finder top-right dan bottom-left.

```text
                  (TR)
                  /|
                 / |
                /  |
            sqrt2 a|
              /    |a
             /     |
            /      |
           /-------|
        (TL)   a   (BL)
```

di mana `a` adalah panjang sisi simbol dalam pixel. Hipotenusa-nya `a * sqrt(2)` dan selalu lebih panjang dari kedua kakinya.

Identifikasi vertex sudut-siku-siku karena itu mengerucut ke satu aturan: dari tiga jarak pasangan, yang terpanjang menghubungkan dua finder yang BUKAN top-left; finder yang tersisa adalah top-left. Aturan ini bertahan terhadap rigid motion simbol apa pun karena rigid motion mempertahankan jarak. Rotasi 90, 180, 270 — identitas yang sama. Soft tilt — panjang kaki sedikit meregang, tapi pasangan terpanjang tetap hipotenusa. `orderFinderTriple` yang ada sudah melakukan ini dan kita pertahankan tanpa perubahan.

## 3. Bagian yang sulit: membedakan top-right dari bottom-left

Setelah kita tahu finder mana yang top-left, dua finder tersisa. Masing-masing bisa jadi top-right atau bottom-left. Kita butuh aturan rotation-invariant yang memilih yang benar.

Triknya adalah **cross product**. Perlakukan dua vektor dari `TL` ke masing-masing finder sisa sebagai vektor 2D dan hitung cross product-nya. Di image yang tegak:

```text
v_tr = TR - TL    (menunjuk ke kanan)
v_bl = BL - TL    (menunjuk ke bawah)

cross = v_tr.x * v_bl.y - v_tr.y * v_bl.x
```

Untuk QR yang tegak di koordinat image (di mana `y` tumbuh ke bawah):

```text
v_tr  = (a, 0)
v_bl  = (0, a)
cross = a * a - 0 * 0 = a²    (positif)
```

Simbol QR yang nyata (tidak mirror) selalu memiliki handedness yang sama tanpa peduli rotasinya, sehingga sign dari `cross((TR - TL), (BL - TL))` sama di setiap sudut. Untuk meng-assign label dengan benar, kita secara tentative memilih salah satu dari dua finder sisa sebagai `TR`, menghitung cross product, dan menukar `TR` dan `BL` jika sign-nya keluar negatif.

## 4. Analisis sign yang dikerjakan pada 0 / 90 / 180 / 270 derajat

Untuk meyakinkan diri kita bahwa cross product tetap positif di setiap rotasi axis-aligned, kita bahas tiap kasus. Kita tempatkan TL simbol di sudut yang ditunjukkan rotasi, dengan panjang sisi `a` dalam pixel image:

| Rotasi | TL    | TR    | BL    | `TR - TL` | `BL - TL` | Cross product |
| --- | --- | --- | --- | --- | --- | --- |
| 0 (tegak) | `(10, 10)` | `(10 + a, 10)` | `(10, 10 + a)` | `(a, 0)` | `(0, a)` | `a²` |
| 90 (CW) | `(10 + a, 10)` | `(10 + a, 10 + a)` | `(10, 10)` | `(0, a)` | `(-a, 0)` | `a²` |
| 180 | `(10 + a, 10 + a)` | `(10, 10 + a)` | `(10 + a, 10)` | `(-a, 0)` | `(0, -a)` | `a²` |
| 270 (CW) | `(10, 10 + a)` | `(10, 10)` | `(10 + a, 10 + a)` | `(0, -a)` | `(a, 0)` | `a²` |

Tiap kasus memberi `+a²`. Soft tilt yang menginterpolasi antara dua rotasi tetangga mempertahankan sign karena cross product adalah fungsi kontinu dari posisi finder dan tidak pernah melewati nol untuk segitiga non-degenerate (segitiga yang vertex-nya kolinear akan memberi cross product nol, tapi validasi sudut-siku-siku plus rasio kaki yang sudah ada sudah menyingkirkan kasus itu).

## 5. Simbol mirror

Simbol QR mirror — yang pattern ink-nya direfleksikan — akan memberi sign cross-product yang berlawanan. QR code nyata tidak pernah mirror: encoder menghasilkan handedness yang tetap, dan simbol yang tercetak atau ditampilkan di layar mempertahankan handedness itu. Jadi kita tidak punya kepentingan untuk menolak handedness mirror; kita hanya mengamati bahwa input nyata selalu punya satu sign dan algoritma kita bersandar pada fakta itu untuk meng-assign label.

Jika gambar yang mirror entah bagaimana di-feed ke decoder (misal test image sintetis yang matrix-nya di-flip horizontal), check cross-product tetap akan menghasilkan label yang "konsisten" yang homography akan dengan senang hati decode, tapi matrix yang dihasilkan akan gagal di hilir — format-info BCH tidak akan tervalidasi, atau Reed-Solomon akan melewati budget koreksinya. Kegagalannya kentara dan permukaan API publik tetap bersih; kita menerima trade-off ini alih-alih menambah sentinel `ErrMirroredSymbol` khusus yang tidak akan pernah disentuh caller dunia nyata.

## 6. Kenapa homography menangani sisanya

Setelah `orderFinderTriple` mengembalikan triple `(TL, TR, BL)` yang benar, `homographyFromFinders` membangun transform projektif 3x3 `H` sehingga:

```text
H * (3, 3, 1)     = TL (di koordinat pixel)
H * (n - 4, 3, 1) = TR
H * (3, n - 4, 1) = BL
H * (n - 4, n - 4, 1) = BR (di-ekstrapolasi via parallelogram completion)
```

di mana `n` adalah panjang sisi dalam modul dan koordinat modul `(3, 3)`, `(n-4, 3)`, `(3, n-4)`, `(n-4, n-4)` adalah pusat dari empat region finder (dengan slot bottom-right ditempati oleh alignment pattern atau estimasi parallelogram-completion-nya).

Homography 3x3 adalah transform projektif invertible paling umum di 2D dan tertutup pada komposisi dengan rotasi dan translasi. Konkret-nya, jika simbol dirotasi `theta` dan ditranslasikan, homography yang kita pecahkan menyerap keduanya ke dalam satu matrix — kita tidak pernah perlu menerapkan langkah "derotation" sebelum sampling. Module sampling lalu menelusuri grid modul demi modul dan homography memetakan tiap pusat modul ke pixel sumber yang benar tanpa peduli orientasi.

Secara matematis: homography `H` apa pun dapat di-dekomposisi sebagai `H = T * R * K * P`, di mana `T` adalah translasi, `R` rotasi, `K` skala/shear, dan `P` bagian projektif. Linear solve empat-titik kita di `solveLinear8` memulihkan `H` tanpa perlu memisahkan faktor-faktornya.

## 7. Boundary cakupan di 30 derajat

Kenapa v0.4 berhenti pada soft tilt sekitar 30 derajat alih-alih sampai 90? Alasannya tinggal di tahap 3 pipeline v0.3 — scanner baris `1:1:3:1:1`. Scan horizontal melalui finder yang ter-rotasi sebesar sudut `theta` menabrak proyeksi "horizontal" tiap modul. Untuk rotasi axis-aligned proyeksinya sama persis dengan ukuran modul. Untuk tilt `theta`, panjang run di dalam finder menjadi `cos(theta) * modul + tan(theta) * jitter`, dan rasionya mengalir menjauhi `1:1:3:1:1` ideal seiring `theta` membesar.

`fitsFinderRatio` menerima tiap run dalam `±50%` dari ukuran yang diharapkan. Memasukkan angkanya: pada `theta = 30°`, drift-nya kira-kira `1 / cos(30°) - 1 ≈ 15%`. Pada `theta = 45°`, drift melompat ke kira-kira `1 / cos(45°) - 1 ≈ 41%` dan mulai mendekati tepi band toleransi tanpa headroom untuk jitter tambahan yang diperkenalkan oleh interpolasi bilinear di PNG yang dirotasi. Pada `theta = 60°`, row scan secara efektif berjalan di diagonal finder pattern dan rasionya patah seluruhnya.

Jadi v0.4 merilis rotasi axis-aligned + soft tilt hingga sekitar 30 derajat dan secara eksplisit meninggalkan band `[30°, 90°)` sebagai pekerjaan masa depan. Mengangkat batas itu membutuhkan baik toleransi yang lebih lebar di scanner (berisiko — false positive melonjak di latar berisik) atau finder detector yang berbeda sama sekali (contour tracing, pencarian kipas orientasi). Keduanya lebih besar dari rilis minor.

## 8. Pointer Implementasi

- `qrgen/decode_image.go` menampung `orderFinderTriple`. Perubahan di v0.4 mengganti blok terakhir `if tr.y > bl.y { swap }` dengan check handedness cross-product; semua di atasnya (deteksi vertex sudut-siku-siku, sanity rasio kaki, check hipotenusa) tidak berubah karena memang sudah rotation-invariant.
- Cross product hanyalah satu multiply-subtract-compare, jadi perubahan v0.4 allocation-neutral dan tidak menambah biaya yang terdeteksi ke Otsu fast path.
- `homographyFromFinders` di file yang sama tidak disentuh di v0.4. Memverifikasi ini secara eksperimental adalah bagian dari benchmark sweep R5.
- `qrgen/decode_rotation_test.go` (baru di v0.4) menampung helper `rotateImage` yang membangun set fixture sintetis dan assertion round-trip-nya. Fixture-nya deterministic dan tetap in-memory; tidak ada blob PNG yang mendarat di `testdata/`.

## Referensi

- Proyek ZXing — *open-source decoder reference*: <https://github.com/zxing/zxing>. Method `FinderPatternFinder.orderBestPatterns` mereka memakai trik handedness cross-product yang sama yang kita adopsi di sini; doc ini sebagian merupakan transkripsi dari algoritma itu ke dalam konvensi dan notasi qrgen.
- Hartley, R., Zisserman, A. — *Multiple View Geometry in Computer Vision*, ed. 2, Cambridge University Press, 2004, §4. Fondasi untuk klaim bahwa homography 3x3 menyerap rotasi diberikan korespondensi corner yang benar.
- ISO/IEC 18004:2015 — §11.2 (locator pattern detection) dan §11.3 (image sampling). Spec mengasumsikan "simbol kira-kira right-side-up" tanpa elaborasi; dokumen ini mengisi geometri yang membuat kita dapat menghilangkan asumsi itu untuk kasus axis-aligned.
- Project Nayuki — *QR Code generator library, decoder companion notes*: <https://www.nayuki.io/page/qr-code-generator-library>. Cross-check yang berguna untuk geometri finder pada simbol yang dirotasi.
