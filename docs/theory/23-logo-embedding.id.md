# Logo Embedding

Para renderer v0.12 bisa menaruh logo di tengah simbol QR — tampilan branded-QR yang familiar — sambil menjaganya tetap bisa dipindai. Tak ada yang berubah pada encoding: logo digambar ke raster jadi (PNG) atau disisipkan ke dokumen (SVG), dan simbol tetap decode karena error correction Reed–Solomon memulihkan modul yang ditutupi logo. Dokumen ini mencatat kenapa itu berhasil, seberapa besar logo yang aman, bump error-correction otomatis, background pad, dan compositing zero-dependency.

> Versi Inggris: [23-logo-embedding.md](23-logo-embedding.md).

## 1. Kenapa logo di tengah tetap terpindai

Logo menutupi sebagian modul; bagi decoder modul itu salah. QR bertahan lewat **error correction Reed–Solomon**, yang memulihkan sebagian codeword di tiap block. Empat EC level memulihkan kira-kira L 7%, M 15%, Q 25%, dan H 30% dari data codeword. Logo yang menutupi sekitar seperlima simbol merusak sekitar seperlima modul, yang duduk di dalam budget H dengan margin — jadi logo di tengah yang wajar pada EC level H terbaca baik.

Dua fakta struktural membuatnya bekerja lebih baik dari sekadar persentase mentah:

- **Interleaving menyebar kerusakan.** Sebuah simbol QR meng-interleave block Reed–Solomon-nya ke seluruh matrix (doc 04), jadi satu tambalan kontigu di tengah tak menghapus satu block — ia mengambil irisan dari banyak block, dan koreksi milik tiap block menangani irisannya. Error correction bersifat per-block, jadi menyebar sejumlah kerusakan tetap ke semua block adalah kasus terbaik.
- **Bagian tengah adalah tempat aman.** Ketiga finder pattern ada di sudut dan timing pattern berjalan di baris dan kolom 6; logo di tengah tak menyentuh satu pun. Decoder butuh finder dan timing untuk melokasi dan men-sample grid, jadi logo harus menjauh darinya — yang otomatis dilakukan logo di tengah berukuran wajar.

## 2. Ukuran dan bump EC-H otomatis

Ukuran aman adalah pertukaran antara tampil bagus dan tetap di dalam budget recovery. Default di sini menutupi sekitar **20% lebar simbol**, jadi kira-kira 4% modul secara luas (kuadrat `0,2 x 0,2`) — margin lebar bahkan di dalam EC level terendah. Default-nya sengaja konservatif: EC-H toh dipaksa, jadi background pad, logo dunia-nyata yang lebih ramai, dan pemindaian yang tak sempurna semua tetap nyaman di dalam budget.

Karena budget itu hanya ada di EC level tinggi, meminta logo **memaksa error-correction level setidaknya H**. Ini terjadi di resolusi option, setelah semua option diterapkan, jadi order-independent dan pemanggil yang eksplisit meminta H (atau sudah di sana) tak terpengaruh. Pemanggil tetap bisa membuat logo terlalu besar — lewat sekitar seperempat lebar, simbol mulai gagal — jadi plafon ukuran didokumentasikan alih-alih dipaksa diam-diam; default-nya sengaja konservatif.

## 3. Background pad

Logo digambar di atas **background pad** kecil — persegi panjang putih sedikit membulat yang sedikit lebih besar dari logo. Ia melakukan dua hal: memberi logo quiet zone lokal yang bersih supaya terbaca sebagai elemen tersendiri alih-alih menyatu dengan modul di sekitarnya, dan mengubah region transparan logo menjadi putih alih-alih membiarkan modul gelap tembus dan membingungkan mata (decoder toh sudah memperlakukan seluruh tambalan sebagai kerusakan). Pad adalah bagian dari area tertutup, jadi ikut terhitung ke budget recovery; default menjaga pad-plus-logo di dalam pita aman.

## 4. Compositing PNG versus penyisipan SVG

Kedua renderer mencapai hasil sama lewat cara berbeda:

- **PNG** menggambar logo langsung ke image hasil render sebelum encoding. Persegi pad diisi, lalu logo — diskala ke box target (bagian 5) — dikomposit di atasnya dengan `image/draw` memakai `draw.Over`, jadi alpha milik logo membaur ke pad putih. Hasilnya satu raster dengan logo tertanam.
- **SVG** menyisipkan logo sebagai elemen `<image>` dengan `data:` URI base64, diposisikan dan diukur di atas `<rect>` background, setelah path modul. Karena gambar berupa data URI ketimbang `href` eksternal, SVG tetap satu file self-contained, konsisten dengan tujuan renderer v0.5 (doc 16). SVG menskala `<image>`-nya sendiri, jadi tak perlu resampling di jalur ini.

## 5. Resampling zero-dependency

Jalur PNG harus memuat logo sembarang ke box yang ukurannya bergantung dimensi pixel simbol, yang bergantung version, jadi library harus menskala gambar. `image/draw` standard library mengomposit tetapi tak bisa mengubah ukuran, dan `golang.org/x/image/draw` — yang bisa — adalah modul eksternal yang ditolak aturan zero-dependency. Jadi milestone mengirim resampler kecil in-tree:

- **Downscaling** (kasus umum, logo besar dikecilkan agar pas) memakai **box filter**: tiap pixel tujuan merata-ratakan pixel sumber yang memetakan ke dalamnya, yang menghindari aliasing kasar yang akan diproduksi nearest-neighbour pada foto atau ikon yang dikecilkan.
- **Upscaling** (logo kecil diperbesar) memakai **nearest-neighbour**, yang memadai dan murah.

Pemanggil yang butuh kualitas kelas-Lanczos bisa menskala logo sendiri lebih dulu dan mengopernya pada ukuran final-nya; resampler no-op saat sumber sudah cocok dengan box target.

## 6. Penunjuk implementasi

- `qrgen/logo.go` memuat config `Logo` (`Image`, `Scale`, `Background`, padding), `WithLogo`, resampler, dan compositing PNG (pad plus logo diskala). `renderOptions` mendapat field `logo` yang dialirkan `Encode`; `render_png.go` menggambar logo setelah modul, `render_svg.go` mengeluarkan data URI `<image>`.
- `resolveOptions` menaikkan EC level ke H saat ada logo dan level-nya lebih rendah, jadi budget recovery selalu ada.
- Semua dipagari `WithLogo`; encode tanpa-logo byte-for-byte identik, dijaga test dan golden fixture yang utuh.
- Test utama meng-encode teks dengan logo default dan memastikan `DecodeBytes` tetap mengembalikan teks asli — modul tertutup menjadi error Reed–Solomon yang dipulihkan koreksi — dengan cross-check gozxing bahwa decoder independen setuju.

## Referensi

- ISO/IEC 18004:2015 — struktur error-correction Reed–Solomon, interleaving block, dan empat EC level yang kapasitas recovery-nya dikonsumsi logo.
- `docs/theory/04-reed-solomon.md` — koreksi per-block dan interleaving yang membuat okusi di tengah bisa dipulihkan.
- `docs/theory/08-rendering.md` dan `docs/theory/16-svg-rendering.md` — renderer PNG dan SVG yang overlay logo perluas.
- Go standard library — `image`, `image/draw` (compositing `draw.Over`), `image/color`, dan `encoding/base64` (data URI SVG); sengaja bukan `golang.org/x/image`.
