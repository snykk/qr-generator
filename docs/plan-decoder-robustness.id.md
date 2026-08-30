# QR Decoder — Rencana Robustness Sudut Sembarang

Dokumen ini menjelaskan rencana implementasi **deteksi finder sudut-sembarang** untuk rilis minor `v0.13.0`. Ini memperluas decoder untuk melokasi simbol yang miring atau terputar jauh melewati pita ~30 derajat yang ditangani line scanner 1:1:3:1:1 sekarang, menutup limitation decoder terakhir yang tercatat. Ini milestone tersulit sejauh ini — ini computer vision, bukan transkripsi spec — jadi rencananya jujur soal pendekatan, validasi, dan di mana batas cakupan realistisnya.

> Status: **draf / dokumen hidup.** Milestone DR1..DR5 mendarat bertahap di branch `decoder-robustness`; tiap milestone adalah commit fokus (atau deret commit kecil) berikut test, mengikuti irama milestone sebelumnya.

> Versi Inggris: [docs/plan-decoder-robustness.md](plan-decoder-robustness.md).

---

## 1. Visi & Tujuan

- Men-decode foto QR yang diambil pada sudut curam — pita 30..90 derajat tempat finder scanner sekarang gagal — tanpa intervensi pemanggil, sama seperti rotation handling v0.4 dan adaptive thresholding v0.3 yang otomatis.
- Melakukannya dengan memperbaiki **satu** stage yang gagal di sudut curam: **deteksi finder**. Segala setelahnya (pengurutan finder, estimasi version, homography, sampling modul) sudah berupa projective transform dan menangani tilt sembarang; hanya line scan 1:1:3:1:1 yang melokasi finder yang patah di bawah perspektif kuat. Jadi milestone ini menambah detektor finder yang rotation-and-perspective-invariant dan memakai ulang pipeline geometri yang ada apa adanya.
- Menjaga jalur cepat tetap cepat: detektor baru jalan sebagai **fallback**, hanya saat line scanner yang ada gagal menemukan tiga finder, meniru fallback Otsu -> Sauvola (doc 14). Simbol tegak mempertahankan kecepatannya sekarang.
- Menjaga **zero runtime dependency**: detektor adalah pass connected-component pure-Go; fixture test tilt di-generate oleh warp gambar in-tree, tanpa OpenCV atau `x/image`.
- **Jujur soal batas.** CV itu empiris: milestone melebarkan pita kerja dan mendokumentasikan rentang sudut yang benar-benar dilewatinya pada fixture sintetis, alih-alih mengklaim robustness universal.

## 2. Kenapa detektor sekarang gagal, dan perbaikannya

`findFinders` sekarang menyusuri baris gambar (dan cross-check kolom) untuk lima run yang cocok dengan rasio **1:1:3:1:1** (`fitsFinderRatio`, +-50% per modul). Finder pattern adalah persegi konsentris, jadi garis lurus lewat pusatnya menyilang rasio itu pada *rotasi* apa pun — itulah kenapa v0.4 sudah menangani rotasi sampai ~30 derajat. Yang mematahkannya adalah **foreshortening perspektif**: lewat ~30 derajat tilt, panjang persilangan tertekan tak merata, rasio terukur melenceng keluar pita toleransi, dan tak ada run lima yang lolos. Melebarkan toleransi akan membanjiri detektor dengan false positive.

Perbaikan yang robust adalah berhenti mengukur rasio 1-D dan sebaliknya mendeteksi **struktur topologis** finder, yang dipertahankan sebuah projective transform. Finder adalah pusat gelap solid di dalam ring terang di dalam ring gelap — tiga region bersarang dengan pusat konsentris dan rasio area mendekati 3x3 : 5x5 : 7x7. Nesting dan konsentrisitas bertahan di homography mana pun, jadi analisis **connected-component** menemukan finder pada sudut apa pun:

1. Label connected component gelap dari gambar terbinarisasi (union-find dua-pass).
2. Untuk tiap component gelap, temukan component terang yang dikelilinginya dan component gelap yang terkurung di dalamnya (hierarki component / lubang).
3. Terima triple sebagai finder candidate saat ketiga centroid berimpit (dalam toleransi) dan area bersarangnya cocok dengan struktur 7:5:3 dalam toleransi.
4. Kembalikan pusat candidate, yang menyuap pipeline `orderFinderTriple` -> `estimateVersion` -> `homographyFromFinders` -> `refineHomography` -> `sampleMatrix` yang ada, persis seperti output line scanner sekarang.

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

### DR3 — Detektor Finder Connected-Component `(L)`

Tujuan: detektornya sendiri, diuji pada bitmap yang dikonstruksi.

- [ ] Connected-component labelling dua-pass atas bitmap terbinarisasi (union-find), menghasilkan bounding box, area, centroid, dan warna per-component.
- [ ] Analisis nesting: mencocokkan triple konsentris gelap -> terang -> gelap dengan rasio area 7:5:3-ish menjadi finder candidate; dedupe candidate nyaris-duplikat.
- [ ] Unit test pada bitmap buatan-tangan: finder pattern sintetis terdeteksi di pusat sejatinya; persegi polos dan noise acak ditolak; tiga finder dalam satu frame menghasilkan tiga candidate.

### DR4 — Fallback Pipeline + Round-Trip Tilt `(M)`

Tujuan: men-decode simbol miring nyata end-to-end.

- [ ] `decodeImageDebug` memanggil detektor component saat line scanner gagal, lalu lanjut lewat pipeline geometri yang tak berubah; debug state merekam detektor mana yang dipakai.
- [ ] Warp tilt in-tree (test helper) me-render QR dan memproyeksikannya pada rotasi/tilt tertentu; `TestArbitraryAngleRoundTrip` men-decode sebaran sudut dan merekam pita yang lolos. Jalur cepat tegak tetap byte-for-byte pada perilaku sekarang (test decode yang ada tak tersentuh).

### Checkpoint A — Decode publik membaca tilt sintetis lintas pita target.

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
├── decode_finder_cc.go        # baru — detektor finder connected-component
├── decode_finder_cc_test.go   # baru — unit test CCL + nesting
├── decode_image.go            # wiring fallback findFinders; catatan debug state
├── decode_tilt_test.go        # baru — warp tilt in-tree + round-trip sudut-sembarang
docs/
├── plan-decoder-robustness.md      # file ini
├── plan-decoder-robustness.id.md   # padanan Indonesia
└── theory/
    ├── 24-arbitrary-angle-detection.md     # baru
    └── 24-arbitrary-angle-detection.id.md  # baru
```

## 6. Risiko & Catatan Teknis

- **Ini mungkin tak sepenuhnya tertutup.** Tak seperti fitur spec, detektor CV tak punya success rate terjamin. Rencana memitigasi dengan (a) menjaganya sebagai fallback sehingga tak ada yang regresi, dan (b) memvalidasi pada fixture sintetis dan melaporkan pita sebenarnya alih-alih klaim. Kalau detektor component melewati, katakanlah, 30..70 derajat tetapi tidak 70..90, itu hasil terdokumentasi dan tetap peningkatan nyata.
- **False positive.** Melonggarkan deteksi finder berisiko menerima non-finder. Test nesting + konsentrisitas + rasio-area lebih ketat dari rasio 1-D, dan tiga finder yang saling konsisten (via `orderFinderTriple` dan `estimateVersion`) harus setuju, jadi candidate nyasar tersaring di hilir.
- **Fallback menjaga jalur cepat.** Pass component (CCL atas gambar) lebih mahal dari beberapa row scan, jadi ia jalan hanya saat line scanner gagal. Simbol tegak dan miring-ringan tak pernah membayarnya, dan test decode serta timing yang ada tak terpengaruh.
- **Homography sudah menangani perspektif.** Milestone sengaja tak menyentuh `homographyFromFinders`/`refineHomography`; keduanya sudah men-fit projective transform 3x3 sembarang dari pusat finder, jadi begitu pusatnya ditemukan pada sudut curam, sisanya tinggal jalan. Refinement alignment-pattern (v0.4) semakin mengoreksi perspektif.
- **Warp test-fixture bukan produk.** Warp tilt hidup hanya di test; ia memfabrikasi input untuk membuktikan detektor, dan kualitasnya sendiri (sampling nearest vs bilinear) hanya memengaruhi seberapa sulit gambar test, bukan library.

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
