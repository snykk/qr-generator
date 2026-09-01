# QR Decoder — Rencana Robustness Sudut Sembarang

Dokumen ini menjelaskan rencana implementasi **deteksi finder sudut-sembarang** untuk rilis minor `v0.13.0`. Ini memperluas decoder untuk melokasi simbol yang miring atau terputar jauh melewati pita ~30 derajat yang ditangani line scanner 1:1:3:1:1 sekarang, menutup limitation decoder terakhir yang tercatat. Ini milestone tersulit sejauh ini — ini computer vision, bukan transkripsi spec — jadi rencananya jujur soal pendekatan, validasi, dan di mana batas cakupan realistisnya.

> Status: **draf / dokumen hidup.** Milestone DR1..DR5 mendarat bertahap di branch `decoder-robustness`; tiap milestone adalah commit fokus (atau deret commit kecil) berikut test, mengikuti irama milestone sebelumnya.

> Versi Inggris: [docs/plan-decoder-robustness.md](plan-decoder-robustness.md).

---

## 1. Visi & Tujuan

- Men-decode foto QR yang diambil pada sudut curam — pita tilt tempat pipeline sekarang gagal — tanpa intervensi pemanggil, sama seperti rotation handling v0.4 dan adaptive thresholding v0.3 yang otomatis.
- Memperbaiki stage yang benar-benar patah di bawah perspektif. Plan awalnya mengira itu deteksi finder; pengukuran menunjukkan itu **homografi afin tiga-pusat**, yang tak bisa membalik keystoning perspektif (bagian 2). Jadi milestone ini mengestimasi **homografi projektif dari sudut finder** dan menyerahkan deteksi finder ke pass connected-component yang memasok sudut-sudut itu.
- Menjaga jalur cepat tetap cepat: pipeline projektif jalan sebagai **fallback**, hanya saat decode afin gagal, meniru fallback Otsu -> Sauvola (doc 14). Simbol tegak dan berotasi mempertahankan kecepatan dan hasilnya sekarang.
- Menjaga **zero runtime dependency**: detektor dan solver homografi murni Go; fixture test tilt di-generate oleh warp gambar in-tree, tanpa OpenCV atau `x/image`.
- **Jujur soal batas.** CV itu empiris: milestone melebarkan pita kerja dan mendokumentasikan rentang sudut yang benar-benar dilewatinya pada fixture sintetis, alih-alih mengklaim robustness universal.

## 2. Kenapa decoding gagal saat miring, dan perbaikannya

> Koreksi arah (ditemukan empiris saat implementasi). Plan awal berasumsi deteksi finder adalah bottleneck: bahwa line scan **1:1:3:1:1** (`fitsFinderRatio`, +-50% per modul) patah di bawah foreshortening perspektif lewat ~30 derajat. Diukur terhadap fixture tilt sintetis, itu keliru dalam dua hal. Pertama, toleransi longgar line scan tetap menemukan ketiga finder sampai ~55 derajat. Kedua, dan menentukan, decoding gagal mulai ~20 derajat **bahkan ketika finder sudah ditemukan** — karena `homographyFromFinders` membangun map **afin** dari tiga pusat finder, dan map afin tak bisa membalik keystoning perspektif. Sebuah test kontrol memastikannya: menyampel fixture miring yang sama dengan homografi **projektif** ground-truth mendekode bersih sampai 70 derajat. Jadi penghambat sebenarnya adalah tahap geometri, bukan deteksi finder.

Perbaikan yang robust adalah **homografi projektif yang diestimasi dari sudut pola finder**. Projective transform persis merupakan map antara bidang dan citra perspektifnya; ia butuh empat-plus korespondensi yang tidak degenerate. Tiga pusat finder hanya memberi tiga titik tak sempurna (centroid finder bukan invarian projektif), tetapi tiap finder adalah persegi dengan empat sudut, jadi tiga finder memberi dua belas korespondensi sudut yang tersebar di seluruh simbol. Pipeline-nya menjadi:

1. Lokasi finder lewat analisis **connected-component** (union-find dua-pass + nesting gelap/terang/gelap dengan sisi bounding box 7:5:3), yang labelling-nya juga membiarkan kita mengunjungi ulang piksel untuk sudut. Ini menjangkau ~55 derajat yang sama seperti line scan; di luar itu keduanya tak mendeteksi finder.
2. Ekstrak empat sudut cincin-luar tiap finder (metode titik-terjauh, agnostik rotasi) dan tetapkan tiap sudut koordinat modulnya lewat sumbu simbol.
3. Selesaikan **homografi projektif least-squares** atas dua belas sudut (`computeHomographyLS`, normal equations).
4. Karena version tak bisa dibaca andal dari simbol yang foreshorten, seed dari panjang sisi quad finder dan **trial** version-version terdekat, membiarkan validasi Reed-Solomon memilih yang mendekode.
5. Rangkai semuanya sebagai **fallback**: `decodeImage` menjalankan pipeline afin yang tak diubah lebih dulu dan baru jatuh ke yang projektif saat itu gagal decode, jadi simbol tegak dan berotasi tetap byte-for-byte.

## 3. Cakupan

### Termasuk dalam v0.13.0

- Detektor finder connected-component (`qrgen/decode_finder_cc.go` atau serupa): CCL dua-pass, analisis lubang/nesting, pencocokan konsentrisitas dan rasio-area, menghasilkan pusat `finderCandidate` yang kompatibel dengan pipeline `finderTriple` yang ada.
- Menyambungkannya sebagai **fallback** di `decodeImageDebug`: coba line scanner cepat dulu; saat gagal, jalankan detektor component sebelum menyerah. Catatan bergaya `binariserUsedState` baru bisa merekam detektor mana yang berhasil, untuk test.
- Fixture test tilt sintetis: warp gambar in-tree yang me-render QR dan memproyeksikannya pada tilt/rotasi tertentu, plus test round-trip decode lintas rentang sudut yang menetapkan pita kerja.
- Dokumen referensi `docs/theory/24-arbitrary-angle-detection.md` (EN + ID), dan pembaruan README/limitations mencerminkan pita baru.

### Belum termasuk (masih)

- **Robustness universal.** Ini CV best-effort: blur berat, glare yang merusak nesting finder, lipatan/lengkungan, atau sudut hampir-90-derajat yang menyerempet mungkin tetap gagal. Dokumen menyatakan pita terukurnya.
- **Deteksi multi-simbol** (beberapa QR dalam satu gambar). Satu simbol, seperti sekarang.
- **Parameter Sauvola tunable dan morphological cleanup.** Tuas robustness terpisah (separuh lain dari butir roadmap); ditunda agar milestone ini fokus ke detektor finder.
- **API publik baru.** `Decode`/`DecodeBytes` mendapat kapabilitas ini transparan; tanpa perubahan signature.

---

## 4. Milestone

Milestone mendarat berurutan. **Checkpoint A** (setelah DR4) men-decode tilt sintetis lintas pita target lewat `Decode` publik. **Checkpoint B** (DR5) adalah rilis `v0.13.0`.

### DR1 — Plan Doc `(S)`

- [ ] `docs/plan-decoder-robustness.md` dan padanan Indonesianya, mencakup visi, analisis kenapa-gagal, cakupan, milestone DR1..DR5, delta tata letak file, risiko, referensi, dan pertanyaan terbuka.

### DR2 — Dokumen Referensi `(S)`

Tujuan: mendokumentasikan pendekatan dan buktinya sebelum kode.

- [ ] `docs/theory/24-arbitrary-angle-detection.md` — kenapa line scan 1:1:3:1:1 bertahan di bawah rotasi tetapi patah di bawah foreshortening perspektif lewat ~30 derajat; wawasan nesting-topologis dan kenapa ia homography-invariant; connected-component labelling; signature nesting/konsentrisitas/rasio-area finder; strategi fallback dan kenapa ia menjaga jalur cepat; serta metode validasi tilt-sintetis. Ditutup dengan penunjuk implementasi dan batas cakupan yang jujur.
- [ ] Padanan Indonesia `docs/theory/24-arbitrary-angle-detection.id.md`.
- [ ] Memperbarui `docs/theory/README.md` dan `.id.md`: entri 24 plus baris pemetaan kode.

### DR3 — Detektor Finder Connected-Component `(L)` — selesai

Tujuan: detektor component, diuji pada bitmap yang dikonstruksi.

- [x] Connected-component labelling dua-pass atas bitmap terbinarisasi (union-find), menghasilkan bounding box, area, centroid, warna per-component, dan peta label per-piksel.
- [x] Analisis nesting: mencocokkan triple konsentris gelap -> terang -> gelap dengan sisi bounding box 7:5:3-ish menjadi finder candidate; dedupe nyaris-duplikat.
- [x] Unit test pada bitmap buatan-tangan: finder sintetis terdeteksi di pusat sejatinya; persegi polos dan cincin tanpa pusat gelap ditolak; tiga finder menghasilkan triple terurut.

### DR4 — Geometri Projektif + Round-Trip Tilt `(L)` — selesai

Tujuan: men-decode simbol miring nyata end-to-end. Di sinilah koreksi arah mendarat (bagian 2): alih-alih menyuap pusat finder ke pipeline afin, ekstrak sudut finder dan fit homografi projektif.

- [x] Ekstraksi sudut dari cincin luar tiap finder (`extractQuadCorners`) dan pengurutan quad (`findFinderQuadTriple`).
- [x] `homographyFromFinderQuads` menetapkan tiap sudut koordinat modulnya lewat sumbu simbol dan menyelesaikan homografi projektif least-squares (`computeHomographyLS`).
- [x] Seed version dari panjang sisi quad finder plus trial version terurut-kedekatan yang divalidasi Reed-Solomon (`decodeProjectiveFromBitmap`).
- [x] `decodeImage` menjalankan pipeline afin lebih dulu dan jatuh ke projektif hanya saat gagal decode; error afin dipertahankan. Kasus tegak dan berotasi tetap byte-for-byte (test batas v0.4 yang dulu meng-assert kegagalan 45 derajat kini meng-assert pemulihan lewat jalur projektif).
- [x] Warp tilt in-tree (`warpTilt`) dan `TestArbitraryAngleRoundTrip`; sweep merekam pita yang lolos dan `TestUprightUsesAffinePath` menjaga jalur cepat.

### Checkpoint A — Decode publik membaca tilt sintetis lintas pita target. Tercapai: andal sampai ~35-40 derajat lintas orientasi, ~55 derajat pada rotasi menguntungkan, dibatasi ~55-60 derajat oleh deteksi finder.

### DR5 — Polish & Rilis `(S)`

Tujuan: memotong `v0.13.0`.

- [ ] README: bagian decoding mencatat pita sudut yang melebar dan batas jujurnya; butir Limitations "arbitrary-angle" diperbarui; Roadmap dan Scope diperbarui.
- [ ] Entri `v0.13.0` di `CHANGELOG.md` plus anchor compare/tag ditulis; dibiarkan unstaged untuk maintainer (meniru v0.6 sampai v0.12).
- [ ] `go test -race ./...` bersih, gofmt bersih.
- [ ] Tag `v0.13.0` (diserahkan ke maintainer sesuai alur git/rilis yang sudah mapan; anotasi direkomendasikan di percakapan rilis).

---

## 5. Usulan Delta Tata Letak File

```
qrgen/
├── decode_finder_cc.go        # baru — CC labelling, cocok nesting, ekstraksi sudut, triple quad
├── decode_finder_cc_test.go   # baru — unit test CCL + nesting
├── decode_image.go            # homografi projektif, trial version, dan decodeImage afin-lalu-projektif
├── decode_tilt_test.go        # baru — warp tilt in-tree + round-trip sudut-sembarang
docs/
├── plan-decoder-robustness.md      # file ini
├── plan-decoder-robustness.id.md   # padanan Indonesia
└── theory/
    ├── 24-arbitrary-angle-detection.md     # baru
    └── 24-arbitrary-angle-detection.id.md  # baru
```

## 6. Risiko & Catatan Teknis

- **Ini tak sepenuhnya tertutup, dan itu terdokumentasi.** Tak seperti fitur spec, pipeline CV tak punya success rate terjamin. Pitanya empiris (DR4 Checkpoint A) dan bergantung orientasi; README dan theory doc menyatakannya jujur alih-alih mengklaim ketangguhan universal. Ini peningkatan nyata — decoding yang dulu gagal mulai ~20 derajat kini menjangkau ~35-55 — bukan solusi lengkap.
- **Bottleneck-nya geometri, bukan deteksi.** Asumsi awal (bahwa line scan finder gagal lebih dulu) terbantah oleh pengukuran; homografi afin tiga-centroid-lah yang membatasi pipeline lama, dan centroid finder bukan invarian projektif. Perbaikannya mengestimasi transformasi projektif dari dua belas sudut finder.
- **Version di bawah perspektif.** Jarak antar-finder foreshorten dan estimasi modul bounding box membengkak, jadi estimasi version tak andal. Dimitigasi dengan seed sisi-sudut dan trial version terurut-kedekatan yang digerbangi Reed-Solomon, yang membuat version salah gagal cepat dan yang benar mendekode.
- **Fallback menjaga jalur cepat.** Jalur projektif (labelling CC plus trial version terbatas) lebih mahal dari pipeline afin, jadi ia jalan hanya saat decode afin gagal. Simbol tegak dan berotasi tak pernah membayarnya, dan test decode serta timing yang ada tak terpengaruh.
- **Warp test-fixture bukan produk.** Warp tilt hidup hanya di test; ia memfabrikasi input untuk membuktikan pipeline, dan kualitas sampling-nya sendiri hanya memengaruhi seberapa sulit gambar test, bukan library.

## 7. Referensi

- ISO/IEC 18004:2015 — struktur finder pattern (modul konsentris 7x7/5x5/3x3) dan quiet zone yang diandalkan detektor.
- `docs/theory/12-image-processing.md`, `14-adaptive-thresholding.md`, dan `15-rotation-handling.md` — binarisasi yang ada, pola fallback Sauvola, dan pengurutan finder rotation-invariant yang milestone ini bangun di atasnya.
- Connected-component labelling (union-find dua-pass) — algoritma segmentasi gambar standar, diimplementasi pure Go.
- Detektor QR ZXing / OpenCV — prior art untuk lokasi finder berbasis kontur/component pada sudut sembarang, dipakai sebagai referensi konseptual (bukan dependency).

## 8. Pertanyaan Terbuka

Dijawab sebelum milestone terkait dimulai:

- **Algoritma detektor.** CCL union-find dua-pass dengan analisis lubang (diusulkan) versus test rasio multi-arah per-candidate di centroid component. Pendekatan topologis lebih perspective-robust; konfirmasi di DR3.
- **Threshold signature finder.** Toleransi konsentrisitas dan deviasi rasio-area yang diterima untuk nesting 7:5:3 — di-tune terhadap fixture sintetis di DR3/DR4.
- **Fallback vs. selalu-jalan.** Diusulkan: fallback hanya saat line scanner gagal, untuk melindungi jalur cepat. Konfirmasi ini tak melewatkan kasus di mana line scanner mengembalikan triple yang salah (bukan kosong); kalau begitu, pertimbangkan menjalankan keduanya dan merekonsiliasi.
- **Pita yang diklaim.** Rentang sudut mana yang akan dinyatakan README/doc sebagai reliably-decoded, ditetapkan empiris dari hasil DR4 alih-alih diklaim di muka.
