# Terminal Rendering

Renderer v0.8 menambahkan output **teks** di samping raster PNG (doc 08) dan dokumen vektor SVG (doc 16). Kalau keduanya menulis gambar, `renderTerminal` menulis string multi-baris berisi block character yang digambar terminal sebagai simbol yang bisa dipindai, dan dibaca kamera ponsel langsung dari layar. Dokumen ini mencatat aspect ratio sel yang melatari half-block packing, pemetaan pasangan-modul ke glyph, fallback ASCII, masalah polaritas yang menentukan apakah simbol bisa terpindai sama sekali, penanganan quiet zone dan baris ganjil, serta kenapa renderer ini fungsi sibling yang mengembalikan string.

> Versi Inggris: [19-terminal-rendering.md](19-terminal-rendering.md).

## 1. Kenapa renderer terminal

- **Tanpa file, tanpa viewer.** Simbol QR sering dibutuhkan di tempat program memang sudah berjalan: shell. Mencetaknya ke terminal membuat sebuah setup script bisa menampilkan kode join Wi-Fi, enrolment TOTP, atau URL pairing tanpa file gambar ditulis dan tanpa image viewer dibuka.
- **Teks yang pipeable.** Output-nya `string` polos, jadi menyatu dengan redirection, logging, dan konteks teks apa pun, persis seperti payload builder (doc 18) yang menghasilkan string polos.
- **Bisa dipindai dari layar.** Kamera tidak peduli apakah sel gelap dan terang itu pixel atau karakter; ia hanya butuh modul yang mendekati persegi, kontras yang cukup, dan quiet zone. Terminal bisa menyediakan ketiganya (bagian 2, 5, 6).
- **Bukan soal resolusi.** Tak seperti SVG, intinya bukan scaling — simbol terminal sebesar grid karakternya. Intinya kesegeraan: simbol muncul di stream yang sama dengan output program lainnya.

## 2. Aspect ratio sel dan half-block packing

Sel terminal monospace tidak persegi. Biasanya kira-kira dua kali lebih tinggi daripada lebarnya (rasio tinggi banding lebar mendekati 2:1). Modul QR itu persegi. Jadi rendering naif — satu sel terminal per modul — menghasilkan simbol yang teregang menjadi kira-kira dua kali tinggi semestinya, yang memboroskan ruang vertikal dan bisa membingungkan scanner yang mengharapkan modul persegi.

Solusinya adalah **half-block packing**: tiap sel terminal membawa *sepasang modul vertikal*, modul atas dan modul bawah, memakai Unicode Block Element yang mengisi separuh atas atau separuh bawah sebuah sel. Karena sel itu kira-kira 2:1, tiap separuh-sel kira-kira 1:1 — persegi — sehingga tiap modul mendarat di footprint yang mendekati persegi, dan keseluruhan simbol jadi separuh lebih pendek dibanding grid naif. Ini teknik yang sama dipakai `qrencode -t UTF8` dan library `qrterminal`.

## 3. Pemetaan pasangan-modul ke glyph

Untuk sepasang modul vertikal — atas `T`, bawah `B`, masing-masing gelap atau terang — glyph sel dipilih supaya tinta (foreground) yang tergambar menutupi persis separuh yang gelap:

| Atas `T` | Bawah `B` | Glyph | Nama (code point)             |
|----------|-----------|-------|-------------------------------|
| gelap    | gelap     | `█`   | Full Block (U+2588)           |
| gelap    | terang    | `▀`   | Upper Half Block (U+2580)     |
| terang   | gelap     | `▄`   | Lower Half Block (U+2584)     |
| terang   | terang    | (spasi) | spasi (U+0020)              |

Di terminal berlatar terang, glyph dilukis dengan foreground gelap dan separuh yang tak terlukis menampilkan background terang, jadi keempat kasus mereproduksi keempat pasangan modul gelap/terang dengan setia. Satu simbol penuh dikeluarkan pasangan-baris demi pasangan-baris, kiri ke kanan, satu glyph per kolom, tiap baris output diakhiri newline.

Contoh mikro. Ambil pola 3-kali-3 ini (`X` gelap, `.` terang), quiet zone dihilangkan demi kejelasan:

```text
X . X
. X .
X . X
```

Tiga baris itu ganjil, jadi ada satu pasangan penuh `(row0, row1)` dan satu `row2` tak berpasangan di ekor (bagian 6). Membaca kolom kiri ke kanan:

- pasangan `(row0, row1)`: `X/.` -> `▀`, `./X` -> `▄`, `X/.` -> `▀`  menghasilkan `▀▄▀`
- ekor `row2` dengan bawah terang implisit: `X` -> `▀`, `.` -> spasi, `X` -> `▀`  menghasilkan `▀ ▀`

```text
▀▄▀
▀ ▀
```

## 4. Fallback ASCII

Glyph block tersedia universal di terminal modern, tetapi mode ASCII murni (`WithTerminalASCII`) menutup font atau pipe langka yang tak bisa menampilkannya. ASCII tidak punya glyph separuh-sel, jadi ia kembali ke **satu baris terminal per baris modul** dan memulihkan modul persegi dengan melipatduakan lebar: tiap modul gelap menjadi dua karakter (`##`) dan tiap modul terang dua spasi. Satu karakter per modul akan ter-render pada lebar-banding-tinggi native sel 1:2 dan tampak teregang ke arah sebaliknya; dua karakter lebar terhadap satu sel tinggi kira-kira 2:2, yakni persegi lagi. Fallback ini karena itu benar tetapi kira-kira dua kali lebih tinggi dibanding half-block, yang merupakan ongkos melepas glyph block.

## 5. Polaritas dan keterbacaan

Inilah satu hal yang menentukan apakah simbol terpindai. Sebuah glyph block dilukis dengan warna foreground terminal. Di terminal **berlatar terang** foreground itu gelap, jadi sebuah glyph terbaca sebagai modul gelap — benar. Di terminal **berlatar gelap** foreground-nya terang, jadi glyph yang sama terbaca sebagai modul *terang*, membalik simbol; scanner lalu melihat QR dengan gelap dan terang tertukar, dan decoder standar tak akan membacanya.

Renderer tidak mencoba mendeteksi tema terminal. Ia default ke kasus latar terang dan menyediakan `WithTerminalInvert` untuk latar gelap. Inversi adalah persis komplemennya: gelap/terang tiap modul dibalik sebelum pemetaan bagian 3 berjalan, yang menukar `█` dengan spasi dan `▀` dengan `▄`, lalu mengubah quiet zone terang menjadi glyph terlukis. Di terminal gelap, sel quiet zone terlukis itu terbaca terang dan modul yang tertukar terbaca dengan polaritas benar, jadi kamera melihat simbol normal lagi. Ini analog terminal dari panduan kontras pada `WithColors` (doc 08): library bisa menempatkan modul dengan benar, tetapi manusianya harus memastikan gelap benar-benar tampak lebih gelap daripada terang di layarnya.

v0.8 hanya mengeluarkan teks polos. Mode theme-independent yang menulis escape code foreground/background ANSI — yang akan menghormati `WithColors` dan menghapus tebak-tebakan polaritas sepenuhnya — adalah follow-up yang disengaja, bukan bagian rilis ini.

## 6. Quiet zone dan ekor baris ganjil

Quiet zone bukan opsional. Scanner butuh pita modul terang di sekeliling simbol untuk mengunci finder pattern, jadi `WithQuietZone` (default 4) dihormati di sini persis seperti di renderer raster; border terang dikeluarkan sebagai spasi (atau, dalam mode invert, sebagai glyph terlukis).

Half-block packing punya satu kasus tepi struktural. Sisi simbol QR `n` selalu ganjil (21, 25, ... 177), dan menambah quiet zone simetris sebanyak `q` modul tetap membuat total `n + 2q` ganjil. Memasangkan baris dua-dua karena itu selalu menyisakan satu baris terakhir tak berpasangan. Renderer mengeluarkan baris terakhir itu sebagai glyph separuh-atas di atas bawah terang implisit: modul gelap menjadi `▀`, modul terang menjadi spasi. Menangani ekor secara eksplisit menjaga tepi bawah simbol — biasanya baris-baris terakhir quiet zone — tetap benar alih-alih hilang atau terdobel.

## 7. Fungsi sibling yang mengembalikan string

`renderTerminal` mengikuti pola fungsi sibling yang dimapankan untuk para renderer (doc 16, bagian 7): tidak ada interface `Render` dan tidak ada variabel format runtime. `Encode` memanggil `renderPNG`, `EncodeSVG` memanggil `renderSVG`, dan `EncodeTerminal` memanggil `renderTerminal`, masing-masing langsung. Memilih output lewat fungsi mana yang dipanggil menjaga tiap kontrak kembalian tetap tak ambigu.

Satu beda yang disengaja adalah tipe kembalian. `renderPNG` dan `renderSVG` mengembalikan `[]byte` karena payload mereka berupa raster biner dan dokumen yang dimaksudkan ditulis ke file atau socket. `renderTerminal` mengembalikan `string` karena payload-nya teks yang dilihat manusia dan dimaksudkan untuk dicetak; sebuah `string` menyatu wajar dengan `fmt.Print` dan tak perlu decoding untuk diperiksa dalam test. Saklar terminal-spesifik — invert dan ASCII — berjalan dalam struct kecil `terminalOptions`, bukan `renderOptions` raster, karena field berorientasi pixel (`moduleSize`, `foreground`, `background`) tak bermakna untuk output teks dan didokumentasikan sebagai no-op, sama seperti pada `Matrix`.

## 8. Penunjuk implementasi

- `qrgen/render_terminal.go` memuat `renderTerminal(m *matrix, opts terminalOptions) string` plus struct kecil `terminalOptions` (`quietZone`, `invert`, `ascii`). Ia membangun output dengan `strings.Builder`, menyusuri grid yang sudah dipadkan secara pasangan-baris dan memilih tiap glyph dari pemetaan bagian 3 (atau bentuk ASCII-nya), dengan ekor bagian 6 ditangani setelah loop pasangan.
- `EncodeTerminal` di `qrgen/api.go` menjalankan separuh-depan `resolveOptions -> validate -> buildMatrix` yang sama dengan `Encode`, lalu memanggil `renderTerminal` alih-alih renderer raster. `WithTerminalInvert` dan `WithTerminalASCII` di `qrgen/options.go` mengalirkan kedua saklar itu.
- Test mengunci output persis dengan golden string multi-baris untuk mode half-block, ASCII, dan invert, dan — karena rendering-nya loss-free — mem-parse glyph kembali menjadi grid `[][]bool` lalu menjalankannya lewat `DecodeMatrix`, memastikan teks hasil decode sama dengan input. Parse-back ini adalah kebalikan pemetaan bagian 3 dan sekaligus menjadi cek bahwa pemetaannya tak ambigu.

## Referensi

- The Unicode Standard — *Block Elements* (U+2580–U+259F): glyph half dan full block `▀` (U+2580), `▄` (U+2584), `█` (U+2588). <https://www.unicode.org/charts/PDF/U2580.pdf>
- `qrencode` — mode output terminal `-t UTF8` dan `-t UTF8i` (terbalik), yang memapankan half-block packing dan saklar invert untuk terminal gelap. <https://fukuchi.org/works/qrencode/>
- `qrterminal` — library Go yang me-render QR ke terminal dengan opsi half-block dan invert, titik banding langsung. <https://github.com/mdp/qrterminal>
- `docs/theory/08-rendering.md` dan `docs/theory/16-svg-rendering.md` — renderer raster dan vektor; keharusan quiet zone dan rasional fungsi sibling tetap berlaku, dan panduan kontras di sana adalah panduan polaritas di sini.
- ISO/IEC 18004:2015 — simbologi QR itu sendiri; terminal rendering adalah target output di atasnya.
