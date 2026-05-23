# QR Decoder — Plan Adaptive Thresholding

Dokumen ini menjelaskan rencana implementasi enhancement **adaptive thresholding** yang menargetkan rilis minor `v0.3.0`. Ini adalah cabang robustness pertama yang dipecah dari Roadmap v0.2.0 (cabang kedua, **rotation handling**, dikelola di plan dan branch terpisah lalu rilis sebagai `v0.4.0`).

> Status: **draft / dokumen hidup.** Milestone T1..T6 dikerjakan bertahap di branch `decoder-thresholding`; tiap milestone berupa commit fokus (atau seri commit kecil) yang sudah lengkap dengan tes, mengikuti ritme M1..M11 dan D1..D14.

> Versi Inggris: [docs/plan-decoder-thresholding.md](plan-decoder-thresholding.md).

---

## 1. Visi & Tujuan

- Mengangkat batasan **"No local thresholding"** yang dicatat di README pada bagian `## Limitations` dan `## Roadmap` supaya decoder dapat memulihkan QR code dari foto dengan pencahayaan tidak rata, soft shadow, atau gradien cahaya yang kuat.
- Mempertahankan **Otsu global threshold** yang ada sebagai jalur cepat default supaya PNG sintetis dan capture dengan pencahayaan rata tidak menanggung biaya tambahan. Adaptive thresholding hanya aktif ketika output Otsu terbukti buruk.
- Menjaga API publik tetap sama: caller terus memanggil `qrgen.Decode` / `qrgen.DecodeBytes` / `qrgen.DecodeMatrix` tanpa perubahan. Semua perilaku baru bersifat internal di image stage.
- Tetap theory-first dan bilingual: menulis `docs/theory/14-adaptive-thresholding.md` (EN + ID) sebelum kode apa pun mendarat, sejalan dengan doc 11..13.
- Tetap pure Go: tanpa third-party dependency baru. Perhitungan integral image Sauvola cukup sederhana di atas `[]uint8`.

## 2. Prinsip Desain

1. **Otsu dulu, Sauvola sebagai fallback.** Mayoritas input bersih; kita tidak ingin menanggung biaya window-scan Sauvola di setiap decode. Pipeline mencoba Otsu, menjalankan finder detection, dan baru fallback ke Sauvola ketika detection gagal atau ketika Otsu menghasilkan binarisasi dengan rasio foreground-background yang degenerate.
2. **Tidak ada permukaan publik baru di v0.3.** Tidak ada functional option baru, tidak ada sentinel error baru. Keputusan strategi tersembunyi di dalam `decodeImage` supaya API tetap minimal sampai ada user nyata yang meminta kontrol lebih.
3. **Integral image untuk kecepatan.** Sauvola naif memakan biaya `O(width * height * w^2)` untuk window berukuran `w`. Kita pre-compute integral image untuk `sum(x)` dan `sum(x^2)` sehingga mean dan variance per window menjadi `O(1)` per pixel dan total biaya tetap `O(width * height)`.
4. **Tunable tapi punya default.** Window size dan parameter `k` milik Sauvola disimpan sebagai konstanta package-level yang tidak di-export, dipilih mengikuti referensi standar (`w = 25`, `k = 0.2`); kita menahan diri untuk meng-expose-nya sebelum ada bukti dari image set dunia nyata yang menunjukkan default lain lebih baik.
5. **Tes lebih dulu.** Setiap milestone disertai Go test berbasis tabel dan minimal satu kasus round-trip yang gagal di `master` (`Decode` mengembalikan `ErrFinderNotFound`) dan lulus di branch (Decode mengembalikan payload asli).

## 3. Cakupan

### Termasuk di v0.3.0

- **Sauvola adaptive thresholding** dengan akselerasi integral image.
- **Heuristik fallback Otsu -> Sauvola** yang ter-wire di `decodeImage`, di-gate oleh kegagalan jalur Otsu.
- **Theory doc** `14-adaptive-thresholding.md` (EN + ID) yang membahas batasan Otsu, formula Sauvola, integral image, dan trade-off vs Niblack / Bernsen / Adaptive Gaussian.
- **Fixture tes sintetis** untuk pencahayaan tidak rata: linear gradient, radial gradient, vignette, dan soft drop-shadow; di-render secara prosedural di dalam test supaya `testdata/` tetap bersih dari blob biner.
- **Benchmark** yang membuktikan jalur Otsu-only tidak mengalami regresi (`BenchmarkDecodeImageSmall`, `BenchmarkDecodeImageURL` harus tetap dalam 1% dari baseline v0.2.0).
- **Update dokumentasi**: menghapus "No local thresholding" dari Limitations; meng-update Roadmap; menambah entry `v0.3.0` di CHANGELOG.

### Belum termasuk

- **Rotation handling.** Dikelola di plan dan branch paralel (`decoder-rotation`); rilis sebagai v0.4.0.
- **Multi-symbol detection.** Tanpa perubahan.
- **Knob publik untuk parameter Sauvola.** Default only; ditinjau lagi ketika caller mengeluh atau gambar nyata membutuhkan setting berbeda.
- **Adaptive thresholding untuk *encoder*.** Encoding tidak melihat gambar.
- **Alternatif non-Otsu** di luar Sauvola (Niblack, Bernsen, Wolf, Adaptive Gaussian). Dibahas di theory doc, tidak diimplementasi.

---

## 4. Milestone

Milestone dikerjakan berurutan. **Checkpoint A** (setelah T3) memberi fallback Sauvola yang bekerja dan dapat memulihkan setidaknya satu fixture gambar yang gagal di `master`. **Checkpoint B** (T6) adalah rilis `v0.3.0`.

### T1 — Theory Doc Adaptive Thresholding `(S)`

Goal: menutup algoritma baru dan heuristik fallback di `docs/theory/` sebelum kode apa pun mendarat.

- [x] `docs/theory/14-adaptive-thresholding.md` — recap Otsu beserta failure mode-nya (gradient, shadow, low contrast), Niblack sebagai pendahulu Sauvola, formula Sauvola `T(x, y) = mean(x, y) * (1 + k * (std(x, y) / R - 1))` dengan default standar `R = 128`, `k = 0.2`, `w = 25`, konstruksi integral image untuk window query O(1), dan tabel perbandingan vs Niblack / Bernsen / Adaptive Gaussian yang menjelaskan kenapa Sauvola unggul untuk dokumen dan material cetak seperti simbol QR. Juga mendokumentasikan dispatch runtime dua-stage (gate bimodality proaktif `η < η_min` plus post-check reaktif) sehingga milestone implementasi T2 dan T3 hanya perlu merujuk ke section bernama.
- [x] Versi Indonesia `docs/theory/14-adaptive-thresholding.id.md`.
- [x] Update `docs/theory/README.md` dan `docs/theory/README.id.md` untuk menambah entry 14 dengan ringkasan satu baris di subsection baru "Decoder robustness (v0.3.0)", plus satu baris di tabel "Hubungan dengan kode" yang merujuk ke `qrgen/decode_image_sauvola.go` (direncanakan, T2 + T3).

### T2 — Binariser Sauvola `(M)`

Goal: `sauvolaBinarise(img image.Image) *bitmap` yang mengembalikan bentuk `bitmap` yang sama dengan `binarise` yang sudah ada. Tinggal di `qrgen/decode_image_sauvola.go`.

- [x] Bangun dua integral image: `sum` untuk nilai pixel dan `sum2` untuk nilai kuadratnya, keduanya `[]uint64` flattened berukuran `(w+1) * (h+1)` supaya query corner-arithmetic tidak butuh bounds check. Rekurensi running-row-sum menjaga build tetap satu pass linear.
- [x] Helper query `windowMeanStd(sum, sum2, w, h, x, y, half)` yang mengembalikan `(mean, std)` untuk window berpusat dengan half-extent `half`, di-clip pada batas image; menjaga terhadap variance negatif kecil dari floating-point rounding.
- [x] Terapkan formula Sauvola per pixel dan emit `bool` ke dalam struct `bitmap` yang sama dengan yang digunakan Otsu. Konstanta `sauvolaWindow = 25`, `sauvolaK = 0.2`, `sauvolaR = 128.0` berupa value package-level yang tidak di-export.
- [x] Cocokkan konvensi `p <= t` milik Otsu supaya finder detection di hilir tidak berubah.
- [x] Pecah `sauvolaBinariseFromGray` keluar dari `sauvolaBinarise` supaya dispatch T3 dapat memakai ulang buffer grayscale yang sudah dihitung pass Otsu tanpa walk image ulang.
- [x] Tes di `qrgen/decode_image_sauvola_test.go`: nilai integral image yang dicek manual pada fixture 3x2; `windowMeanStd` di-cross-validate terhadap referensi naif O(w^2) di atas buffer 12x10 pseudo-random untuk beberapa half-extent; image uniform tetap all-light (property yang akan disandari gate proaktif di T3); fixture dua-region pencahayaan di mana Sauvola mengklasifikasi semua titik sample ink dan paper dengan benar DAN fixture yang sama membuktikan Otsu gagal di setidaknya satu arah sehingga test tidak dapat lulus sia-sia; image yang lebih kecil dari window tidak panic; image berukuran nol mengembalikan bitmap kosong.

### T3 — Heuristik Fallback di `decodeImage` `(S)`

Goal: memanggil Sauvola hanya ketika output Otsu terlihat tidak sehat, dan melewatkan binarisasi Otsu sepenuhnya ketika histogram sudah membuktikan hasilnya pasti tidak sehat.

- [x] **Pre-check (proaktif, gratis):** `otsuThreshold` sekarang mengembalikan `(threshold, η)` di mana `η = σ²_B / σ²_T` adalah ukuran separability standar di `[0, 1]`. Dispatch di `decodeImage` membaca η dan, ketika nilainya jatuh di bawah `etaMin`, melewati langkah binarisasi Otsu sepenuhnya dan langsung ke Sauvola — menghemat satu pass finder detection penuh di kasus kegagalan.
- [x] **Post-check (reaktif, defense-in-depth):** ketika binarisasi Otsu berjalan, dispatch menganggap outputnya tidak sehat kalau salah satu dari: (a) `findFinders` mengembalikan `ErrFinderNotFound`, atau (b) `foregroundRatio(bm)` jatuh di luar `[foregroundLo, foregroundHi]` (output single-class yang degenerate). Pada kedua kasus tersebut, dispatch me-rebinarise image grayscale dengan Sauvola via buffer grayscale yang dipakai bersama dan menjalankan ulang finder detection. Baru ketika pass Sauvola juga gagal dispatch mengembalikan `ErrFinderNotFound`.
- [x] Default `etaMin = 0.5`, `foregroundLo = 0.05`, `foregroundHi = 0.95` mengikuti paper asli Otsu; catatan empiris yang terekam dari tes: textbook Otsu memberi η ≈ 0.64 untuk Gaussian dan 0.75 untuk distribusi uniform apa pun, sehingga gate proaktif hanya menyala untuk input yang benar-benar degenerate (monokrom / single-delta). Gate reaktif memikul mayoritas beban fallback di v0.3. Tuning menunggu fixture T4.
- [x] Memperkenalkan `binariserUsedState` (tidak di-export) dengan nilai `binariserOtsu`, `binariserSauvolaProactive`, `binariserSauvolaReactive` plus method `String()`, dan `decodeImageDebug` — saudara internal-package dari `decodeImage` yang mengembalikan binariser state bersama dengan text. `decodeImage` sekarang berupa wrapper tipis yang membuang state; tidak ada yang bocor ke API publik.
- [x] Tes di `qrgen/decode_image_sauvola_test.go` menutupi ketiga cabang: PNG bersih yang ter-encode meng-assert `binariserOtsu` plus payload round-trip; image monokrom 80x80 (η=0 via cabang variance-collapse) meng-assert `binariserSauvolaProactive`; mutasi brightness-compression pada QR bersih yang mengangkat ink sisi kanan di atas paper sisi kiri meng-assert `binariserSauvolaReactive` setelah lebih dulu memverifikasi bahwa Otsu sendiri memang gagal pada fixture dan η tetap di atas `etaMin` sehingga cabang proaktif tidak mungkin menyala. Recovery decode end-to-end pada fixture reaktif ditunda ke T4 ketika fixture sintetis ter-kurasi mendarat.
- [x] Helper `otsuBinariseFromGray` dan `foregroundRatio` di-ekstrak supaya dispatch dapat menjalankan Otsu dan Sauvola di atas pass grayscale yang sama tanpa walk image ulang.

### Checkpoint A — Fallback Sauvola memulihkan setidaknya satu gambar yang gagal di v0.2.

### T4 — Fixture Sintetis Uneven Lighting `(S)`

Goal: mengunci coverage regresi pada failure mode pencahayaan yang menjadi target fallback.

- [x] Lima fixture tinggal di `qrgen/decode_image_sauvola_test.go`, semuanya dibangun prosedural dengan `Encode("HELLO")` dilanjutkan `applyPixelTransform` yang membiarkan rectangle modul QR tetap utuh dan membatasi perturbasi hanya pada quiet zone: penggelapan konstan (`TestT4ConstantQuietZoneDarkening`), linear horizontal gradient (`TestT4LinearGradientOnQuietZone`), radial vignette (`TestT4RadialVignetteOnQuietZone`), strip shadow tajam di sepanjang margin kiri (`TestT4DropShadowOnQuietZone`), dan diagonal gradient (`TestT4DiagonalGradientOnQuietZone`).
- [x] Tiap fixture di-assert dengan tiga cara via helper `assertSauvolaRecovery`: Otsu sendiri gagal finder detection (sehingga fallback Sauvola memang melakukan pekerjaan nyata), `DecodeBytes` publik mengembalikan payload asli, dan dispatch state-nya `binariserSauvolaReactive`.
- [x] Kelima fixture jalan di bawah 10 ms tiap-tiapnya di laptop; seluruh batch T4 selesai jauh di bawah 100 ms sehingga test suite tetap gesit.
- [x] **Temuan yang dicatat (dibawa ke T6 dan theory doc).** Failure mode Sauvola yang recoverable adalah **kontaminasi quiet zone**: kontras ink/paper di dalam QR harus tetap utuh agar window lokal Sauvola dapat membedakan tiap modul, sedangkan quiet zone di luar QR boleh digelapkan bebas untuk menarik threshold global Otsu ke atas nilai quiet zone. Mutasi brightness-compression yang diterapkan ke QR itu sendiri (fixture reaktif di T3) memang mengalahkan Otsu tapi diskriminasi lokal Sauvola turun bersama yang global — family input itu untuk saat ini tidak recoverable end-to-end dengan default `sauvolaK = 0.2` dan `R = 128`. Pekerjaan lanjutan (pasca-v0.3, kemungkinan v0.4 bareng rotation handling) bisa meninjau ulang dengan meng-expose opsi `WithBinarisation` atau menambah morphological cleanup setelah Sauvola; untuk v0.3 kita rilis dispatch-nya dan mendokumentasikan batasannya secara jujur.
- [x] Varian low-contrast standalone sengaja dihilangkan: kompresi global yang uniform (tanpa variasi spasial) membiarkan struktur between-class Otsu tetap utuh dan ter-decode oleh Otsu fast path yang sudah ditutupi `TestDispatchUsesOtsuOnCleanPNG` selama QR area-nya bertahan.

### T5 — Benchmark & Pengaman Regresi `(S)`

Goal: membuktikan jalur Otsu-only tidak mengalami regresi dan mengukur overhead Sauvola.

- [ ] Jalankan ulang `BenchmarkDecodeImageSmall`, `BenchmarkDecodeImageMultiBlock`, `BenchmarkDecodeImageURL`, `BenchmarkDecodeImageFromPNGDecode` dan konfirmasi allocations dan ns/op tetap dalam 1% dari baseline master (catat angka before/after di commit message).
- [ ] Tambahkan `BenchmarkDecodeImageSauvolaFallback` yang memaksa jalur fallback (fixture gradient) supaya biaya Sauvola terlihat di `go test -bench`.
- [ ] `go test -race ./...` tetap bersih.

### T6 — Polish & Rilis `(S)`

Goal: memotong `v0.3.0`.

- [ ] Update README: hapus bullet `**No local thresholding.**` dari `## Limitations` dan klausa `local thresholding` yang setara dari `## Roadmap`. Ganti dengan kalimat satu baris "Adaptive thresholding (Sauvola fallback)" di bawah `## Decoding QR codes` atau di sub-paragraf baru di section tersebut yang menjelaskan fallback berjalan otomatis.
- [ ] Entry `CHANGELOG.md` `v0.3.0` di bawah `### Added` (binariser Sauvola, heuristik fallback, theory doc 14, benchmark) dan `### Validated` (fixture sintetis uneven-lighting untuk varian linear, radial, diagonal, drop-shadow, low-contrast; jalur Otsu hot path tetap dalam 1% dari baseline v0.2).
- [ ] Bump konstanta versi module-level hanya kalau kita memang menambahnya; selain itu cukup CHANGELOG dan tag yang membawa penanda v0.3.0.
- [ ] Tag `v0.3.0` setelah push pertama ke GitHub supaya tag mendarat pada commit yang dilihat remote: `git tag -a v0.3.0 -m "Adaptive thresholding release" && git push origin v0.3.0`.

---

## 5. Usulan Delta Layout File

Semua kode baru mendarat di dalam `qrgen/` di samping image stage decoder yang sudah ada. Tidak ada direktori package baru.

```
qrgen/
├── decode_image.go              # eksisting — mendapat dispatch Otsu-atau-Sauvola
├── decode_image_sauvola.go      # baru — sauvolaBinarise + helper integral image
├── decode_image_sauvola_test.go # baru — unit test Sauvola + integration test fallback
└── decode_bench_test.go         # eksisting — mendapat BenchmarkDecodeImageSauvolaFallback
docs/
├── plan-decoder-thresholding.md     # file ini (versi Inggris)
├── plan-decoder-thresholding.id.md  # file ini
└── theory/
    ├── 14-adaptive-thresholding.md
    └── 14-adaptive-thresholding.id.md
```

## 6. Risiko & Catatan Teknis

- **Overflow integer di integral image.** Image 4096x4096 dengan grey 8-bit menjumlahkan paling banyak `4096 * 4096 * 255 = ~4.3 * 10^9` untuk `sum` dan `~1.1 * 10^12` untuk `sum2`. Keduanya nyaman muat di `uint64` namun meledak di `uint32`. Kita pakai `uint64` di seluruh jalur.
- **Window size Sauvola pada simbol kecil.** Untuk V1 dengan rendering module-size 4x, simbol punya lebar sekitar 84 px, jadi window 25 px menutupi sekitar 7 modul; itu neighbourhood lokal yang masuk akal. Untuk module size yang sangat besar window menyusut secara relatif; theory doc mencatat caveat ini tanpa mengubah default.
- **False positive di finder detection pasca-Sauvola.** Sauvola dapat memunculkan speckle hitam kecil di region uniform; pengecekan rasio 1:1:3:1:1 dan validasi right-angle geometry di finder pattern detection seharusnya menolak speckle ini, tapi kita tetap menjaga regression test yang melempar fixture gaussian-noise ke pipeline untuk konfirmasi.
- **Reproduksibilitas floating-point.** Perhitungan `std` Sauvola memakai `sqrt`; kita tetap di `float64` dan menerima bahwa threshold yang persis sama hanya stabil lintas platform sebatas presisi yang dijamin standard library Go (tanpa `cgo`, tanpa kejutan vectorisasi SIMD).
- **Divergensi branch dengan kerja rotation.** Branch rotation (`decoder-rotation`) akan mendarat belakangan dan kemungkinan menyentuh `decodeImage` di region yang sama. Kita meminimalkan permukaan konflik dengan mengisolasi Sauvola di balik satu helper dan menjaga logika dispatch di `decodeImage` tetap pada blok kecil `if !found || !healthy`.

## 7. Referensi

- Sauvola, J., Pietikainen, M. — "Adaptive document image binarization," *Pattern Recognition*, 33(2):225–236, 2000. Paper kanonik untuk formula dan default `k = 0.2`, `R = 128`.
- Niblack, W. — *An Introduction to Digital Image Processing*, Prentice-Hall, 1986. Pendahulu Sauvola, dimasukkan ke theory doc sebagai motivasi.
- Shafait, F., Keysers, D., Breuel, T. M. — "Efficient implementation of local adaptive thresholding techniques using integral images," *Document Recognition and Retrieval XV*, SPIE, 2008. Trik integral image yang menjaga Sauvola tetap O(width * height).
- Otsu, N. — "A threshold selection method from gray-level histograms," *IEEE Trans. Systems, Man, and Cybernetics*, 9(1):62–66, 1979. Sudah dikutip di doc 12; dicantumkan ulang di sini untuk bahasan failure mode.

## 8. Pertanyaan Terbuka

Untuk dijawab sebelum milestone yang bersangkutan dimulai:

- **Parameter Sauvola: hard-coded vs configurable.** Default ke hard-coded `w = 25`, `k = 0.2`, `R = 128`. Ditinjau ulang kalau fixture dunia nyata menuntut tuning, tapi belum menambah option publik di v0.3.
- **Escape hatch always-Sauvola.** Apakah kita perlu meng-expose hook internal "force Sauvola" bagi user yang tahu input mereka selalu uneven? Ditunda: satu option `WithBinarisation(strategy)` dapat masuk di v0.4 bersama kerja rotation kalau memang ada permintaan.
- **Presisi deteksi kegagalan Otsu.** Plan saat ini menggabungkan gate bimodality proaktif (`η < η_min` melewati Otsu) dengan post-check reaktif (kegagalan finder detection OR foreground ratio di luar `[0.05, 0.95]`). Default `η_min = 0.5`; nilai eksaknya, dan apakah bimodality saja sudah cukup untuk memensiunkan post-check, layak diukur pada set fixture sintetis T4 sebelum dikunci.
- **Penempatan theory doc.** Apakah doc 14 sebaiknya berdiri sendiri atau jadi subsection baru di bawah doc 12? Pilihan jatuh ke standalone supaya doc 12 tetap dibekukan sebagai catatan v0.2.
