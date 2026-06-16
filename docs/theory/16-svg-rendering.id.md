# Rendering SVG

Renderer v0.5 menambahkan output **vektor yang scalable** di samping raster PNG awal. Jika `renderPNG` menulis grid pixel berukuran tetap, `renderSVG` menulis dokumen teks yang mendeskripsikan simbol secara geometris, sehingga ia tetap tajam di zoom berapa pun dan kecil di disk untuk payload umum. Dokumen ini mencatat model dokumen SVG yang dipakai untuk simbol QR, pendekatan menggambar path-data, sistem koordinat, penanganan warna, dan kenapa renderer-nya berupa sibling function alih-alih sesuatu yang disembunyikan di balik interface.

> Versi Inggris: [16-svg-rendering.md](16-svg-rendering.md).

## 1. Kenapa SVG

- **Scaling lossless.** Simbol QR adalah geometri murni — persegi-persegi di atas grid. Raster mengunci geometri itu pada satu resolusi; SVG mendeskripsikannya sekali dan membiarkan viewer men-scale-nya ke ukuran apa pun tanpa blur dan tanpa artefak resampling. Pipeline cetak dan display high-DPI mendapat simbol yang sempurna di dimensi berapa pun.
- **File kecil.** Untuk mayoritas payload deskripsi vektor lebih kecil dari PNG ekuivalen, dan ia gzip-compress dengan baik karena path data sangat repetitif.
- **Embeddability.** SVG masuk langsung ke HTML, dapat di-style atau di-theme oleh dokumen host, dan dipahami setiap tool desain.
- **Tepi tajam.** Dengan satu instruksi renderer dapat memberi tahu viewer untuk tidak meng-anti-alias batas modul, yang menjaga simbol tetap decodable (lihat bagian 5).

## 2. Model dokumen SVG untuk simbol QR

Simbol QR yang ter-render adalah tiga hal yang ditumpuk: sebuah kanvas, sebuah background terang, dan modul-modul gelap. Di SVG itu memetakan ke:

```text
<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg"
     width="W" height="H"
     viewBox="0 0 D D"
     shape-rendering="crispEdges">
  <rect width="D" height="D" fill="#FFFFFF"/>     <!-- background termasuk quiet zone -->
  <path d="M3 3h1v1h-1z M5 3h1v1h-1z ..." fill="#000000"/>  <!-- setiap modul gelap -->
</svg>
```

Root `<svg>` membawa namespace SVG, ukuran pixel nominal di `width`/`height`, kotak koordinat logis di `viewBox`, dan hint crisp-edges. Satu `<rect>` mengecat seluruh kanvas — termasuk quiet zone — dalam warna background. Satu `<path>` mengecat setiap modul gelap dalam warna foreground. Itu seluruh dokumennya; tidak ada elemen per-modul.

## 3. Path data: satu path, bukan banyak rect

Cara naif menggambar modul gelap adalah satu `<rect>` per modul. Simbol V1 punya hingga beberapa ratus modul gelap; V40 punya puluhan ribu. Ratusan atau ribuan elemen `<rect>` membengkakkan file dan memperlambat parser.

Sebagai gantinya kita meng-emit satu `<path>` yang atribut `d`-nya berupa urutan subpath tertutup kecil, satu persegi per modul gelap:

```text
M x y h 1 v 1 h -1 z
```

dibaca sebagai: **m**ove ke pojok kiri-atas modul `(x, y)`, gambar garis **h**orizontal satu unit ke kanan, garis **v**ertikal satu unit ke bawah, garis **h**orizontal satu unit ke kiri, lalu **z** tutup perseginya kembali. Menggabungkan satu subpath seperti itu per modul gelap menghasilkan satu elemen `<path>` yang mengisi semuanya dalam satu operasi `fill`. Ini pendekatan yang dipakai `toSvgString` milik Project Nayuki dan merupakan standar de-facto untuk output QR SVG.

Optimisasi lebih lanjut menggabungkan run modul gelap yang bersebelahan horizontal menjadi satu rectangle yang lebih lebar (`h 3` alih-alih tiga persegi `h 1` terpisah), mengecilkan path lagi. v0.5 merilis bentuk per-modul yang sederhana demi kejelasan dan kebenaran; run-length merging adalah penyetelan fokus-ukuran masa depan.

## 4. Sistem koordinat: unit modul yang scale

`viewBox` dinyatakan dalam **unit modul**, bukan pixel: `viewBox="0 0 D D"` di mana `D = n + 2q` (sisi matrix `n` plus quiet zone `q` modul di tiap sisi). Di dalam kotak itu, modul gelap `(row, col)` adalah unit persegi di `(col + q, row + q)`. Bekerja dalam unit modul menjaga path data tetap kecil dan bernilai integer, dan — karena `viewBox` mendefinisikan sistem koordinat logis yang independen dari ukuran ter-render — simbol scale ke dimensi fisik apa pun dengan bersih.

Ukuran ter-render nominal lalu diatur oleh `width` dan `height`, keduanya default ke `moduleSize * D` pixel. Ini membuat SVG dan PNG yang dihasilkan dengan option yang sama mendeskripsikan dimensi nominal yang sama: simbol V1 dengan `WithModuleSize(8)` lebarnya `8 * (21 + 8) = 232` px di kedua format. Viewer bebas meng-override ukuran ter-render — simbol tetap memetakan dengan bersih karena `viewBox` membawa proporsi yang sebenarnya.

## 5. crispEdges dan decodability

Viewer SVG meng-anti-alias secara default: tepi shape yang jatuh di antara dua device pixel di-blend menjadi abu-abu perantara supaya terlihat halus. Untuk simbol QR itu merugikan — decoder men-sample pusat tiap modul dan men-threshold gelap vs terang, dan batas yang buram mendorong pixel tepi mendekati threshold, mengikis margin yang diandalkan binariser. Atribut `shape-rendering="crispEdges"` pada root `<svg>` memberi tahu viewer untuk menonaktifkan anti-aliasing dan men-snap tepi ke grid pixel, mempertahankan transisi hitam/putih tegas yang diasumsikan decoding QR. Ini mencerminkan pilihan sengaja renderer PNG untuk menggambar modul sebagai persegi tajam tanpa anti-aliasing (doc 08).

Trade-off-nya adalah pada faktor scale non-integer `crispEdges` dapat membuat lebar modul sedikit tidak rata (sebagian modul satu pixel lebih lebar dari yang lain saat rasteriser membulatkan). Ketidakrataan itu jauh kurang merusak bagi decoding dibanding blur fringe abu-abu yang digantikannya, jadi ia default yang tepat; `viewBox` unit-modul tetap dipertahankan apa pun yang terjadi karena scalability adalah inti dari memilih SVG.

## 6. Penanganan warna

`WithColors` menerima `color.Color` apa pun. `color.Color.RGBA()` milik Go mengembalikan empat channel 16-bit, alpha-premultiplied, dalam rentang `[0, 0xFFFF]`. Untuk meng-emit string hex gaya-CSS `#RRGGBB` kita turunkan tiap channel ke 8-bit dengan membagi dengan `0x101` (= 65535/255) alih-alih shift kanan 8, yang membulatkan dengan benar di seluruh rentang alih-alih memotong.

Alpha butuh kehati-hatian. SVG 1.1 tidak menerima hex `#RRGGBBAA` 8-digit secara portabel, jadi ketika sebuah warna tidak fully opaque renderer meng-emit atribut `fill-opacity` terpisah yang membawa alpha fraksional (`alpha / 0xFFFF`). Warna yang fully opaque — kasus yang sangat umum, termasuk default hitam-di-putih — menghilangkan `fill-opacity` sepenuhnya supaya dokumen tetap minimal. Karena `RGBA()` premultiplied, channel hex dibagi balik dengan alpha sebelum emisi untuk memulihkan warna sebenarnya yang caller berikan.

## 7. Sibling function, bukan interface

Versi lama doc 08 mengklaim bahwa "format lain dapat ditambahkan kemudian di balik `Render` interface yang sama." Interface seperti itu tidak pernah ada, dan v0.5 secara sengaja tidak menambahnya. Ada persis dua renderer, `renderPNG` dan `renderSVG`, keduanya berbagi signature identik `func(m *matrix, opts renderOptions) ([]byte, error)`, dan tidak satu pun pernah dipilih lewat variabel saat runtime — `Encode` memanggil `renderPNG` langsung dan `EncodeSVG` memanggil `renderSVG` langsung. Interface akan menambah abstraksi dengan persis satu dispatch site per implementasi dan tanpa caller polimorfik, yang justru situasi di mana YAGNI berlaku. Ini penalaran yang sama yang menjaga dispatch Sauvola v0.3 sebagai `if` lurus alih-alih strategy interface (doc 14, bagian 7). Doc 08 sudah dikoreksi untuk mendeskripsikan sibling render function yang berbagi `renderOptions` alih-alih interface fiktif.

Memilih format lewat fungsi mana yang Anda panggil — `Encode` untuk PNG bytes, `EncodeSVG` untuk SVG bytes — juga menjaga kontrak return tiap fungsi tetap jelas, yang akan dikaburkan oleh satu `Encode` plus enum `WithFormat` (byte slice-nya berisi apa? bergantung pada option tersembunyi).

## 8. Pointer Implementasi

- `qrgen/render_svg.go` menampung `renderSVG(m *matrix, opts renderOptions) ([]byte, error)` dan helper `colorToHex`. Ia memakai ulang struct `renderOptions` yang ada dan method `withDefaults`-nya tanpa perubahan.
- Emitter membangun dokumen dengan `strings.Builder` dan `fmt`; tidak ada dependency library XML karena strukturnya tetap dan semua nilai atribut berupa angka atau string warna yang terkontrol.
- Tes mem-parse output dengan `encoding/xml` untuk menjamin well-formedness, menghitung command `M` di path untuk mengonfirmasi total modul gelap cocok dengan matrix, dan mengecek `viewBox` / `width` / `height` terhadap matematika option.
- `EncodeSVG` / `EncodeSVGToFile` di `qrgen/api.go` menjalankan paruh-depan `resolveOptions -> validate -> buildMatrix` yang sama dengan `Encode`, lalu memanggil `renderSVG` alih-alih `renderPNG`.

## Referensi

- W3C — *Scalable Vector Graphics (SVG) 1.1 (Second Edition)*: <https://www.w3.org/TR/SVG11/>. Grammar path data (bagian 8.3), `shape-rendering` (bagian 11.2), basic shapes dan elemen `rect`.
- Project Nayuki — *QR Code generator library*: `toSvgString`-nya me-render seluruh simbol sebagai satu path, pendekatan yang diadopsi di sini. <https://www.nayuki.io/page/qr-code-generator-library>
- `docs/theory/08-rendering.md` — catatan rendering PNG; keputusan color-model dan crisp-edges di sana terbawa ke SVG, dan kalimat `Render`-interface-nya dikoreksi sebagai bagian dari milestone ini.
- Standard library Go — `image/color` (semantik `color.Color.RGBA`) dan `encoding/xml` (dipakai oleh tes).
