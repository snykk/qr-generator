# Decoding pada Sudut Sembarang

Decoder v0.13 membaca simbol QR yang difoto dari sudut curam -- pita kemiringan tempat pipeline sebelumnya gagal. Untuk sampai ke sana, kuncinya adalah menemukan tahap yang benar-benar rusak di bawah perspektif, dan ternyata bukan tahap yang diasumsikan roadmap. Dokumen ini mencatat diagnosis yang sebenarnya, solusi projektif yang dibangun di atasnya, dan -- secara jujur -- sejauh mana ia menjangkau.

> Versi bahasa Inggris: [24-arbitrary-angle-detection.md](24-arbitrary-angle-detection.md).

## 1. Apa yang sebenarnya rusak saat miring

Dugaan awal adalah deteksi finder yang gagal lebih dulu: line scanner 1:1:3:1:1 (dokumen 12, 15) mengukur rasio run satu dimensi, dan foreshortening perspektif semestinya mendistorsi rasio itu melewati toleransinya. Diuji terhadap fixture kemiringan sintetis, dugaan itu keliru dalam dua hal.

Pertama, line scanner lebih tangguh dari perkiraan. Toleransi per-modulnya +-50%, cukup longgar sehingga ia tetap menemukan ketiga finder hingga sekitar 55 derajat kemiringan. Jadi deteksi finder tak pernah menjadi yang pertama gagal.

Kedua, dan menentukan: **bahkan ketika finder sudah ditemukan, decode gagal mulai sekitar 20 derajat.** Penyebabnya tahap geometri. [homographyFromFinders](../../qrgen/decode_image.go) membangun transformasi image-ke-modul dari tiga pusat finder plus sudut keempat hasil melengkapi parallelogram -- sebuah map **afin**. Map afin mempertahankan keparalelan, jadi ia tak bisa merepresentasikan keystoning yang dihasilkan bidang miring: sisi jauh simbol tercitra lebih kecil daripada sisi dekat, dan transformasi afin tidak punya suku untuk itu. Refinement alignment pattern (v0.4) semestinya memasok titik keempat yang sejati, tetapi di bawah kemiringan prediksi afin atas pusat alignment jatuh di luar window pencarian yang kecil, sehingga refinement diam-diam kembali ke estimasi afin. Dan ada batas yang lebih halus: **centroid** sebuah finder bukan invarian projektif -- map projektif mengirim pusat sebuah persegi ke titik yang bukan centroid dari quad tercitra -- sehingga bahkan tiga korespondensi pusat pun membawa error yang tumbuh seiring sudut.

Sebuah eksperimen kontrol menyelesaikannya. Menyampel fixture miring yang sama persis dengan homografi projektif *ground-truth* mendekode bersih sampai 70 derajat. Simbolnya sepenuhnya terbaca pada sudut curam; yang kurang hanyalah transformasi yang bisa membalik perspektif. Itulah inti keseluruhan milestone ini.

## 2. Solusinya: homografi projektif dari sudut finder

Homografi projektif (3x3 penuh dengan baris bawah non-trivial) persis merupakan transformasi antara sebuah bidang dan citra perspektifnya, jadi ia bisa membalik keystoning. Ia punya delapan derajat kebebasan dan butuh minimal empat korespondensi titik yang tidak projektif-degenerate.

Tiga pusat finder hanya memberi tiga titik -- dan tak sempurna, sesuai bagian 1. Tetapi tiap finder adalah **persegi**, dan persegi punya empat sudut. Tiga finder memberi dua belas korespondensi sudut, tersebar di seluruh simbol, yang over-determine homografi dan mengunci perspektifnya. Ini basis standar yang dipakai decoder sungguhan (ZXing, OpenCV). Jadi tugas milestone ini adalah menemukan finder, mengekstrak sudutnya, lalu menyelesaikannya.

## 3. Menemukan finder: connected component

Sudut berasal dari region finder, jadi langkah pertama adalah menyegmentasinya. Detektor connected-component melabeli citra terbinarisasi dengan two-pass union-find (4-konektivitas) dan mencocokkan penyarangan topologis finder -- cincin gelap berongga yang kira-kira persegi, membungkus cincin terang konsentris, membungkus pusat gelap, dengan sisi bounding box dalam proporsi 7:5:3. Penyarangan dan konsentrisitas bertahan pada homografi apa pun, jadi ini menemukan finder pada kemiringan yang sama seperti yang dijangkau line scanner (secara empiris ~55 derajat; di luar itu keduanya gagal dan tak ada tahap berikutnya yang bisa menolong). Labelling juga mengembalikan peta komponen per-piksel, sehingga piksel-piksel cincin luar bisa dikunjungi ulang untuk ekstraksi sudut.

## 4. Mengekstrak empat sudut

Untuk komponen cincin-luar tiap finder, sudut-sudutnya adalah vertex ekstrem dari sebuah quad cembung. Cara yang agnostik terhadap rotasi untuk menemukannya: ambil titik terjauh dari centroid cincin (satu sudut), lalu titik terjauh dari *titik itu* (sudut seberang), lalu dua titik yang terletak paling jauh ke masing-masing sisi garis yang menghubungkan keduanya. Ini bekerja pada orientasi apa pun, tak seperti mengambil ekstrem sejajar-sumbu dari `x+y` dan `x-y`, yang menurun kualitasnya menjelang 45 derajat rotasi.

## 5. Menetapkan koordinat modul untuk tiap sudut

Solver homografi butuh tiap sudut dipasangkan dengan posisinya di grid modul simbol. Keempat sudut keluar dalam urutan sembarang, jadi masing-masing diklasifikasikan dengan memproyeksikan offset-nya dari pusat finder ke sumbu-sumbu simbol sendiri: arah top-left ke top-right dan arah top-left ke bottom-left, diambil dari triple finder yang sudah terurut. Tanda kedua proyeksi menempatkan sudut itu pada salah satu dari empat sudut luar finder, yang berada 3,5 modul dari pusat finder pada tiap sumbu. Sudut finder top-left memetakan ke koordinat modul sekitar (3, 3); top-right sekitar (n-4, 3); bottom-left sekitar (3, n-4).

## 6. Menyelesaikan: least squares atas dua belas titik

Dengan lebih dari empat korespondensi, sistemnya over-determined, jadi diselesaikan dalam arti total-least-squares lewat normal equations `(A^T A) h = A^T b` dengan normalisasi h22 = 1, memakai ulang solver Gaussian 8x8 yang sudah ada. Merata-ratakan atas dua belas sudut menyerap noise ekstraksi per-sudut yang akan diwarisi solve empat-titik mana pun.

## 7. Version di bawah perspektif, dan trial decode

Satu masalah tersisa: ukuran grid. Sudut-sudut jauh homografi ditempatkan di modul (n-4, ...), jadi transformasi itu sendiri butuh version n, dan grid modul yang disampel darinya berukuran n kali n. Namun version dibaca dari jarak antar-finder dalam satuan modul, dan di bawah kemiringan jarak itu foreshorten sementara estimasi pitch modul dari bounding box finder yang berotasi justru membengkak -- sehingga estimasi biasa jatuh di luar rentang.

Perbaikannya dua bagian. *Seed* yang lebih baik: estimasi pitch modul dari panjang sisi luar sebuah quad finder (tiap sisi 7 modul), bukan dari bounding box sejajar-sumbunya, yang tidak dibengkakkan rotasi. Dan sebuah *trial*: karena seed masih bisa meleset beberapa version di bawah perspektif kuat, decoder mencoba kandidat version berurutan menurut kedekatan dengan seed, memasang homografi dan mencoba decode untuk masing-masing. Validasi Reed-Solomon dan format information membuat version yang salah gagal cepat dan version yang benar mendekode, jadi keberhasilan pertama adalah jawabannya. Version yang benar biasanya adalah seed atau salah satu tetangga terdekatnya, sehingga trial cepat selesai.

## 8. Sebuah fallback, agar tak ada yang regresi

Pipeline projektif ketat merupakan percobaan kedua. `decodeImage` pertama menjalankan pipeline afin yang tak diubah -- line scanner cepat dan homografi berbasis pusat -- yang optimal untuk simbol tegak dan berotasi serta menangani setiap kasus yang ditangani decoder sebelumnya. Hanya ketika itu gagal menghasilkan grid yang bisa didekode, tanda khas kemiringan perspektif, ia jatuh ke pipeline projektif connected-component. Simbol tegak tak pernah membayar pass komponen atau trial version, decode yang ada tetap byte-for-byte tak berubah, dan error dari percobaan afin adalah yang dimunculkan saat gagal total sehingga pesan error stabil.

## 9. Validasi, dan batas yang jujur

Tidak ada tabel spec untuk mengecek decoder projektif, jadi ia divalidasi secara empiris. Sebuah warp kemiringan khusus-tes merender simbol tegak dan memproyeksikannya lewat homografi rotasi-plus-kemiringan pilihan (tiap piksel tujuan dipetakan balik dan disampel bilinear), menghasilkan foto sintetis pada rentang sudut. Tes round-trip mendekodenya dan meng-assert pita yang bertahan.

Hasil yang jujur: lintas orientasi, decoder andal membaca kemiringan sampai kira-kira 35 hingga 40 derajat, dan sampai sekitar 55 derajat pada rotasi in-plane yang menguntungkan; batas atas tepatnya bergantung pada orientasi karena ekstraksi sudut dan seed bounding box menurun saat persegi finder foreshorten. Di luar ~55 hingga 60 derajat finder sendiri berhenti terdeteksi dan tak ada tahap berikutnya yang bisa memulihkan. Blur berat, silau yang merusak penyarangan finder, lipatan, dan sudut nyaris menyerempet tetap gagal. Karena seluruh jalur ini fallback, tak satu pun dari itu meregresikan kasus tegak dan berotasi; ia hanya menambah decode yang sebelumnya gagal total.

## 10. Penunjuk implementasi

- `qrgen/decode_finder_cc.go`: labelling two-pass union-find, pencocok penyarangan, ekstraksi sudut (`extractQuadCorners`, `farthestFrom`, `farthestAcross`), dan `bestFinderQuadTriple`, yang mengurutkan quad finder lewat `orderFinderTriple` yang invarian rotasi.
- `qrgen/decode_image.go`: `homographyFromFinderQuads` (penetapan koordinat-modul sudut) dan `computeHomographyLS` (solve least-squares); `decodeOneQuadTriple` (trial version), yang dicapai lewat `decodeImageCC` dan `decodeCCFromBitmap`, yang sejak v0.16 juga menjalankan pencarian Micro dari dokumen 27; serta `decodeImage`, yang mengurutkan percobaan afin lalu projektif.
- `qrgen/decode_tilt_test.go`: warp kemiringan khusus-tes dan round-trip sudut-sembarang, plus penjaga regresi tegak-pakai-afin.

## Referensi

- ISO/IEC 18004:2015 -- struktur modul konsentris 7x7 / 5x5 / 3x3 pola finder dan quiet zone yang mengisolasinya.
- `docs/theory/12-image-processing.md` -- binarisasi dan pipeline finder afin yang menjadi tempat milestone ini melakukan fallback.
- `docs/theory/14-adaptive-thresholding.md` -- pola dispatch coba-lalu-fallback yang dipakai ulang di sini.
- `docs/theory/15-rotation-handling.md` -- pengurutan finder yang invarian rotasi, dipakai ulang tanpa perubahan untuk triple quad.
- Direct Linear Transform untuk estimasi homografi, dan analisis connected-component / kontur untuk lokasi finder, adalah computer vision standar; detektor QR ZXing dan OpenCV memakai basis sudut-plus-homografi yang sama sebagai prior art konseptual (bukan dependensi).
