# QR Generator — Rencana Web Demo

Dokumen ini menjelaskan rencana implementasi **web demo** untuk library ini, dengan target rilis minor `v0.18.0`: sebuah halaman statis di GitHub Pages yang membuat, membedah, dan memindai simbol QR Code, Micro QR Code, dan rMQR Code memakai library ini sendiri, yang di-compile ke WebAssembly. Milestone ini juga menambahkan `qrgen.Inspect`, API publik yang melaporkan bagaimana sebuah simbol dibangun, lapis demi lapis, karena tampilan edukatif di halaman membutuhkannya, begitu pula siapa pun yang mempelajari library ini.

> Status: **draf / dokumen hidup.** Milestone WD1..WD6 mendarat bertahap di branch `web-demo`; tiap milestone adalah commit fokus (atau deret kecil) berikut test, mengikuti irama milestone sebelumnya. Milestone ini diimplementasikan sebelum dokumennya ditulis, atas permintaan pemilik repo, dan dokumen ini menjelaskan apa yang sudah dibangun.

> Versi bahasa Inggris: [docs/plan-web-demo.md](plan-web-demo.md).

---

## 1. Visi & Tujuan

- Siapa pun bisa mencoba library ini tanpa memasang Go: setiap option encoding, payload builder, structured append, dan decoding gambar, langsung di browser.
- Menjalankan **library yang asli**, bukan tiruan JavaScript: kode Go di-compile ke WebAssembly, sehingga halaman menunjukkan persis apa yang dilakukan `Encode`, `Inspect`, dan `DecodeAll`.
- Mengajar: menunjukkan fungsi tiap modul dalam simbol, segmen dan anggaran bit pesan, codeword dan bloknya, serta cara mask dipilih, dengan tautan dari tiap bagian ke theory doc-nya.
- Menjaga karakter repo: tanpa runtime dependency untuk library, tanpa toolchain JavaScript, dan tanpa langkah build untuk halaman selain meng-compile kode Go.
- Memvalidasi API baru terhadap fakta yang tidak berasal dari dirinya sendiri: contoh kerja di theory doc 10 dan 28, tata letak yang dihitung manual, dan encoder yang sudah ada, yang harus direproduksi `Inspect` dengan persis.

## 2. Apa isi demo ini

- **Create.** Teks, URL, atau payload terstruktur (Wi-Fi, vCard, email, telepon, SMS, lokasi) di-encode sebagai QR Code, Micro QR Code, atau rMQR Code, dengan semua option yang dimiliki library: level error correction, versi, tinggi rMQR, mask, ECI, quiet zone, ukuran modul, warna, logo, dan structured append. Ada pratinjau langsung, unduhan PNG dan SVG, serta rendering teks dari `EncodeTerminal`.
- **Inside the symbol.** Simbol yang sama dibedah dengan `Inspect`: peta modul yang diwarnai menurut perannya, dengan simbol sebelum masking dan pola mask sebagai lapisan tambahan, dan setiap modul data terhubung ke bit codeword yang dibawanya; segmen dan anggaran bit; codeword yang dikelompokkan per blok Reed-Solomon; serta setiap kandidat mask beserta skornya, yang mana pun bisa dipaksakan dengan satu klik.
- **Scan.** Gambar yang dipilih, di-drop, atau di-paste, dibaca dengan `DecodeAll` (dan `Decode` sebagai cadangan), dengan outline dan pusat finder tiap simbol digambar di atas gambarnya; set structured append yang dirangkai ulang dari beberapa gambar; dan kamera.

## 3. Batasan, dan apa yang diputuskannya

- **GitHub Pages hanya menyajikan file statis.** Tidak ada server yang bisa menjalankan library, jadi browser yang harus menjalankannya: target `js/wasm` milik Go meng-compile library dan sebuah bridge kecil menjadi satu modul. Toolchain standar sudah cukup; TinyGo memang menghasilkan file lebih kecil, tetapi menambah compiler kedua dengan kekurangannya sendiri di standard library.
- **`/docs` sudah terpakai.** Pages bisa menerbitkan root sebuah branch atau folder `/docs`-nya, dan `/docs` berisi plan dan catatan teori. Karena itu halaman diterbitkan oleh sebuah workflow (`actions/deploy-pages`), yang sekaligus mem-build modul WebAssembly, sehingga tidak ada binary yang di-commit.
- **Tanpa toolchain JavaScript.** HTML, CSS, dan ES module polos, dengan compiler Go sebagai satu-satunya langkah build.
- **Main thread yang sibuk membekukan halaman.** Runtime WebAssembly milik Go berjalan di satu thread, dan satu frame kamera atau simbol versi 40 butuh waktu yang terasa, jadi modulnya berjalan di Web Worker dan halaman berkomunikasi dengannya lewat pesan.
- **Tampilan edukatif butuh data internal.** `Matrix` hanya mengembalikan grid akhir. Peran tiap modul, grid sebelum masking, bit stream di tiap modul data, dan skor mask ada di dalam encoder, jadi semuanya butuh API publik, yang diputuskan bersama pemilik repo sebelum implementasi (bagian 11).

## 4. Pendekatan, berpijak pada kode

- **Bisa dipakai apa adanya:** setiap encoder dan decoder publik, payload builder, `EncodeStructuredAppend` dan `DecodeStructuredAppend`, serta himpunan option, yang dipetakan satu lawan satu oleh halaman.
- **Baru di library:** `Inspect`, yang dibangun di atas pipeline encoder itu sendiri. Setiap fungsi penempatan memberi label peran pada modul yang digambarnya saat matriks sedang di-trace, `placeBits` mencatat bit stream yang ditaruhnya di tiap modul data, dan `Inspect` menjalankan `buildMatrix` dengan tracing aktif, lalu menyusun ulang pesannya (segmen, data codeword, blok) untuk simbol yang sudah terpilih.
- **Asumsi tersembunyi yang perlu dihindari:** area format QR yang di-reserve ikut mencakup modul timing (6, 8) dan (8, 6), yang harus tetap berlabel timing; pada simbol R7, corner pattern kiri-bawah jatuh di tepi bawah finder; Micro QR M1 dan M3 mengakhiri datanya dengan codeword 4 bit, yang menggeser semua codeword sesudahnya sebanyak empat bit di stream; remainder bit tidak membawa codeword; mask yang dipaksakan melewati penilaian encoder; dan detektor multi-simbol di decoder lambat pada frame berisi noise halus (bagian 9).

## 5. Keputusan desain

- **`Inspect(text, opts...) (*Inspection, error)`.** Menerima semua option yang diterima `Matrix` dan gagal persis seperti `Matrix` gagal. `Inspection` melaporkan simbologi, versi, level (dengan `DetectionOnly` untuk M1), mask dan skor setiap kandidat (`MaskScores`, ditambah `MaskPenalties` berisi empat aturan QR), segmen beserta lebar field-nya, jumlah bit data, terminator, dan kapasitas, blok Reed-Solomon, codeword dalam urutan penempatan, dan lapisan per modul: `Modules` (sama dengan `Matrix`), `Unmasked`, `MaskPattern`, `Roles`, `ModuleCodeword`, dan `ModuleBit`.
- **`ModuleRole`.** Data, remainder, finder, separator, timing, alignment, format, version, dark module, finder sub-pattern, dan corner pattern; `RoleData` adalah zero value, dan `IsFunction` membedakan modul fungsi dari modul pesan.
- **Tracing tidak membebani encoder.** Trace adalah pointer yang hanya diisi oleh `Inspect`; setiap encoder membangun matriks tanpa trace, dan sebuah test memastikannya.
- **Skor mask selalu dihitung.** Skor untuk mask yang dipaksakan adalah yang akan ditimbang encoder, sehingga halaman bisa menunjukkan kenapa pilihan otomatisnya berbeda.
- **Bridge JSON.** `web/internal/bridge` memetakan delapan operasi (encode, inspect, masks, payload, structuredAppend, decode, decodePixels, decodeStructuredAppend) dari request JSON dan blob biner ke library lalu kembali lagi. `web/wasm` mendaftarkan satu fungsi JavaScript yang meneruskan panggilan ke sana. Bridge-nya adalah Go biasa, dites secara native dan di bawah WebAssembly.
- **Frame kamera.** Halaman mengambil kotak tengah dari tiap frame, memperkecilnya ke paling besar 480 piksel, mengubahnya ke abu-abu dengan bobot `color.GrayModel`, dan mengirim satu frame setiap kali.
- **Deployment.** `.github/workflows/pages.yml` mem-build dan menerbitkan halaman setiap ada push ke `master`, yang hanya menerima merge rilis, dan bisa juga dijalankan manual.
- **Tata letak.** Semua untuk halaman ini ada di `web/`: `site/` (halamannya), `wasm/` (entry point), `internal/bridge/`, dan `build.sh`.

## 6. Milestone

Milestone mendarat berurutan. **Checkpoint A** (setelah WD5) adalah halaman yang berfungsi di browser. **Checkpoint B** (WD6) adalah rilis `v0.18.0`.

### WD1 — Plan Doc `(S)`

- [x] Dokumen ini dan padanan bahasa Inggrisnya.

### WD2 — Dokumen Referensi `(S)`

- [x] `docs/theory/29-symbol-inspection.md` (EN + ID): lapisan-lapisan sebuah simbol dan sensus tiap peran, cara tracing mencatat peran dan bit stream, pemetaan bit stream ke codeword, grid sebelum masking dan skor mask, anggaran bit pesan, lokasi codeword di bloknya, dan menjalankan library di browser. Ditambah entri di README theory dan baris peta kodenya.

### WD3 — Inspect `(M)`

- [x] `ModuleRole`, `Inspection`, `InspectSegment`, `InspectBlock`, dan `Inspect`, dengan tracing di kode penempatan QR, Micro QR, dan rMQR.
- [x] Test: `Modules` sama dengan `Matrix` di semua simbologi dan option; setiap modul data membawa bit `Codewords` yang diklaimnya; sensus peran hasil hitungan manual untuk QR versi 1 dan 7, Micro M1, serta rMQR R7x43 dan R11x27; contoh kerja di dokumen 10 dan 28; skor mask yang cocok dengan pilihan encoder; lebar field; half codeword M1; dan error yang identik dengan error `Matrix`.

### WD4 — Bridge WebAssembly `(M)`

- [x] `web/internal/bridge` dengan delapan operasinya, `web/wasm`, dan `web/build.sh`.
- [x] Test: setiap setting sampai ke library, lokasi codeword cocok dengan bloknya, varian mask cocok dengan `Matrix`, payload cocok dengan builder-nya, structured append bisa round-trip, file gambar dan piksel mentah ter-decode lengkap dengan outline, dan konversi abu-abu cocok dengan `color.GrayModel`; test yang sama juga lolos di bawah WebAssembly di Node. Benchmark untuk satu frame kamera, dengan dan tanpa simbol.

### WD5 — Halaman `(L)`

- [x] `web/site`: tampilan Create, Inside the symbol, dan Scan, worker-nya, serta tema terang dan gelap.
- [x] Cek sekali jalan, dilaporkan di commit: Chrome headless menjalankan setiap tampilan, termasuk kamera lewat perangkat video palsu, di kedua tema dan di lebar layar ponsel, tanpa error di console.

### Checkpoint A — halaman berfungsi di browser.

### WD6 — Pages & Rilis `(S)`

- [x] `.github/workflows/pages.yml`, langkah build WebAssembly di CI, dan `web/README.md`.
- [x] README (pengantar, tautan demo, tabel API, scope, roadmap), dan peta kode theory.
- [x] Entri `v0.18.0` di `CHANGELOG.md`, dibiarkan unstaged untuk release branch.
- [x] `go test -race ./...` bersih, gofmt bersih.

## 7. Strategi validasi

1. **`Inspect` sama dengan encoder.** Untuk setiap simbologi dan option yang mengubah encoding, `Modules` sama dengan keluaran `Matrix`.
2. **Lapisan-lapisannya menjelaskan simbol.** Setiap modul data, sebelum di-mask, memuat bit `Codewords` yang ditunjuk `ModuleCodeword` dan `ModuleBit`, setiap bit codeword muncul tepat sekali, modul remainder terang sebelum masking, dan modul fungsi tidak pernah di-mask.
3. **Fakta dari luar.** Codeword di theory doc 10 (`HELLO WORLD`, V1-M) dan dokumen 28 (dua stream R11x27-M, keduanya dari zint), serta jumlah modul yang dihitung manual dari tata letak di dokumen 5, 26, dan 28.
4. **Skor mask menjelaskan pilihannya.** Mask otomatis adalah penalty QR terendah atau skor Micro tertinggi, dan memaksakan sebuah mask tidak mengubah skornya.
5. **Bridge, dua kali.** Test native, dan test yang sama di-compile ke WebAssembly lalu dijalankan di Node.
6. **Halaman di browser.** Satu putaran Chrome headless melewati setiap tampilan, dengan kamera palsu, kedua tema, dan lebar layar ponsel.

## 8. Usulan Delta Tata Letak File

```
qrgen/
├── inspect.go             # baru — ModuleRole, Inspection, Inspect, tracing
├── inspect_test.go        # baru
├── matrix.go, micro_matrix.go, rmqr_matrix.go   # label peran dan bit stream saat di-trace
├── encode.go, micro_encode.go, options.go       # saklar tracing; helper microData
web/
├── README.md
├── build.sh               # meng-compile site/qrgen.wasm, menyalin site/wasm_exec.js
├── .gitignore             # dua file hasil generate
├── wasm/main.go           # entry point js/wasm
├── internal/bridge/       # delapan operasi, test, benchmark
└── site/                  # index.html, style.css, app.js, client.js, worker.js,
                           # create.js, inspect.js, scan.js, util.js
.github/workflows/pages.yml   # baru — build dan terbitkan ke GitHub Pages
.github/workflows/ci.yml      # langkah build WebAssembly
docs/
├── plan-web-demo.md / .id.md
└── theory/29-symbol-inspection.md / .id.md
```

## 9. Risiko & Catatan Teknis

- **Ukuran unduhan.** Modul WebAssembly berukuran sekitar 4,7 MB, atau 1,3 MB setelah dikompresi seperti yang dilakukan GitHub Pages. Modul ini dimuat sekali lalu di-cache oleh browser.
- **Noise lambat dipindai.** Satu frame 640 kali 480 berisi noise acak yang halus butuh sekitar 0,7 detik secara native dan 2,6 detik di WebAssembly, karena setiap bintik menjadi satu connected component dan pencarian finder membandingkan komponen bersarang secara berpasangan; frame realistis tanpa simbol butuh sekitar 30 ms secara native, dan frame dengan simbol sekitar 6 ms (20 ms di WebAssembly). Halaman membatasi biayanya dengan hanya memindai kotak tengah berukuran paling besar 480 piksel, satu frame setiap kali, di worker. Mempercepat detektor pada frame semacam itu adalah pekerjaan library untuk milestone berikutnya.
- **`wasm_exec.js` harus cocok dengan compiler-nya.** `build.sh` menyalinnya dari toolchain yang meng-compile modul, jadi keduanya tidak pernah berselisih versi.
- **Kamera butuh secure context.** GitHub Pages memakai HTTPS dan `localhost` dianggap aman; halaman memberi tahu kalau kamera tidak tersedia.
- **Pages perlu diaktifkan sekali.** Sumber Pages di repo harus diset ke "GitHub Actions" sebelum deployment pertama.

## 10. Referensi

- Go, `syscall/js` dan port `js/wasm` (`lib/wasm/wasm_exec.js`).
- GitHub Pages dengan workflow kustom: `actions/configure-pages`, `actions/upload-pages-artifact`, `actions/deploy-pages`.
- `docs/theory/05-matrix-construction.md`, `06-masking.md`, `04-reed-solomon.md`, `02-data-encoding.md`, `10-worked-example.md`, `26-micro-qr.md`, `28-rmqr.md` — apa yang ditampilkan oleh lapisan-lapisan inspeksi.

## 11. Pertanyaan Terbuka

- **Cakupan versi pertama.** Terjawab sebelum implementasi: keempat bagian (encoding lengkap, decoding gambar, kamera, dan tampilan edukatif).
- **Stack.** Terjawab: HTML, CSS, dan JavaScript polos tanpa langkah build.
- **Folder dan deployment.** Terjawab: `web/`, diterbitkan setiap ada push ke `master`.
- **Data untuk tampilan edukatif.** Terjawab: API publik `Inspect`, dirilis bersama demo, alih-alih menghitung ulang lapisan di JavaScript atau menunda tampilannya.
