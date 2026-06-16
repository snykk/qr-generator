# QR Encoder — Rencana Renderer Terminal / ASCII

Dokumen ini menjelaskan rencana implementasi **renderer terminal / ASCII** untuk rilis minor `v0.8.0`. Ini melanjutkan fase perluasan output (SVG di v0.5) dengan menggambar simbol QR memakai Unicode block element (atau ASCII polos), sehingga sebuah kode bisa langsung dicetak ke terminal, dialirkan ke file teks, atau ditempel di konteks teks apa pun, lalu dipindai dengan kamera ponsel langsung dari layar.

> Status: **draf / dokumen hidup.** Milestone TR1..TR5 mendarat bertahap di branch `terminal-renderer`; tiap milestone adalah commit fokus (atau deret commit kecil) berikut test, mengikuti irama milestone M, D, T, R, S, MM, dan P.

> Versi Inggris: [docs/plan-terminal-renderer.md](plan-terminal-renderer.md).

---

## 1. Visi & Tujuan

- Me-render matrix modul hasil encode menjadi `string` multi-baris berisi block character yang bisa dipindai kamera ponsel langsung dari layar, tanpa file gambar di tengah jalan.
- Membuatnya **menyatu dengan pipeline yang sama.** `EncodeTerminal(text, opts...)` menjalankan version selection, pemilihan mask, dan penanganan option yang persis sama dengan `Encode` dan `EncodeSVG`, dan hanya berbeda di render step terakhir, sama seperti `EncodeSVG` yang sudah berbeda dari `Encode`.
- **Ringkas secara default** lewat half-block vertical packing: tiap baris teks mewakili sepasang baris modul secara vertikal memakai glyph block, sehingga hasilnya separuh lebih pendek dibanding grid naif satu-baris-per-modul, dan tiap modul tetap mendekati persegi mengingat aspect ratio sel terminal yang kira-kira 2:1 (tinggi banding lebar).
- Menyediakan **fallback ASCII** untuk terminal atau font tanpa dukungan Unicode block element, dan kontrol **invert** untuk terminal berlatar gelap.
- Menjaga perubahan ini **murni aditif dan tidak menyentuh engine.** Renderer membaca matrix yang sudah jadi lalu mengeluarkan teks; ia tidak menyentuh pipeline encoder maupun decoder, dan tidak ada perubahan API yang sudah ada.
- Filosofinya sama dengan tiap milestone sebelumnya: pure Go, zero runtime dependency (`strings.Builder` dari standard library), reference-first dengan dokumen bilingual, serta test golden-string plus parse-back round-trip lewat `DecodeMatrix`.

## 2. Prinsip Desain

1. **`EncodeTerminal` mengembalikan `string`.** Output terminal adalah teks yang memang untuk dicetak, jadi `string` menyatu wajar dengan `fmt.Print` dan gampang di-unit-test. Ini sengaja berbeda dari `Encode`/`EncodeSVG` yang mengembalikan `[]byte` karena payload mereka berupa raster biner dan dokumen; di sini payload-nya teks yang dilihat manusia.
2. **Half-block packing sebagai default.** Tiap sel output mewakili sepasang modul vertikal dengan Unicode Block Element `█` (dua-duanya gelap), `▀` (atas gelap), `▄` (bawah gelap), dan spasi (dua-duanya terang). Ini memangkas jumlah baris dibanding satu baris terminal per modul, dan membuat modul mendekati persegi, karena sel terminal kira-kira dua kali lebih tinggi daripada lebarnya dan pembagian atas/bawah memberi tiap modul footprint sekitar 1:1.
3. **Fallback ASCII demi portabilitas.** `WithTerminalASCII` me-render tiap modul gelap sebagai dua karakter (`##`) dan tiap modul terang sebagai dua spasi, melipatduakan lebar agar modul tetap persegi tanpa bergantung pada glyph block element. Mode ini satu baris terminal per baris modul, jadi lebih tinggi tetapi paling kompatibel.
4. **Polaritas eksplisit, bukan ditebak.** Default mengasumsikan terminal berlatar terang, di mana glyph block terbaca gelap; `WithTerminalInvert` membalik polaritas untuk terminal berlatar gelap supaya modul gelap hasil render tetap terbaca gelap oleh kamera. v0.8 mengeluarkan teks polos tanpa warna ANSI; mode ANSI yang theme-independent dan menghormati `WithColors` dicatat sebagai enhancement masa depan, bukan bagian rilis ini.
5. **Quiet zone dipakai ulang.** `WithQuietZone` berlaku apa adanya (default 4 modul terang di tiap sisi), yang memang dibutuhkan scanner. `WithModuleSize` dan `WithColors` tidak berpengaruh pada output teks polos dan didokumentasikan sebagai no-op di sini, persis seperti pada `Matrix`, dicadangkan untuk mode ANSI masa depan.
6. **Test lebih dulu, termasuk parse-back round-trip.** Selain golden string yang mengunci output visual persis, sebuah test round-trip me-render matrix, mem-parse block character kembali menjadi grid `[][]bool`, menjalankannya lewat `DecodeMatrix`, lalu memastikan teks hasil decode sama dengan input. Ini membuktikan rendering loss-free dan orientasinya benar tanpa perlu image decoder.

## 3. Cakupan

### Termasuk dalam v0.8.0

- `EncodeTerminal(text string, opts ...Option) (string, error)` — rendering half-block Unicode atas simbol hasil encode, termasuk quiet zone.
- `WithTerminalInvert(bool) Option` — membalik polaritas gelap/terang untuk terminal berlatar gelap.
- `WithTerminalASCII(bool) Option` — fallback ASCII lebar-ganda (`##` / spasi) untuk terminal tanpa dukungan block element.
- `WithQuietZone` dihormati pada output terminal; `WithModuleSize` dan `WithColors` didokumentasikan sebagai no-op untuk output terminal (seperti `Matrix`), dicadangkan untuk mode ANSI masa depan.
- CLI: `-format terminal` (half-block Unicode) dan `-format ascii` (fallback ASCII), plus flag `-invert`, menulis ke stdout atau `-out`.
- Dokumen referensi `docs/theory/19-terminal-rendering.md` (EN + ID) yang membahas aspect ratio sel, half-block packing, glyph block, fallback ASCII, polaritas/inversi, quiet zone, dan keterbacaan dari layar, lengkap dengan sitasi.
- Sebuah contoh runnable `examples/encode/terminal/main.go` dan bagian penggunaan di README.

### Belum termasuk (masih)

- **Output warna ANSI.** Rendering theme-independent yang mengeluarkan escape code foreground/background sesuai `WithColors`. Ditunda ke kemungkinan follow-up; v0.8 adalah teks polos plus saklar `invert`.
- **Zoom `WithModuleSize` di terminal.** Mengulang modul secara horizontal/vertikal untuk memperbesar simbol terminal. Kemungkinan follow-up; v0.8 mengunci satu sel half-block per modul.
- **Deteksi otomatis latar terminal atau TTY** untuk memilih polaritas sendiri. Pemanggil memilih lewat `WithTerminalInvert`; deteksi otomatis paling jauh ada di lapisan CLI, bukan library.
- **Protokol inline-image terminal** (iTerm2, Kitty, sixel). Itu output gambar, bukan rendering teks, dan akan menghidupkan kembali jalur raster.
- **Wrapper `EncodeTerminalToFile`.** Sebuah `string` gampang ditulis oleh pemanggil atau `-out` di CLI, jadi helper file khusus hanya menambah permukaan tanpa nilai.

---

## 4. Milestone

Milestone mendarat berurutan. **Checkpoint A** (setelah TR3) memberi renderer dengan output yang sudah terkunci golden. **Checkpoint B** (TR5) adalah rilis `v0.8.0`.

### TR1 — Plan Doc `(S)`

- [x] `docs/plan-terminal-renderer.md` dan padanan Indonesianya, mencakup visi, prinsip, cakupan, milestone TR1..TR5, delta tata letak file, risiko, referensi, dan pertanyaan terbuka.

### TR2 — Dokumen Referensi Terminal Rendering `(S)`

Tujuan: mendokumentasikan teknik rendering dan trade-off-nya sebelum kode apa pun mendarat.

- [x] `docs/theory/19-terminal-rendering.md` — aspect ratio sel terminal dan kenapa half-block packing menjaga modul tetap persegi; pemetaan persis pasangan-modul ke glyph (`█ ▀ ▄` dan spasi) berikut code point Unicode Block Element-nya; fallback ASCII lebar-ganda; masalah polaritas (sebuah glyph terbaca gelap di terminal terang dan terang di terminal gelap) dan bagaimana `invert` menyelesaikannya; keharusan quiet zone; bagaimana jumlah baris total yang ganjil menyisakan satu baris modul tak berpasangan; serta contoh kerja untuk simbol kecil. Ditutup dengan rasional compose-not-encode dan penunjuk implementasi.
- [x] Padanan Indonesia `docs/theory/19-terminal-rendering.id.md`.
- [x] Memperbarui `docs/theory/README.md` dan `.id.md`: entri 19 plus baris pemetaan kode yang menunjuk ke `qrgen/render_terminal.go`.

### TR3 — Renderer + Option + Golden Test `(M)`

Tujuan: renderer-nya sendiri, dengan output terkunci golden string.

- [x] `qrgen/render_terminal.go` dengan `renderTerminal(m *matrix, opts terminalOptions) string` (saudara `renderPNG`/`renderSVG`), mengimplementasikan half-block packing, fallback ASCII, saklar invert, dan quiet zone. Menangani baris terakhir tak berpasangan (separuh atas saja) dengan rapi.
- [x] `EncodeTerminal` di `qrgen/api.go` dan `WithTerminalInvert` / `WithTerminalASCII` di `qrgen/options.go`, dialirkan lewat resolved options.
- [x] Test golden-string di `qrgen/render_terminal_test.go`: simbol dengan mask terpaku, di-render dalam mode half-block, ASCII, dan invert, dibandingkan terhadap golden string multi-baris; lebar quiet zone; kasus baris ganjil terakhir.

### Checkpoint A — renderer menghasilkan output terminal yang benar, stabil, dan bisa dipindai.

### TR4 — Parse-Back Round-Trip + CLI + Contoh `(S)`

Tujuan: membuktikan rendering loss-free dan menyambungkannya ke CLI.

- [x] `TestTerminalRoundTrip` me-render sejumlah payload, mem-parse block character kembali menjadi `[][]bool`, menjalankan `DecodeMatrix`, lalu memastikan teks hasil decode sama dengan input — untuk mode half-block, ASCII, dan invert.
- [x] CLI: `-format terminal` dan `-format ascii` dialirkan lewat `EncodeTerminal`; flag `-invert` dipetakan ke `WithTerminalInvert`; output ke stdout secara default untuk format ini. Teks usage dan contoh diperbarui.
- [x] Contoh runnable `examples/encode/terminal/main.go` mencetak QR sebuah URL ke stdout dalam half-block lalu sekali lagi dalam mode invert; diverifikasi dengan `go run`.

### TR5 — Polish & Rilis `(S)`

Tujuan: memotong `v0.8.0`.

- [x] README: bagian penggunaan "Terminal output"; satu baris terminal di ringkasan API yang mencantumkan `EncodeTerminal`, `WithTerminalInvert`, `WithTerminalASCII`; Scope mendapat baris output terminal; butir Roadmap "Additional renderers" mencatat terminal/ASCII rilis di v0.8 (JPEG/PDF masih terbuka).
- [x] Entri `v0.8.0` di `CHANGELOG.md` plus anchor compare/tag ditulis; dibiarkan unstaged di working tree agar maintainer yang commit bersama rilis (meniru v0.6 dan v0.7).
- [x] `go test -race ./...` bersih, gofmt bersih.
- [ ] Tag `v0.8.0` (diserahkan ke maintainer sesuai alur git/rilis yang sudah mapan; anotasi direkomendasikan di percakapan rilis).

---

## 5. Usulan Delta Tata Letak File

```
qrgen/
├── render_terminal.go        # baru — renderTerminal(m, opts) string
├── render_terminal_test.go   # baru — golden + parse-back round-trip
├── api.go                    # +EncodeTerminal
├── options.go                # +WithTerminalInvert, +WithTerminalASCII (+field)
cmd/qrgen/
└── main.go                   # +format terminal/ascii, +flag -invert
docs/
├── plan-terminal-renderer.md     # file ini
├── plan-terminal-renderer.id.md  # padanan Indonesia
└── theory/
    ├── 19-terminal-rendering.md     # baru
    └── 19-terminal-rendering.id.md  # baru
examples/encode/terminal/
└── main.go                   # baru — demo half-block + invert ke stdout
```

## 6. Risiko & Catatan Teknis

- **Polaritas adalah inti keterbacaan.** Sebuah glyph block terbaca gelap di terminal berlatar terang tetapi terang di terminal berlatar gelap; kalau polaritasnya salah, modul gelap hasil render tampak terang bagi kamera dan simbol tidak akan terpindai. Default menyasar latar terang; `WithTerminalInvert` menutup kasus latar gelap. Ini analog terminal dari panduan kontras pada `WithColors`, dan didokumentasikan secara menonjol.
- **Dukungan font half-block.** `█ ▀ ▄` adalah Unicode Block Element (U+2588, U+2580, U+2584), didukung tiap font monospace dan terminal modern. Fallback ASCII (`WithTerminalASCII`) menutup lingkungan langka yang tak memilikinya.
- **Jumlah baris total ganjil.** Sisi simbol QR selalu ganjil (21, 25, ... 177), dan menambah quiet zone simetris tetap membuat totalnya ganjil. Half-block mengemas baris berpasangan, jadi baris terakhir tak berpasangan dan di-render sebagai glyph separuh-atas (`▀`) di atas bagian bawah terang implisit. Renderer menangani kasus terakhir ini secara eksplisit supaya tepi bawah benar.
- **Aspect ratio.** Sel terminal kira-kira dua kali lebih tinggi daripada lebarnya. Half-block membagi tiap sel menjadi separuh atas dan bawah, memberi tiap modul footprint mendekati persegi; fallback ASCII melipatduakan lebar per modul untuk mencapai hal yang sama. Keduanya menjaga simbol tampak persegi sehingga terpindai bersih.
- **Tanpa urusan anti-aliasing atau scaling.** Tak seperti jalur PNG, rendering teks bersifat eksak: sebuah modul adalah glyph atau spasi, jadi tidak ada ambiguitas binarisasi bagi kamera selain pilihan polaritas.
- **Round-trip tanpa gambar.** Mem-parse glyph hasil render kembali menjadi `[][]bool` lalu memanggil `DecodeMatrix` memvalidasi kebenaran secara langsung, melewati image decoder; parse-back harus membalik persis pemetaan char-ke-modul untuk tiap mode (half-block, ASCII, invert), yang juga menjadi uji berguna atas kejelasan pemetaan itu.

## 7. Referensi

- Unicode Block Elements, U+2580–U+259F — glyph half dan full block (`▀` U+2580, `▄` U+2584, `█` U+2588).
- Konvensi QR-terminal de-facto — `qrencode -t UTF8` / `UTF8i` dan library Go `qrterminal`, yang memapankan half-block packing dan saklar invert untuk terminal gelap.
- ISO/IEC 18004:2015 — simbologi QR itu sendiri; terminal rendering adalah target output di atasnya, seperti PNG dan SVG.

## 8. Pertanyaan Terbuka

Dijawab sebelum milestone terkait dimulai:

- **Set karakter default.** Half-block Unicode (dipilih: ringkas, mendekati persegi, didukung universal di terminal modern) versus full-block lebar-ganda (kompatibilitas maksimal tetapi dua kali lebih tinggi). Fallback ASCII sudah menutup lingkungan non-Unicode, jadi half-block jadi default. Dikonfirmasi di TR2.
- **Tipe kembalian.** `string` (dipilih, lihat prinsip 1) versus `[]byte` demi simetri dengan `Encode`/`EncodeSVG`. Output teks lebih cocok `string`.
- **Default polaritas.** Mengasumsikan terminal berlatar terang plus saklar `invert` (dipilih) versus mengeluarkan warna ANSI agar theme-independent (ditunda ke follow-up). Dikonfirmasi di TR2.
- **Token CLI.** `-format terminal` (Unicode) plus `-format ascii` (ASCII) plus flag `-invert` (diusulkan) versus satu `-format terminal` dengan flag `-ascii` terpisah. Dua token format lebih jelas dibaca di command line.
- **Newline akhir.** Apakah `EncodeTerminal` menutup baris terakhir dengan newline. Diusulkan: ya, tiap baris termasuk yang terakhir diakhiri newline supaya output tercetak rapi dan ter-concat secara terduga.
