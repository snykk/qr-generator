# Deteksi Finder pada Sudut Sembarang

Decoder v0.13 menemukan simbol QR yang miring atau berputar jauh melewati pita ~30 derajat yang mampu ditangani detektor finder berbasis pemindaian baris. Caranya adalah memperbaiki satu-satunya tahap yang gagal pada sudut curam — menemukan ketiga pola finder — dengan detektor connected-component yang invarian terhadap transformasi projektif apa pun, lalu menyerahkan pusat finder ke pipeline homografi yang sudah ada tanpa perubahan. Dokumen ini mencatat mengapa pemindai baris gagal, mengapa pendekatan topologis bertahan, bagaimana detektor komponen bekerja, dan — secara jujur — sejauh mana ia benar-benar menjangkau.

> Versi bahasa Inggris: [24-arbitrary-angle-detection.md](24-arbitrary-angle-detection.md).

## 1. Rotasi tidak masalah; perspektif itulah masalahnya

Detektor yang ada memindai baris citra untuk mencari lima run gelap/terang/gelap/terang/gelap yang panjangnya cocok dengan rasio **1:1:3:1:1**, lalu memeriksa silang kolomnya (dokumen 12, 15). Pola finder adalah kotak-kotak konsentris, jadi garis lurus yang melewati pusatnya menyeberangi rasio itu pada *rotasi in-plane* berapa pun — persis alasan v0.4 sudah bisa mendekode simbol yang berputar hingga ~30 derajat dari sumbu tanpa penanganan khusus.

Yang mengalahkannya bukan rotasi melainkan **pemendekan perspektif (foreshortening)**. Ketika simbol difoto dari kemiringan, proyeksi memampatkan jarak secara tidak merata di sepanjang finder: sisi jauh dari kotak 7-modul tercitra lebih kecil daripada sisi dekatnya, sehingga kelima panjang penyeberangan tidak lagi terbaca 1:1:3:1:1. Melewati kira-kira 30 derajat kemiringan, rasio terukur bergeser keluar dari toleransi `fitsFinderRatio` sebesar +-50% per modul dan tidak ada run lima yang lolos. Melebarkan toleransi bukan solusi: itu akan menerima tekstur gelap-terang-gelap biasa sebagai finder dan membanjiri detektor dengan false positive.

## 2. Wawasan topologis

Transformasi projektif dapat merentang, menggeser (shear), dan memendekkan sebuah bentuk, tetapi ia tidak dapat mengubah apa yang berada *di dalam* apa. Pola finder adalah tiga region bersarang — pusat gelap padat, di dalam cincin terang, di dalam cincin gelap — dan penyarangan itu terjaga pada sudut kamera berapa pun. Jadi alih-alih mengukur rasio satu dimensi, deteksilah **strukturnya**:

- sebuah region gelap,
- membungkus region terang,
- membungkus region gelap,
- dengan ketiga pusat kira-kira berhimpit, dan
- dengan luas dalam proporsi kira-kira `3x3 : 5x5 : 7x7` yang ditentukan modul-modul finder.

Penyarangan dan konsentrisitas adalah invarian topologis dan semi-afin; keduanya berlaku pada 10 derajat maupun 80. Mendeteksinya menuntut segmentasi citra menjadi region-region, yang persis dilakukan connected-component labelling.

## 3. Connected-component labelling

Detektor menjalankan labelling **dua-lintasan union-find** standar atas bitmap terbinarisasi:

1. **Lintasan pertama**, memindai dalam urutan baca: beri setiap piksel label sementara dari tetangga atas dan kirinya yang sudah berlabel dan sewarna (gelap atau terang), dan ketika kedua tetangga itu membawa label berbeda, union-kan keduanya dalam struktur disjoint-set.
2. **Lintasan kedua**: ganti setiap label sementara dengan representatif himpunannya, sehingga setiap region sewarna maksimal punya satu label.

Untuk tiap komponen berlabel, detektor mengumpulkan bounding box, luas piksel, dan sentroid. Ini O(piksel) dengan union-find nyaris linear, dijalankan hanya pada jalur fallback (bagian 5), jadi tidak memperlambat kasus umum.

## 4. Tanda tangan finder

Dengan komponen di tangan, detektor mencari triple bersarang. Cincin gelap luar finder adalah komponen gelap; cincin terang yang dibungkusnya adalah komponen terang yang sentroidnya duduk di tempat yang sama; pusat gelap adalah komponen gelap di dalamnya. Detektor menerima sebuah kandidat ketika:

- ketiga sentroid berhimpit dalam fraksi kecil dari box luar (konsentrisitas), dan
- luas bersarang menurun dalam proporsi luas-modul finder kira-kira `49 : 25 : 9`, dalam toleransi (rasio tak perlu tepat — perspektif mengubah luas absolut tetapi *urutan* dan proporsi kasarnya bertahan), dan
- box luar kira-kira mendekati persegi (blob yang sangat memanjang bukan finder bahkan di bawah kemiringan, dalam batas wajar).

Tiap kandidat yang lolos menyumbang pusatnya. Tiga pusat yang saling konsisten kemudian mengalir ke `orderFinderTriple` (invarian rotasi lewat uji handedness cross-product, dokumen 15), `estimateVersion`, `homographyFromFinders`, dan `refineHomography` yang tidak diubah, yang memasang transformasi projektif 3x3 penuh — jadi begitu ketiga pusat ditemukan pada sudut curam, sisa proses dekode langsung jalan. Kandidat bukan-finder yang nyasar tersaring di sini: ia tak akan membentuk triple yang konsisten secara geometris dengan dua finder asli.

## 5. Sebuah fallback, bukan pengganti

Lintasan komponen memakan segmentasi O(piksel) penuh, lebih mahal daripada segelintir pemindaian baris, jadi ia berjalan **hanya ketika pemindai baris gagal mengembalikan tiga finder** — bentuk yang sama dengan fallback Otsu -> Sauvola (dokumen 14). Simbol yang tegak dan sedikit miring ditemukan oleh pemindai cepat dan tak pernah membayar lintasan komponen; yang miring curam jatuh ke sana. Ini menjaga kasus umum pada kecepatannya sekarang dan menjamin tanpa regresi: jika pemindai baris masih berhasil, hasilnya dipakai persis seperti sebelumnya.

## 6. Validasi, dan batas yang jujur

Tidak ada tabel spec untuk mengecek detektor CV, jadi ia divalidasi secara empiris. Sebuah **warp kemiringan** khusus-tes merender QR tegak dan memproyeksikannya lewat homografi rotasi-plus-kemiringan pilihan (tiap piksel tujuan dipetakan balik dan disampel dari sumbernya), menghasilkan foto sintetis pada 30, 45, 60, dan derajat lebih curam. Tes round-trip mendekode masing-masing dan mencatat pita yang lolos; README dan dokumen ini menyatakan pita yang benar-benar dilewati, bukan sekadar aspirasi.

Harapan yang jujur: ini melebarkan pita andal secara substansial melewati ~30 derajat, tetapi ini adalah computer vision usaha-terbaik (best-effort). Sudut mendekati 90 derajat yang menyerempet (ketika simbol hampir tegak lurus pandang dan finder menyusut jadi serpihan tipis), blur berat, atau silau yang merusak penyarangan finder masih bisa gagal. Karena detektor ini sebuah fallback, tak satu pun dari itu meregresikan kasus tegak dan sedikit miring; ia hanya menambah dekode yang sebelumnya gagal total.

## 7. Penunjuk implementasi

- `qrgen/decode_finder_cc.go` menampung labelling dua-lintasan union-find, akumulasi per-komponen, dan pencocok penyarangan/konsentrisitas/rasio-luas yang memancarkan pusat `finderCandidate`.
- `decodeImageDebug` di `qrgen/decode_image.go` memanggilnya sebagai fallback setelah pemindai baris tak mengembalikan triple, lalu berlanjut lewat pipeline geometri yang tak diubah; state debug mencatat detektor mana yang berhasil, yang di-assert oleh tes.
- Warp kemiringan dan round-trip sudut-sembarang berada di `qrgen/decode_tilt_test.go`; warp itu perancah khusus-tes, tak pernah menjadi bagian pustaka.

## Referensi

- ISO/IEC 18004:2015 — struktur modul konsentris 7x7 / 5x5 / 3x3 pola finder dan quiet zone yang memisahkannya dari data.
- `docs/theory/12-image-processing.md` — binarisasi dan pemindaian finder baris/kolom yang menjadi tempat milestone ini melakukan fallback.
- `docs/theory/14-adaptive-thresholding.md` — pola dispatch coba-lalu-fallback yang dipakai ulang di sini.
- `docs/theory/15-rotation-handling.md` — pengurutan finder yang invarian rotasi dan batas cakupan ~30 derajat yang diperluas milestone ini.
- Connected-component labelling (dua-lintasan, union-find) — algoritma segmentasi citra standar, diimplementasikan dalam Go murni; detektor QR ZXing dan OpenCV memakai analisis komponen/kontur sebagai prior art konseptual.
