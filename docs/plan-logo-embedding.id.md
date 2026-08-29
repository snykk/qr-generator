# QR Encoder — Rencana Logo Embedding

Dokumen ini menjelaskan rencana implementasi **logo embedding** untuk rilis minor `v0.12.0`. Ini membiarkan pemanggil menaruh logo di tengah simbol QR — tampilan branded-QR yang umum — sambil menjaga simbol tetap bisa dipindai dengan bersandar pada budget error-correction. Ini milestone pertama setelah himpunan mode encoder rampung (Kanji di v0.11), beralih ke karakter output ketimbang encoding.

> Status: **draf / dokumen hidup.** Milestone L1..L5 mendarat bertahap di branch `logo-embedding`; tiap milestone adalah commit fokus (atau deret commit kecil) berikut test, mengikuti irama milestone M, D, T, R, S, MM, P, TR, ECI, SA, dan K.

> Versi Inggris: [docs/plan-logo-embedding.md](plan-logo-embedding.md).

---

## 1. Visi & Tujuan

- Membiarkan pemanggil mengoper gambar logo dengan `WithLogo` dan menaruhnya di tengah simbol hasil render, baik di output PNG maupun SVG, tanpa perubahan lain pada cara mereka memanggil `Encode`/`EncodeSVG`.
- Menjaga simbol tetap **bisa dipindai** dengan bersandar pada recovery Reed–Solomon: logo menutupi sebagian modul, yang menjadi error yang dikoreksi decoder, jadi inti milestone ini adalah memilih ukuran logo default yang aman dan error-correction level tinggi, bukan encoding baru.
- Membuatnya **otomatis di tempat yang penting**: meminta logo menaikkan error-correction level ke H (level recovery ~30%) kecuali pemanggil sudah meminta setidaknya itu, sehingga area tertutup tetap di dalam budget tanpa pemanggil perlu berpikir soal itu.
- Menjaga **zero runtime dependency**: penskalaan logo agar pas memakai resampler kecil in-tree (standard library tak punya image scaler), dan jalur SVG menyisipkan logo sebagai data URI supaya dokumen tetap self-contained.
- Filosofinya sama dengan tiap milestone sebelumnya: pure Go, reference-first dengan dokumen bilingual, dan test — di sini round-trip decode-tetap-jalan (lewat decoder kita dan gozxing) yang membuktikan logo tak merusak pemindaian.

## 2. Prinsip Desain

1. **Logo adalah overlay saat render, bukan perubahan encoding.** Matrix dibangun persis seperti sekarang (di EC level tinggi); logo digambar ke raster jadi atau disisipkan ke SVG. Encoding, masking, dan decoder tak tersentuh.
2. **Satu option, dua format.** `WithLogo(Logo)` membawa gambar dan presentasinya (scale, background, padding). `Encode` menaruhnya di PNG dan `EncodeSVG` menyisipkannya di SVG; `EncodeTerminal` mengabaikannya (output teks tak punya tempat untuk gambar), sama seperti `WithModuleSize` yang no-op di sana.
3. **Default aman, bisa disetel.** Logo default menutupi ~20% lebar simbol dan duduk di atas background pad putih membulat (quiet zone lokal supaya terbaca bersih dan tak menyatu dengan modul). Keduanya bisa diatur lewat struct `Logo`.
4. **EC-H otomatis.** `resolveOptions` menaikkan EC level ke H kapan pun ada logo dan pemanggil menyetel level lebih rendah, sehingga budget recovery menutupi okusi. Ini order-independent (diterapkan setelah semua option).
5. **Penskalaan zero-dependency.** Logo di-resample ke box target-nya dengan box-filter downscale in-tree (dan nearest-neighbour upscale), karena `image/draw` tak bisa menskala dan `golang.org/x/image` akan jadi dependency baru. Alpha dikomposit di atas background pad dengan `image/draw`.
6. **Test membuktikan tetap terpindai.** Test utama meng-encode teks dengan logo default, men-decode-nya, dan memastikan teks asli kembali — modul tertutup menjadi error Reed–Solomon yang dipulihkan koreksi — plus cross-check gozxing.

## 3. Cakupan

### Termasuk dalam v0.12.0

- Struct config `Logo` (`Image image.Image`, `Scale float64`, `Background color.Color`, `Padding` / corner-radius) dan `WithLogo(Logo) Option`.
- PNG: mengomposit logo yang diskala (di atas background pad) ke image hasil render sebelum encoding, lewat field `logo` yang dialirkan lewat `renderOptions`.
- SVG: menyisipkan logo sebagai `<image>` di tengah dengan data URI base64, di atas `<rect>` background.
- Bump EC-H otomatis saat ada logo.
- Resampling logo in-tree (box-filter downscale, nearest-neighbour upscale).
- Dokumen referensi `docs/theory/23-logo-embedding.md` (EN + ID), contoh runnable, dan bagian penggunaan README.

### Belum termasuk (masih)

- **Menjamin keterbacaan untuk logo kebesaran.** Library memilih default aman dan EC-H, tetapi pemanggil yang menyetel `Scale` sangat besar bisa melampaui budget recovery; dokumen menyatakan panduannya dan milestone tak mencoba solver okusi per-simbol yang eksak.
- **QR artistik** (recolor per-modul, gaya dot, gradient fill, gambar di dalam modul). Hanya overlay logo di tengah.
- **Logo terminal.** `EncodeTerminal` mengabaikan `WithLogo`.
- **Resampling fotografis berkualitas tinggi.** Resampler in-tree cukup baik untuk logo; pemanggil yang mau output kualitas-Lanczos bisa menskala gambar sendiri lebih dulu.

---

## 4. Milestone

Milestone mendarat berurutan. **Checkpoint A** (setelah L3) memberi logo yang bisa dipindai di output PNG. **Checkpoint B** (L5) adalah rilis `v0.12.0`.

### L1 — Plan Doc `(S)`

- [x] `docs/plan-logo-embedding.md` dan padanan Indonesianya, mencakup visi, prinsip, cakupan, milestone L1..L5, delta tata letak file, risiko, referensi, dan pertanyaan terbuka.

### L2 — Dokumen Referensi Logo Embedding `(S)`

Tujuan: mendokumentasikan penalaran recovery-budget dan pendekatan compositing sebelum kode.

- [x] `docs/theory/23-logo-embedding.md` — kenapa logo di tengah terpindai (recovery Reed–Solomon, budget ~30% di EC-H, kenapa bagian tengah menghindari finder dan timing pattern); ukuran default aman dan alasannya; bump EC-H otomatis; background pad; compositing PNG versus penyisipan data-URI SVG; dan pilihan resampling zero-dependency. Ditutup dengan penunjuk implementasi.
- [x] Padanan Indonesia `docs/theory/23-logo-embedding.id.md`.
- [x] Memperbarui `docs/theory/README.md` dan `.id.md`: entri 23 plus baris pemetaan kode yang menunjuk ke `qrgen/logo.go`.

### L3 — WithLogo + Compositing PNG `(M)`

Tujuan: logo yang bisa dipindai di output PNG.

- [x] `qrgen/logo.go` dengan struct `Logo`, `WithLogo`, resampler, dan compositing PNG (background pad + logo diskala di atas image). `renderOptions` mendapat field `logo`; `Encode` mengalirkannya.
- [x] `resolveOptions` menaikkan EC ke H saat ada logo dan level-nya lebih rendah.
- [x] Test: round-trip decode-tetap-jalan lewat decoder kita pada ukuran default; logo terlalu besar tetap decode pada default lebih kecil yang didokumentasikan atau ditandai; resampler downscale dan upscale dengan benar; encode tanpa-logo byte-for-byte tak berubah.

### Checkpoint A — logo ukuran default terkomposit ke PNG dan simbol tetap decode.

### L4 — Penyisipan SVG + Round-Trip + Contoh `(S)`

Tujuan: jalur SVG dan validasi independen.

- [x] `EncodeSVG` menyisipkan logo sebagai `<image>` di tengah (data URI base64) di atas rect background; SVG tetap self-contained dan well-formed.
- [x] `TestRoundTripWithThirdPartyDecoder` mendapat satu kasus with-logo (PNG); gozxing membacanya, mengonfirmasi logo tak mengalahkan decoder independen.
- [x] Contoh runnable `examples/encode/logo/main.go` yang menulis QR ber-logo ke PNG dan SVG.

### L5 — Polish & Rilis `(S)`

Tujuan: memotong `v0.12.0`.

- [x] README: bagian penggunaan logo; baris `WithLogo` dan tipe `Logo` di ringkasan API; butir Roadmap "Logo embedding" ditandai selesai; Scope diperbarui.
- [x] Entri `v0.12.0` di `CHANGELOG.md` plus anchor compare/tag ditulis; dibiarkan unstaged di working tree untuk maintainer (meniru v0.6 sampai v0.11).
- [x] `go test -race ./...` bersih, gofmt bersih.
- [ ] Tag `v0.12.0` (diserahkan ke maintainer sesuai alur git/rilis yang sudah mapan; anotasi direkomendasikan di percakapan rilis).

---

## 5. Usulan Delta Tata Letak File

```
qrgen/
├── logo.go                # baru — struct Logo, WithLogo, resampler, compositing
├── logo_test.go           # baru — round-trip decode-tetap-jalan, test resampler
├── options.go             # +field logo; auto EC-H di resolveOptions
├── render_png.go          # gambar logo ke image sebelum encoding
├── render_svg.go          # sisipkan data URI <image> logo
├── api.go                 # alirkan logo ke para renderer
├── roundtrip_test.go      # +kasus with-logo gozxing
docs/
├── plan-logo-embedding.md      # file ini
├── plan-logo-embedding.id.md   # padanan Indonesia
└── theory/
    ├── 23-logo-embedding.md     # baru
    └── 23-logo-embedding.id.md  # baru
examples/encode/logo/
└── main.go                # baru — QR ber-logo ke PNG dan SVG
```

## 6. Risiko & Catatan Teknis

- **Budget recovery adalah inti segalanya.** Logo terpindai hanya karena error correction memulihkan modul yang tertutup. EC-H memulihkan sekitar 30% codeword; logo ~20% tetap nyaman di dalam itu, tetapi logo lebih besar, background ramai, atau logo yang menimpa finder pattern (tak boleh) merusaknya. Default menaruh logo di tengah, menjaganya kecil, dan memaksa EC-H; dokumen menyatakan plafonnya.
- **Tak ada scaler stdlib.** `image/draw` tak bisa mengubah ukuran, jadi milestone mengirim resampler kecil. Box-filter downscale menghindari aliasing yang akan disebabkan nearest-neighbour pada kasus umum (logo besar dikecilkan agar pas); upscale logo kecil memakai nearest-neighbour. Menambah `golang.org/x/image` akan melanggar aturan zero-dependency dan ditolak.
- **Output default tak boleh bergeser.** Semua dipagari `WithLogo`; encode tanpa-logo byte-for-byte identik, dijaga test dan golden fixture yang utuh.
- **Alpha dan background pad.** Logo dengan transparansi digambar `draw.Over` pad putih, jadi region transparan terbaca sebagai quiet zone lokal alih-alih menampilkan modul di belakangnya. Pad juga memberi logo pemisahan visual dari kode.
- **Self-containment SVG.** Logo disisipkan sebagai data URI base64, bukan `href` eksternal, jadi SVG tetap satu file portable, konsisten dengan tujuan renderer v0.5.

## 7. Referensi

- ISO/IEC 18004:2015 — struktur error-correction Reed–Solomon dan empat EC level (L/M/Q/H) yang kapasitas recovery-nya dikonsumsi logo di tengah.
- `docs/theory/04-reed-solomon.md` dan `docs/theory/08-rendering.md` / `16-svg-rendering.md` — catatan error-correction dan rendering yang logo embedding bangun di atasnya.
- Praktik umum QR branded — logo di tengah pada 15–25% dengan EC level tinggi dan background pad, pendekatan de-facto yang milestone ini ikuti.
- Go standard library — `image`, `image/draw` (compositing), `image/color`, dan `encoding/base64` (data URI SVG); sengaja bukan `golang.org/x/image`.

## 8. Pertanyaan Terbuka

Dijawab sebelum milestone terkait dimulai:

- **Bentuk option.** `WithLogo(Logo)` dengan struct config (diusulkan, ada ruang untuk scale/background/padding) versus `WithLogo(image.Image)` telanjang plus option penyetelan terpisah. Struct menjaga call site rapi untuk beberapa knob itu.
- **Scale default.** 0.2 (20% lebar simbol) diusulkan sebagai default aman di bawah EC-H; konfirmasi terhadap test decode, dan putuskan apakah clamp atau warn di atas ambang.
- **Kebijakan bump EC.** Paksa setidaknya H kapan pun ada logo (diusulkan) versus hanya default ke H dan membiarkan level lebih rendah yang eksplisit berlaku. Memaksa setidaknya H adalah pilihan aman; konfirmasi di L3.
- **Bentuk background pad.** Pad rounded-rectangle putih (diusulkan) versus persegi polos versus tanpa pad. Konfirmasi default-nya dan apakah radius-nya dipaparkan.
- **Kualitas resampler.** Box-filter downscale plus nearest-neighbour upscale (diusulkan) versus nearest-neighbour saja. Konfirmasi box filter sepadan dengan kode ekstra demi kualitas logo.
