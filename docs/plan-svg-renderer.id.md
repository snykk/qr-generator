# QR Encoder — Plan SVG Renderer

Dokumen ini menjelaskan rencana implementasi enhancement **SVG renderer** yang menargetkan rilis minor `v0.5.0`. Ini adalah penambahan format output pertama setelah PNG renderer awal (v0.1.0) dan membuka fase encoder/output-breadth setelah dua rilis decoder-robustness (v0.3.0 adaptive thresholding, v0.4.0 rotation handling).

> Status: **draft / dokumen hidup.** Milestone S1..S6 dikerjakan bertahap di branch `svg-renderer`; tiap milestone berupa commit fokus (atau seri commit kecil) yang sudah lengkap dengan tes, mengikuti ritme M1..M11, D1..D14, T1..T6, dan R1..R6.

> Versi Inggris: [docs/plan-svg-renderer.md](plan-svg-renderer.md).

---

## 1. Visi & Tujuan

- Menambahkan **output vektor yang scalable** ke library supaya caller dapat menghasilkan QR code yang resolution-independent — tajam di ukuran berapa pun, kecil di disk untuk payload umum, dan mudah ditanam di HTML, pipeline cetak, dan tool desain.
- Mengekspos-nya sebagai **fungsi publik yang additive** `EncodeSVG` dan `EncodeSVGToFile` yang mencerminkan bentuk `Encode` / `EncodeToFile` yang ada dan memakai ulang setiap option yang ada (`WithECLevel`, `WithVersion`, `WithMask`, `WithModuleSize`, `WithQuietZone`, `WithColors`). Tidak ada breaking change pada kontrak PNG-bytes milik `Encode`.
- Menjaga **filosofi yang sama** dengan setiap milestone sebelumnya: pure Go, zero runtime dependency di luar standard library (SVG adalah teks polos yang di-emit dengan `strings`/`fmt`), spec-first dengan theory doc bilingual, dan tes golden/round-trip.
- **Membayar utang dokumentasi** yang ditemukan saat triage: `docs/theory/08-rendering.md` menjanjikan bahwa "format lain dapat ditambahkan belakangan di balik `Render` interface yang sama", padahal interface itu tidak ada. v0.5.0 secara sengaja **tidak** memperkenalkan interface tersebut (YAGNI — hanya ada dua renderer dan tidak ada yang dipilih secara polimorfik saat runtime, sesuai keputusan straight-code-daripada-strategy-interface yang sudah dibuat untuk dispatch Sauvola di v0.3). Sebagai gantinya ia menambah `renderSVG` sebagai saudara `renderPNG` yang berbagi `renderOptions` yang ada, dan menulis ulang kalimat di doc 08 supaya mendeskripsikan sibling render function alih-alih interface.

## 2. Prinsip Desain

1. **Saudara, bukan interface.** `renderSVG(m *matrix, opts renderOptions) ([]byte, error)` duduk di samping `renderPNG` dengan signature identik. `EncodeSVG` memanggil `renderSVG` langsung sama seperti `Encode` memanggil `renderPNG` langsung. Tidak ada `Renderer` interface, tidak ada enum `WithFormat` — format dipilih lewat fungsi mana yang Anda panggil, yang menjaga kontrak return tiap fungsi tetap jelas.
2. **Memakai ulang seluruh paruh depan.** `EncodeSVG` menjalankan pipeline `resolveOptions -> validate -> buildMatrix` yang persis sama dengan `Encode`; hanya panggilan render terakhir yang berbeda. Matrix, masking, EC, dan plumbing option tidak disentuh, sehingga jalur encoder yang matang membawa zero regression risk.
3. **Path-data yang compact, bukan satu rect per modul.** Emit satu `<path>` yang atribut `d`-nya menggambar setiap modul gelap, alih-alih ratusan elemen `<rect>` individual. Ini pendekatan standar (Nayuki, qrcode-svg) dan menjaga ukuran file tetap kecil. Run-length merging untuk run modul gelap horizontal adalah optimisasi masa depan yang mungkin, dicatat tapi tidak diwajibkan untuk v0.5.
4. **Decodability dulu.** Set `shape-rendering="crispEdges"` supaya renderer tidak meng-anti-alias batas modul menjadi abu-abu buram yang akan merugikan scanning di hilir. Pakai sistem koordinat yang ramah integer.
5. **Resolution independence dengan ukuran nominal kompatibel-PNG.** `viewBox` dinyatakan dalam unit modul (`0 0 (n+2qz) (n+2qz)`) supaya simbol scale dengan bersih ke ukuran display apa pun, sementara `width`/`height` default ke `moduleSize * (n + 2*quietZone)` pixel sehingga SVG dan PNG yang dihasilkan dengan option yang sama mendeskripsikan dimensi nominal yang sama.
6. **Menghormati WithColors termasuk alpha.** Foreground/background dikonversi ke hex `#RRGGBB`; ketika sebuah warna membawa alpha di bawah opasitas penuh, emit `fill-opacity` di samping hex alih-alih bersandar pada bentuk hex 8-digit yang tidak didukung secara universal.
7. **Tes lebih dulu.** Setiap milestone disertai Go test berbasis tabel. Renderer divalidasi secara struktural (XML well-formed, kehadiran elemen/atribut yang benar, jumlah modul gelap cocok dengan matrix) dan end-to-end di mana memungkinkan.

## 3. Cakupan

### Termasuk di v0.5.0

- `renderSVG(m *matrix, opts renderOptions) ([]byte, error)` di file baru `qrgen/render_svg.go`.
- Publik `EncodeSVG(text string, opts ...Option) ([]byte, error)` dan `EncodeSVGToFile(text, path string, opts ...Option) error`.
- Pemakaian ulang penuh set option yang ada; `WithModuleSize`, `WithQuietZone`, dan `WithColors` semuanya berpengaruh pada output SVG.
- Rendering modul gelap satu-`<path>`, rect background yang menutupi seluruh kanvas termasuk quiet zone, `crispEdges`, `viewBox` unit-modul.
- Konversi warna-ke-hex dengan alpha ditangani via `fill-opacity`.
- Dukungan CLI: `cmd/qrgen` belajar meng-emit SVG, baik via flag `-format svg` atau dengan mendeteksi ekstensi `.svg` pada `-out` (diputuskan di S5).
- Theory doc `docs/theory/16-svg-rendering.md` (EN + ID) dan koreksi untuk `docs/theory/08-rendering.md`.
- Sebuah example yang bisa dijalankan di `examples/encode/svg`.

### Belum termasuk

- **Terminal / ASCII renderer.** Ditunda ke minor berikutnya (bentuk API beda — sebuah string atau `io.Writer`, bukan `[]byte`); dicatat di roadmap.
- **JPEG / PDF renderer.** Tidak diminta; JPEG lossy dan buruk untuk QR, PDF butuh writer yang lebih berat.
- **Sebuah `Renderer` interface atau enum `WithFormat`.** Secara eksplisit ditolak sesuai prinsip 1.
- **Optimisasi path run-length / rectangle-merge.** Path per-modul sudah cukup compact untuk v0.5; merging adalah penyetelan ukuran masa depan.
- **Logo embedding di SVG.** Dikelola terpisah di roadmap.

---

## 4. Milestone

Milestone dikerjakan berurutan. **Checkpoint A** (setelah S4) memberi `EncodeSVG` yang bekerja dan tervalidasi end-to-end. **Checkpoint B** (S6) adalah rilis `v0.5.0`.

### S1 — Plan Doc `(S)`

Goal: dokumen ini dan padanan Indonesia-nya, di-commit sebelum kode atau theory apa pun mendarat.

- [ ] `docs/plan-svg-renderer.md` dan `docs/plan-svg-renderer.id.md` yang mencakup visi, prinsip, cakupan, milestone S1..S6, delta layout file, risiko, referensi, pertanyaan terbuka.

### S2 — Theory Doc SVG + Koreksi Doc 08 `(S)`

Goal: menutupi model dokumen SVG dan pilihan rendering di `docs/theory/` sebelum kode apa pun mendarat, dan melunasi utang dokumentasi phantom-interface.

- [x] `docs/theory/16-svg-rendering.md` — delapan bagian: kenapa SVG, model dokumen SVG, menggambar satu-path vs satu rect per modul, sistem koordinat unit-modul dengan sizing pixel, `crispEdges` dan decodability, warna-ke-hex dengan `fill-opacity` untuk alpha (termasuk pembagian 0x101 dan catatan premultiplied-alpha), alasan sibling-function-bukan-interface, dan pointer implementasi.
- [x] Padanan Indonesia `docs/theory/16-svg-rendering.id.md`.
- [x] Mengoreksi `docs/theory/08-rendering.md` dan `.id.md`-nya: menulis ulang kalimat "behind the same `Render` interface" supaya mendeskripsikan sibling render function yang berbagi `renderOptions` dan signature `func(m *matrix, opts renderOptions) ([]byte, error)`, dengan cross-link ke doc 16 bagian 7.
- [x] Mengupdate `docs/theory/README.md` dan `docs/theory/README.id.md`: entry 16 di subsection baru "Output formats (v0.5.0)" plus satu baris code-mapping yang merujuk ke `qrgen/render_svg.go` (direncanakan, S3).
- [ ] **Disurfacing terpisah (belum diaksikan):** doc 08 juga mendokumentasikan contrast check dan option `WithSkipContrastCheck` yang tidak diimplementasi di kode (hanya komentar advisory pada `WithColors`). Diflag ke maintainer untuk memutuskan antara mengimplementasi check-nya atau menghapusnya dari doc; di luar cakupan koreksi interface S2.

### S3 — Implementasi `renderSVG` `(M)`

Goal: renderer-nya sendiri, berbagi `renderOptions` dengan `renderPNG`.

- [x] `qrgen/render_svg.go` dengan `renderSVG(m *matrix, opts renderOptions) ([]byte, error)`: menerapkan `withDefaults`, menjaga nil matrix dan dimensi/side invalid, meng-emit deklarasi XML, root `<svg>` dengan `viewBox` unit-modul dan `width`/`height` pixel plus `shape-rendering="crispEdges"`, sebuah `<rect>` background satu-kanvas-penuh, dan satu `<path>` foreground yang `d`-nya menggambar tiap modul gelap sebagai `M(c+qz) (r+qz)h1v1h-1z` yang di-offset oleh quiet zone. `<path>`-nya dihilangkan sepenuhnya untuk matrix all-light.
- [x] `colorToHex(c color.Color) (string, float64)` meng-un-premultiply via `channel*0xff/a` (yang meruntuhkan langkah un-premultiply dan 16-ke-8-bit menjadi satu pembagian eksak), mengembalikan `#000000, 0` untuk input fully transparent, dan opasitas fraksional `a/0xFFFF` selain itu.
- [x] `writeOpacity` meng-emit atribut `fill-opacity` hanya ketika opasitas `< 1`, diformat dengan desimal round-trip terpendek (`strconv.FormatFloat(op, 'g', 4, 64)`), supaya kasus opaque hitam-di-putih yang umum tetap minimal.
- [x] Tes di `qrgen/render_svg_test.go`: tiap output parse via `encoding/xml`; `viewBox`/`width`/`height` dicek terhadap matematika option untuk module-size/quiet-zone default dan custom; fill background dan foreground dicek untuk pasangan warna opaque custom; jumlah command move sama dengan total modul gelap; foreground half-alpha `image/color.NRGBA` meng-assert `fill-opacity ≈ 0.5`; matrix all-light tidak meng-emit `<path>`; nil matrix error; dan tabel `colorToHex` mencakup black, white, navy opaque, dan fully transparent. Dikonfirmasi manual SVG V1 "HI" asli merasterisasi ke QR tiga-finder yang valid via qlmanage.

### Checkpoint A — `renderSVG` menghasilkan SVG yang well-formed dan option-correct, tervalidasi secara struktural.

### S4 — API Publik + Example `(M)`

Goal: mengekspos renderer dan membuktikan round trip.

- [x] `EncodeSVG(text string, opts ...Option) ([]byte, error)` dan `EncodeSVGToFile(text, path string, opts ...Option) error` ditambahkan ke `qrgen/api.go`, masing-masing menjalankan paruh-depan `resolveOptions -> validate -> buildMatrix` yang identik dengan `Encode` dan hanya menukar `renderPNG` dengan `renderSVG`; varian file menulis dengan mode 0644.
- [x] Komentar doc mencerminkan `Encode`/`EncodeToFile`, mencatat set option yang dibagi, dan menunjuk ke doc 16.
- [x] **Cross-validation:** `TestEncodeSVGRoundTripGrid` merekonstruksi grid modul dari path yang di-emit — membaca dimensi kanvas dari `viewBox`, menurunkan quiet zone sebagai `(dim - n) / 2`, dan menelusuri tiap subpath `M x y` — lalu meng-assert ia sama dengan `Matrix(text, opts...)` sel demi sel. Mencakup V1-M default, URL di EC-Q, payload numerik quiet-zone-kecil, payload multi-block EC-H, dan kasus warna custom. Ini menutup loop encode -> SVG -> grid yang analog dengan tes round-trip decoder, bebas-dependency.
- [x] Example yang bisa dijalankan `examples/encode/svg/main.go` menulis SVG navy-di-cream bergaya; diverifikasi end-to-end (`go run ./examples/encode/svg`).
- [x] `qrgen/api_svg_test.go` juga mencakup propagasi module-size ke `width`/`height`, well-formedness output file, dan jalur error invalid-option. Race-clean.

### S5 — Dukungan SVG di CLI `(S)`

Goal: membuat SVG terjangkau dari binary `qrgen`.

- [x] Permukaan diputuskan jadi keduanya: flag `-format png|svg` menang ketika diset, dan ekstensi `.svg` pada `-out` menginferensi SVG ketika `-format` tidak diset, via helper `resolveFormat`. PNG tetap default untuk selain itu.
- [x] `runEncode` membangun daftar option yang dibagi sekali, lalu dispatch ke `EncodeSVG` atau `Encode` per format; nama file output default menjadi `qr.svg` untuk SVG dan tetap `qr.png` selain itu. Invokasi PNG yang ada byte-for-byte tidak berubah.
- [x] Banner help CLI dan doc package `cmd/qrgen` mendapat contoh SVG (inferensi `-out url.svg` dan `-format svg -out -` ke stdout).
- [x] Tes di `cmd/qrgen/main_test.go`: `-format svg` menulis SVG yang parseable ke file (bahkan dengan nama non-`.svg`) dan ke stdout; inferensi ekstensi `.svg` bekerja tanpa `-format`; `-format gif` invalid error; dan output `.png` tanpa-format tetap decode sebagai PNG sehingga default tidak regresi. Binary yang dibuild di-smoke-test. Race-clean.

### S6 — Benchmark, Polish Dokumentasi & Rilis `(S)`

Goal: memotong `v0.5.0`.

- [ ] Tambah `BenchmarkEncodeSVGSmall` dan `BenchmarkEncodeSVGURL` di samping benchmark encode yang ada; catat ns/op dan bytes/op supaya ukuran output SVG terlihat di samping PNG.
- [ ] README: `## Rendering to SVG` baru (atau lipat ke section rendering) dengan contoh kode, satu baris `EncodeSVG`/`EncodeSVGToFile` di tabel ringkasan API, contoh CLI `-format svg`, dan update Scope/Roadmap (hapus SVG dari "still out of scope" dan dari bullet roadmap renderer, menyisakan terminal/JPEG/PDF).
- [ ] Entry `CHANGELOG.md` `v0.5.0` di bawah `### Added` dan `### Validated` plus anchor compare/tag di bawah file.
- [ ] `go test -race ./...` bersih; benchmark encoder dalam variansi run-to-run milik v0.4 (SVG adalah jalur baru, jadi satu-satunya perhatian adalah kode paruh-depan yang dibagi tidak tersentuh).
- [ ] Tag `v0.5.0` setelah push pertama ke GitHub supaya tag mendarat pada commit yang dilihat remote. Dikerjakan manual oleh user; annotation direkomendasikan di percakapan rilis.

---

## 5. Usulan Delta Layout File

```
qrgen/
├── render_png.go            # eksisting — tidak berubah
├── render_svg.go            # baru — renderSVG + helper colorToHex
├── render_svg_test.go       # baru — unit test SVG struktural
├── api.go                   # eksisting — mendapat EncodeSVG / EncodeSVGToFile (atau api_svg.go baru)
├── api_svg_test.go          # baru — tes output byte/file + round-trip grid
└── encode_bench_test.go     # eksisting atau baru — mendapat BenchmarkEncodeSVG*
cmd/qrgen/
├── main.go                  # eksisting — mendapat flag -format / inferensi .svg
└── main_test.go             # eksisting — mendapat tes CLI SVG
examples/encode/svg/
└── main.go                  # baru — demo SVG yang bisa dijalankan
docs/
├── plan-svg-renderer.md     # versi Inggris
├── plan-svg-renderer.id.md  # file ini
└── theory/
    ├── 08-rendering.md       # dikoreksi: sibling render funcs, bukan Render interface
    ├── 08-rendering.id.md    # koreksi yang sama
    ├── 16-svg-rendering.md   # baru
    └── 16-svg-rendering.id.md # baru
```

## 6. Risiko & Catatan Teknis

- **Kebenaran XML dan escaping.** SVG adalah XML; atribut numerik sepenuhnya kita kontrol sehingga injeksi bukan kekhawatiran, tapi emitter harus menghasilkan output yang well-formed (namespace yang benar, tag tertutup, atribut dalam tanda kutip). Tes mem-parse output dengan `encoding/xml` untuk menjamin well-formedness alih-alih melihat string secara kasat mata.
- **Anti-aliasing pada skala fraksional.** Kalau viewer men-scale `viewBox` unit-modul ke ukuran pixel non-integer, tepi modul bisa buram. `shape-rendering="crispEdges"` memitigasi ini; theory doc mencatat trade-off-nya dan kenapa kita tetap lebih memilih sistem koordinat unit-modul demi scalability.
- **Konversi color-model.** `color.Color.RGBA()` mengembalikan channel premultiplied 16-bit; mengonversi ke hex 8-bit harus membagi dengan 0x101 (bukan sekadar bit-shift) supaya membulatkan dengan benar, dan penanganan alpha harus menghindari double-premultiplication. Helper `colorToHex` di-unit-test terhadap warna yang diketahui.
- **Ekspektasi ukuran file.** Satu `<path>` untuk simbol V40 besar tapi tetap lebih kecil dari PNG ekuivalen untuk mayoritas payload, dan mudah di-gzip. Run-length merging akan mengecilkannya lagi; ditunda.
- **Tidak ada dampak decoder.** Decoder tidak pernah membaca SVG. Tidak ada perubahan pada jalur decode, jadi seluruh suite tes dan benchmark decoder tidak terpengaruh — tapi `go test -race ./...` tetap menjalankannya untuk memastikan tidak ada yang rusak di package yang dibagi.
- **Creep permukaan CLI.** Menambah `-format` tidak boleh mengubah perilaku default invokasi PNG yang ada; flag-nya default ke PNG dan inferensi `.svg` hanya menyala ketika `-format` tidak diset.

## 7. Referensi

- ISO/IEC 18004:2015 — §11 (rendering simbol bersifat implementation-defined; spec membatasi grid modul, bukan medium output).
- W3C — *Scalable Vector Graphics (SVG) 1.1 (Second Edition)*: <https://www.w3.org/TR/SVG11/>. Grammar path data (§8.3), `shape-rendering` (§11.2), basic shapes.
- Project Nayuki — *QR Code generator library*: method `toSvgString`-nya me-render seluruh simbol sebagai satu path, pendekatan yang diadopsi di sini. <https://www.nayuki.io/page/qr-code-generator-library>
- `docs/theory/08-rendering.md` — catatan rendering PNG yang ada, dikoreksi di S2.

## 8. Pertanyaan Terbuka

Untuk dijawab sebelum milestone yang bersangkutan dimulai:

- **Permukaan format CLI.** `-format png|svg` eksplisit, inferensi ekstensi dari `-out`, atau keduanya? Condong ke keduanya: `-format` menang ketika diset, ekstensi `.svg` menginferensi ketika tidak. Diselesaikan di S5.
- **Penempatan file API.** Taruh `EncodeSVG`/`EncodeSVGToFile` di `api.go` yang ada di samping `Encode`, atau `api_svg.go` khusus? Condong ke `api.go` demi discoverability karena permukaannya kecil; tinjau lagi kalau membuat file sesak.
- **Optimisasi path.** Rilis path per-modul sederhana di v0.5 dan tinggalkan run-length rectangle merging untuk pass fokus-ukuran berikutnya? Default ya — kebenaran dan kejelasan dulu.
- **Kedalaman round-trip grid.** Apakah mem-parse path yang di-emit kembali ke `[][]bool` cukup sebagai cross-validation, atau tes juga harus merasterisasi SVG via renderer pihak ketiga? Default ke parse-path saja supaya tes bebas-dependency, konsisten dengan kebijakan stdlib-only.
