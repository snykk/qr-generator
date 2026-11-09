# QR Generator — Rencana Deteksi rMQR

Dokumen ini menjelaskan rencana implementasi untuk **menemukan simbol rMQR Code di dalam gambar**, dengan target rilis minor `v0.19.0`. v0.17 merilis encoder rMQR dan decoder di tingkat matriks, sehingga grid rMQR yang bersih sudah bisa di-decode lewat `DecodeMatrix`; tetapi `Decode`, `DecodeBytes`, dan `DecodeAll` belum bisa menemukan simbol rMQR di sebuah foto, dan tampilan Scan di web demo belum bisa membacanya. Milestone ini menutup celah itu, seperti yang dilakukan v0.16 untuk Micro QR: satu finder pattern memberi orientasi dan format information, format information memberi ukuran, dan finder sub-pattern di sudut seberang menjadi jangkar untuk ujung lain dari simbol yang panjangnya bisa sampai 139 modul.

> Status: **draf / dokumen hidup.** Milestone RD1..RD7 mendarat bertahap di branch `rmqr-detection`; tiap milestone adalah commit fokus (atau deret kecil) berikut test, mengikuti irama milestone sebelumnya.

> Versi bahasa Inggris: [docs/plan-rmqr-detection.md](plan-rmqr-detection.md).

---

## 1. Visi & Tujuan

- `Decode`, `DecodeBytes`, `-decode` di CLI, dan `DecodeAll` membaca simbol rMQR di dalam gambar, di ke-32 versinya, dalam posisi tegak, terputar, maupun dengan perspektif sedang.
- Decoding QR dan Micro QR tetap persis seperti sekarang: rMQR baru dicari setelah keduanya mendapat giliran pada finder yang sama, dan semua test yang ada tetap lolos tanpa perubahan.
- `DecodeAll` melaporkan simbol rMQR di samping simbol QR dan Micro QR, lengkap dengan versinya.
- Memvalidasi terhadap materi yang **independen**, tidak pernah hanya terhadap diri sendiri: 70 simbol zint dari v0.17 yang dirender ke gambar, dan reader zxing-cpp pada scene yang sama.
- Mengukur rentang kerjanya (skala, rotasi, kemiringan) alih-alih mengasumsikannya, dan menyebut dengan jelas di mana batasnya.

## 2. Apa yang membuat deteksi rMQR berbeda

Tata letak rMQR dijelaskan di theory doc 28; berikut fakta-fakta yang diandalkan oleh deteksi.

- **Satu finder, seperti Micro QR.** Finder 7 kali 7 di kiri-atas, dengan separator terang di kolom 7 dan, mulai tinggi 9, di baris 7. Detektor connected component dari v0.13 sudah menemukannya sebagai cincin bersarang, dan pencarian Micro dari v0.16 sudah mengubah satu finder menjadi homografi sampai seperempat putaran.
- **Simbol yang panjang.** Lebarnya sampai 139 modul dan tingginya 7 sampai 17. Homografi yang ditetapkan oleh satu finder selebar 7 modul diekstrapolasi sampai 20 kali ukurannya sendiri, sehingga galat kecil di sudut finder menjadi beberapa modul di ujung jauh. Micro QR, yang paling besar 17 modul, sudah butuh refinement untuk alasan yang sama (dokumen 27, bagian 6); rMQR tidak bisa bekerja tanpa jangkar kedua.
- **Jangkar kedua: finder sub-pattern.** Pola 5 kali 5 di sudut kanan-bawah, berupa cincin gelap yang mengelilingi cincin terang yang mengelilingi satu modul gelap, bentuknya persis alignment pattern QR. Cincin gelapnya tidak punya separator dan menempel ke modul format dan data, tetapi cincin terangnya selalu menjadi komponen tertutup di sekeliling pusat gelap selebar satu modul.
- **Orientasi dan ukuran dari format information.** Baris 0 di samping finder berisi timing pattern bagian atas, gelap di kolom genap, yang menentukan seperempat putarannya. Salinan format 18 bit di samping finder, di baris 1 sampai 5 pada kolom 8 sampai 11, menyebut versinya sehingga kedua dimensinya diketahui; salinan kedua di samping sub-pattern menyebutnya lagi.
- **Tepi yang panjang dan lurus.** Baris teratas dan terbawah adalah timing pattern yang hanya diselingi alignment pattern dan corner pattern, sehingga kedua tepi panjangnya penuh dengan modul yang pasti gelap.

## 3. Prior art

zxing-cpp 2.3.0 membaca simbol rMQR di dalam gambar dengan `SampleRMQR` (core/src/qrcode/QRDetector.cpp), yang dibaca untuk menyusun rencana ini. Untuk setiap finder yang tidak dipakai simbol QR, ia mencoba keempat seperempat putaran, menyimpan putaran yang modul tepi atasnya, baris 0 pada kolom 8 sampai 11, terbaca gelap, terang, gelap, terang, membaca salinan format di sisi finder, dan memilih putaran dengan word valid terdekat. Setelah itu ia mencari sub-pattern dengan locator alignment pattern miliknya di sekitar titik yang diprediksi homografi finder, membangun transformasi empat titik dari finder dan sub-pattern, lalu mengambil sampel grid. Ia hanya membaca salinan format di sisi finder dan tidak melakukan refinement di sepanjang simbol; kodenya bertanda "TODO: this is a WIP". zxing-cpp menjadi reader pembanding kita, dan tes orientasi serta pembacaan format-nya sama dengan langkah awal rencana ini.

## 4. Pendekatan, berpijak pada kode

- **Bisa dipakai apa adanya:** dispatch binarisasi dan `labelComponents`; `allFinderQuads` dan sudut-sudut finder; dari v0.16, `outerRingCorners` (koreksi setengah piksel), `windCorners`, homografi finder per seperempat putaran, `moduleDark`, march titik tepi dan fit garis, serta `computeHomographyLS`; dari v0.17, posisi format rMQR, 64 format word, dan `decodeRMQRMatrix`; dan dari v0.14, claim loop milik `DecodeAll`.
- **Baru:** gate orientasi dan format rMQR; kandidat sub-pattern dari komponen terang yang sudah dihasilkan `labelComponents`; pemasangan finder dengan sub-pattern berdasarkan posisi prediksi dan ukuran modul; homografi delapan titik; edge fit di sepanjang tepi panjang; sampling grid persegi panjang; salinan format kedua; error contract berbasis stage; dan `Result.RMQRVersion`.
- **Asumsi tersembunyi yang perlu dihindari:** `sampleMatrix` hanya mengambil sampel bujur sangkar; `checkAlignmentAt` mengambil sampel di sepanjang sumbu gambar dan gagal pada pola yang terputar 45 derajat; gate timing Micro mengharapkan timing di kolom 0, yang tidak dimiliki simbol R7; alignment pattern QR terlihat persis seperti sub-pattern, sehingga kandidatnya banyak; simbol Micro lolos tes orientasi rMQR, karena baris 0-nya juga timing; dan format word yang berjarak 3 bit dari salah satu dari 64 word valid adalah bukti yang lemah (kira-kira satu dari empat word acak memenuhinya).

## 5. Keputusan desain

- **Urutan.** Pada setiap binarisasi, triple QR proyektif lebih dulu, lalu pencarian Micro, lalu pencarian rMQR, semuanya pada finder quad yang sama; di `DecodeAll`, triple QR, lalu Micro, lalu rMQR pada finder yang belum diklaim. rMQR tidak pernah mendahului simbol yang bisa di-decode oleh pencarian sebelumnya.
- **Gate.** Sebuah seperempat putaran lolos kalau modul tepi atasnya, baris 0 pada kolom 7 sampai 11, terbaca terang, gelap, terang, gelap, terang: separator, lalu awal timing pattern. Salinan format di sisi finder milik putaran yang lolos, yang berjarak paling banyak 3 bit dari salah satu dari 64 word, menyebut versinya; sebuah kandidat sub-pattern kemudian harus berada di tempat yang ditentukan versi itu, dengan toleransi yang membesar sesuai jarak, dan dengan ukuran modul yang dekat dengan ukuran modul finder.
- **Geometri.** Homografi least-squares atas empat sudut luar finder dan empat sudut luar cincin terang sub-pattern, lalu edge fit di tepi atas dan bawah (dan di tepi kiri dan kanan yang pendek); varian-varian dicoba dari yang terbaik dan Reed-Solomon yang memutuskan, seperti di v0.16.
- **Kedua salinan format.** Begitu simbolnya ditemukan, salinan di sisi sub-pattern harus cocok dengan versinya; saat salinan di sisi finder tidak terbaca, setiap versi yang posisi sub-pattern-nya berisi kandidat dicoba dengan salinan di sisi sub-pattern, sehingga satu salinan saja sudah cukup, sama seperti pada `DecodeMatrix`.
- **Error.** Seperti untuk Micro QR: pencarian mencatat sejauh mana tiap finder lolos (terorientasi, format terbaca, ditemukan), dan hanya simbol rMQR yang sudah ditemukan, yaitu finder beserta sub-pattern yang cocok, yang boleh menggantikan "finder patterns not found" dengan error-nya sendiri.
- **API.** `Result` mendapat `RMQRVersion`, bernilai nol untuk QR dan Micro QR; hasil rMQR punya `Version` dan `MicroVersion` nol, outline di `Corners`, dan finder-nya di `Finders[0]`.
- **Web demo** melaporkan simbol rMQR di tampilan Scan tanpa perubahan selain labelnya.

## 6. Milestone

Milestone mendarat berurutan. **Checkpoint A** (setelah RD4) membaca setiap simbol zint di gambar bersih, terputar, dan miring. **Checkpoint B** (RD7) adalah rilis `v0.19.0`.

### RD1 — Plan Doc `(S)`

- [x] Dokumen ini dan padanan bahasa Inggrisnya.

### RD2 — Dokumen Referensi `(S)`

- [ ] `docs/theory/30-rmqr-detection.md` (EN + ID): apa yang belum ditentukan satu finder pada simbol yang panjang; orientasi dari timing di tepi atas; salinan format di sisi finder dan seberapa lemah sebuah kecocokan; ciri sub-pattern dan kenapa cincin terangnya, bukan cincin gelapnya, yang dicari; pemasangan berdasarkan posisi prediksi; homografi delapan titik dan edge fit; salinan format kedua; tempat pencarian ini bergabung ke pipeline; dan zxing-cpp sebagai prior art. Ditambah entri di README theory dan baris peta kodenya.

### RD3 — Menemukan Simbol `(L)`

- [ ] Gate orientasi dan format, kandidat sub-pattern, pemasangan, homografi delapan titik, sampling persegi panjang, dan decoder untuk satu finder quad.
- [ ] Test: setiap simbol zint, dirender tegak di beberapa skala dan di setiap seperempat putaran, ter-decode lewat pencarian baru ini.

### RD4 — Refinement dan Konfirmasi `(M)`

- [ ] Edge fit dan varian-variannya; salinan format di sisi sub-pattern sebagai konfirmasi dan cadangan; stage-stage-nya.
- [ ] Test: rentang rotasi dan kemiringan, salinan format sisi finder yang rusak, dan apa yang ditambahkan tiap refinement.

### Checkpoint A — setiap simbol rMQR zint terbaca dari gambar yang terputar dan miring.

### RD5 — Pipeline `(M)`

- [ ] Pass rMQR di `Decode` dan `DecodeBytes` setelah pass Micro, beserta error contract-nya; pass rMQR di `DecodeAll` dan `Result.RMQRVersion`; `examples/decode/rmqr`.
- [ ] Test: komposit simbol QR, Micro QR, dan rMQR; tidak ada finder QR atau Micro yang ter-decode sebagai rMQR, dan tidak ada simbol rMQR yang ter-decode sebagai Micro QR.

### RD6 — Pengukuran dan Web Demo `(M)`

- [ ] Sweep skala, rotasi, kemiringan, dan quiet zone atas simbol-simbol zint, serta zxing-cpp pada scene yang sama, dilaporkan di commit.
- [ ] Tampilan Scan di web demo memberi label simbol rMQR; satu putaran browser dengan simbol rMQR lewat file dan kamera.

### RD7 — Polish & Rilis `(S)`

- [ ] README (bagian decoding, tabel API, scope, limitations, roadmap) dan peta kode theory.
- [ ] GitHub Actions dinaikkan ke rilis Node 24-nya (checkout, setup-go, configure-pages, upload-pages-artifact, deploy-pages), dengan run yang bebas dari peringatan Node 20.
- [ ] Entri `v0.19.0` di `CHANGELOG.md`, dibiarkan unstaged untuk release branch.
- [ ] `go test -race ./...` bersih, gofmt bersih.

## 7. Strategi validasi

1. **Simbol zint di dalam gambar.** Ke-70 fixture rMQR dari v0.17, dirender di beberapa skala, seperempat putaran, rotasi bidang, kemiringan perspektif, dan quiet zone, ter-decode menjadi teksnya.
2. **Reader independen.** zxing-cpp membaca scene yang sama di lingkungan scratch; kita bandingkan siapa membaca apa, dan tidak pernah menerima teks yang berbeda.
3. **Tanpa false positive.** Tidak ada finder simbol QR atau Micro QR yang ter-decode sebagai rMQR, tidak ada simbol rMQR yang ter-decode sebagai Micro QR, dan komposit melaporkan setiap simbol sekali, sebagai jenisnya sendiri.
4. **Refinement diukur.** Setiap langkah geometri harus membaca paling tidak sebanyak scene yang dibaca langkah sebelumnya.
5. **Kerusakan.** Simbol yang salinan format sisi finder-nya hancur tetap terbaca lewat salinan yang lain.

## 8. Usulan Delta Tata Letak File

```
qrgen/
├── decode_rmqr.go            # baru — orientasi, format, sub-pattern, homografi, refinement
├── decode_rmqr_test.go       # baru — golden di gambar, rentang, kerusakan, false positive
├── decode_image.go           # pass rMQR setelah pass Micro; sampling persegi panjang
├── decode_multi.go           # pass rMQR di DecodeAll; Result.RMQRVersion
├── decode_finder_cc.go       # kandidat sub-pattern dari komponen terang
web/internal/bridge/bridge.go # hasil rMQR di tampilan Scan
web/site/                     # label
examples/decode/rmqr/main.go  # baru
.github/workflows/*.yml       # rilis action Node 24
docs/
├── plan-rmqr-detection.md / .id.md
└── theory/30-rmqr-detection.md / .id.md
```

## 9. Risiko & Catatan Teknis

- **Ekstrapolasi di simbol yang panjang.** Finder saja tidak bisa menempatkan ujung jauhnya; toleransi pemasangan harus membesar sesuai jarak, dan galat yang terukur yang menentukan seberapa besar.
- **Banyak yang mirip.** Alignment pattern QR berbagi bentuk dengan sub-pattern, jadi sebuah kandidat hanya diterima di tempat yang ditentukan versinya, dengan ukuran modul yang cocok, dan Reed-Solomon yang punya kata akhir.
- **Micro dan rMQR berbagi gate pertama.** Simbol Micro lolos tes orientasi rMQR dan simbol rMQR setinggi 9 bisa lolos tes milik Micro, jadi urutan pencarian dan Reed-Solomon yang memisahkan keduanya; test memeriksa kedua arah.
- **Sub-pattern yang kecil.** Pusat sub-pattern hanya satu modul, sehingga di skala yang sangat kecil cincin terangnya mungkin tidak terpisah; batas skala di bawah mana rMQR berhenti terbaca diukur dan didokumentasikan, tidak disembunyikan.
- **QR dan Micro tidak boleh berubah.** Pass rMQR berjalan paling akhir, sehingga hasil decode keduanya tidak bisa berubah; suite yang ada menjaganya.

## 10. Referensi

- ISO/IEC 23941:2022 — tata letak rMQR, sebagaimana dikutip lewat source zint dan zxing-cpp (theory doc 28).
- zxing-cpp 2.3.0, core/src/qrcode/QRDetector.cpp (`SampleRMQR`, `DetectPureRMQR`) dan QRReader.cpp — prior art, dan reader independen untuk validasi.
- `docs/theory/24-arbitrary-angle-detection.md`, `25-multi-symbol-decoding.md`, `27-micro-qr-detection.md`, `28-rmqr.md` — detektor, claim loop, pencarian satu finder, dan simbol yang menjadi dasar milestone ini.

## 11. Pertanyaan Terbuka

- **Toleransi pemasangan.** Seberapa jauh prediksi sub-pattern dari homografi finder melenceng, menurut panjang, skala, dan rotasi, diukur di RD3 dan menentukan toleransinya.
- **Simbol kecil.** Apakah simbol R7 atau R9, yang sub-pattern-nya dekat dengan finder, butuh edge fit sama sekali, atau cukup homografi delapan titik.
