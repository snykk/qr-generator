# Deteksi Micro QR

Dokumen 26 menjelaskan simbol Micro QR dan bagaimana v0.15 men-decode grid Micro yang bersih. Dokumen ini membahas langkah sebelum itu: menemukan grid tersebut di dalam gambar. Pipeline QR menemukan simbol lewat tiga finder pattern, sedangkan simbol Micro hanya punya satu. Bagian-bagian berikut menunjukkan apa yang bisa ditentukan satu finder dengan sendirinya, apa yang masih terbuka, bagaimana timing pattern dan format information menutup celah itu, dan bagaimana pencarian ini masuk ke pipeline yang ada tanpa mengusik decoding QR.

> Versi bahasa Inggris: [27-micro-qr-detection.md](27-micro-qr-detection.md).

## 1. Kenapa pipeline QR tidak bisa memulai

Kedua detektor QR dibangun di atas sebuah segitiga. Jalur afin (dokumen 12 dan 15) memindai run 1:1:3:1:1, menyimpan tiga cluster terkuat, lalu mengurutkannya: vertex bersudut siku adalah finder kiri-atas, cross product membedakan kanan-atas dari kiri-bawah, jarak antar-finder memberi version, dan tiga pusat plus sudut keempat hasil melengkapi parallelogram memberi homografi. Jalur projektif (dokumen 24) menemukan finder sebagai connected component yang bersarang, mengekstrak sudut-sudutnya, lalu mencocokkan homografi ke dua belas sudut dari triple itu. Keduanya menolak gambar yang finder-nya kurang dari tiga, dan satu finder tidak membentuk segitiga yang bisa dibaca.

Micro QR hanya mempertahankan satu finder karena simbol kecil tidak punya ruang untuk tiga (dokumen 26, bagian 1). Yang membuat satu finder sudah cukup adalah semua hal lain yang diletakkan standar di sebelahnya.

## 2. Apa yang ditentukan empat sudut, dan apa yang masih terbuka

Detektor connected-component sudah mengembalikan setiap finder sebagai quad: pusatnya dan empat sudut luar cincin gelapnya (dokumen 24, bagian 3 dan 4). Dalam koordinat modul, dengan pusat modul di bilangan bulat, sudut-sudut itu terletak di `(-0.5, -0.5)`, `(6.5, -0.5)`, `(6.5, 6.5)`, dan `(-0.5, 6.5)`. Homografi projektif punya delapan derajat kebebasan, dan empat korespondensi dalam posisi umum menentukannya secara pasti, jadi satu finder sudah mengunci seluruh pemetaan antara bidang simbol dan gambar, termasuk perspektifnya.

Yang tidak dikatakan sudut-sudut itu adalah sudut mana yang mana. Finder terdiri dari tiga persegi konsentris: ia tampak sama setelah diputar seperempat putaran maupun di cermin, dan sudut-sudutnya keluar dari detektor tanpa label. Karena itu homografinya hanya diketahui sampai simetri-simetri tersebut. Winding (arah urutan sudut) menghilangkan separuh yang berupa cerminan. Untuk simbol yang dilihat dari depan, sudut-sudut dengan urutan kiri-atas, kanan-atas, kanan-bawah, kiri-bawah melingkupi luas bertanda positif dalam koordinat gambar (y bertambah ke bawah), sama seperti sudut-sudut modulnya, karena homografi yang tidak mencerminkan mempertahankan orientasi. Menyusun keempat sudut hasil ekstraksi dalam winding itu menyisakan empat kemungkinan penetapan, yaitu empat pergeseran siklis, satu untuk tiap seperempat putaran. Bayangan cermin, yang muncul bila simbol dilihat dari balik lembaran transparan, punya winding yang berlawanan dan tidak dibaca, sama seperti untuk QR (dokumen 15).

## 3. Orientasi dari timing pattern

Di sekitar finder simbol Micro, dua sisi yang membelakangi data, atas dan kiri, berbatasan dengan quiet zone. Dua sisi yang menghadap data diikuti separator, dan timing pattern dimulai tepat setelahnya, di baris 0 dan kolom 0, gelap pada indeks genap (dokumen 26, bagian 6). Karena itu modul (baris 0, kolom 8) dan (baris 8, kolom 0) gelap di setiap versi Micro.

Membaca kedua modul itu lewat setiap kandidat homografi menjawab soal seperempat putaran. Pada penetapan yang benar, keduanya adalah modul timing. Pada setiap penetapan yang salah, setidaknya satu dari keduanya jatuh dua modul ke luar, di dalam quiet zone, yang menurut standar lebarnya minimal dua modul:

| Sudut finder yang dianggap sudut luar simbol | (baris 0, kolom 8) jatuh di | (baris 8, kolom 0) jatuh di |
|---|---|---|
| Kiri-atas (benar) | timing, gelap | timing, gelap |
| Kanan-atas | modul format (baris 8, kolom 6) | quiet zone (baris 0, kolom -2) |
| Kanan-bawah | quiet zone (baris 6, kolom -2) | quiet zone (baris -2, kolom 6) |
| Kiri-bawah | quiet zone (baris -2, kolom 0) | modul format (baris 6, kolom 8) |

Pada gambar yang bersih, tepat satu penetapan yang lolos. Modul separator (baris 0, kolom 7) dan (baris 7, kolom 0) tidak menambah apa-apa pada tes ini: pada penetapan kanan-atas, misalnya, keduanya jatuh di baris separator dan di quiet zone, sama-sama terang, persis seperti pada penetapan yang benar. Detektor zxing-cpp memakai tes dua modul yang sama (bagian 9).

## 4. Membaca format information sebelum ukuran diketahui

Kelima belas modul format, kolom 8 baris 1 sampai 8 dan baris 8 kolom 1 sampai 7, semuanya terletak di blok 9 kali 9 pada sudut finder, di tempat yang sama untuk setiap versi (dokumen 26, bagian 7). Karena itu modul-modul ini bisa dibaca lewat homografi finder selagi ukuran simbol belum diketahui, lalu di-decode terhadap ke-32 format word Micro, bukan hanya empat sampai dua belas word milik satu versi yang dipertimbangkan `readMicroFormatInfo` setelah ukuran grid diketahui. Word terdekat yang masih dalam budget BCH(15,5) sebesar tiga error memberi nomor simbol, dan dari situ version, level error correction, dan mask. Kalau lebih dari satu penetapan lolos tes timing, seperti yang bisa terjadi pada gambar yang ramai, penetapan yang format word-nya paling dekat yang menang.

Format word yang valid adalah bukti yang lebih lemah dari kelihatannya. Kode ini punya 32 word dengan jarak minimum 7, jadi bola berjari-jari 3 di sekeliling tiap word tidak saling beririsan, masing-masing berisi 1 + 15 + 105 + 455 = 576 word: 18.432 dari 32.768 kemungkinan word 15 bit, atau 56 persen, ter-decode menjadi salah satu simbol Micro dalam budget. Susunan modul yang acak lebih sering lolos cek format daripada tidak. Format word mengidentifikasi simbolnya setelah kita tahu sedang melihat sebuah simbol; ia tidak bisa menjadi penentu bahwa memang ada simbol di sana. Strip yang seluruhnya terang, yang sudah diperingatkan dokumen 26, adalah salah satu contohnya: dengan tiga bit bernilai satu, M4-L dengan mask 1 (`0x1202`) masih dalam budget darinya.

## 5. Cek silang ukuran lewat timing pattern

Ukuran yang diklaim format word bisa dicek terhadap simbol itu sendiri. Di sepanjang baris 0, simbol Micro berukuran n terbaca begini: separator terang di indeks 7, modul timing dari indeks 8 sampai n - 1 yang berselang-seling mulai dari gelap dan berakhir gelap karena n - 1 genap, lalu quiet zone di n dan n + 1. Kolom 0 terbaca sama. Totalnya 2(n - 5) modul, dari 12 di M1 sampai 24 di M4, yang nilai harapannya hanya bergantung pada n.

Cek silang menghitung ketidakcocokan pada modul-modul itu dan menerima sedikit di antaranya. Tugas utamanya adalah menolak finder yang sama sekali bukan finder Micro. Finder di sudut simbol QR punya bentuk yang persis sama, dan separatornya ada di tempat yang sama dengan separator Micro; tetapi di luar separator, baris 0 simbol QR berisi format information dan data, bukan timing, sehingga kira-kira separuh modul yang dicek tidak cocok. Kedua modul quiet zone diperlukan, bukan hanya yang pertama: indeks timing ganjil sebuah simbol bernilai terang, jadi ukuran klaim yang dua lebih kecil dari ukuran sebenarnya akan menemukan modul terang di indeks n dan lolos, sedangkan indeks n + 1 jatuh di modul timing yang gelap dan menangkapnya.

Ukuran klaim yang meleset satu versi hanya menghasilkan satu ketidakcocokan per tepi, yang mungkin masih diterima toleransi, jadi cek silang tidak selalu bisa membedakan ukuran yang bertetangga dengan sendirinya. Itu tidak perlu. Ukuran yang salah membutuhkan setidaknya empat error pada format word yang dilindungi BCH, dan cek Reed-Solomon pada grid yang disampel dengan ukuran yang salah akan gagal.

## 6. Ekstrapolasi, dan mencari ulang sudut-sudut jauh

Untuk gambar yang sempurna, homografi finder bersifat eksak: simbolnya planar, dan empat korespondensi yang eksak menentukan homografi seluruh bidang. Sudut-sudut yang nyata tidak eksak. Mereka berasal dari piksel ekstrem sebuah cincin hasil binarisasi, jadi membawa noise piksel, dan pusat sebuah piksel terletak setengah piksel di dalam sudut luar cincin yang sebenarnya. Error itu membesar seiring jarak dari titik-titik yang menentukan transformasi. Error skala di sudut-sudut finder, 3,5 modul dari pusatnya, membesar sebanding dengan jarak: sudut yang meleset sepersepuluh modul menjadi hampir empat persepuluh modul di modul kanan-bawah simbol M4, yang berjarak 13 modul dari pusat finder pada tiap sumbu. Di bawah perspektif keadaannya lebih buruk, karena suku-suku foreshortening dari homografi ditentukan oleh persegi kecil lalu diekstrapolasi ke persegi besar. Sampling membaca setiap modul di pusat prediksinya, jadi seluruh budget-nya hanya setengah modul.

Solusinya, kalau memang diperlukan, adalah menemukan titik-titik yang jauh dari finder lalu melakukan fit ulang. Standar menyediakannya. Modul timing terakhir di tiap tepi, di indeks n - 1, selalu gelap dan dibatasi quiet zone di dua sisi, sehingga sudut kanan-atas dan kiri-bawah simbol bisa ditemukan sebagai piksel gelap terluar di dekat posisi prediksinya. Selain itu, aturan masking Micro memilih mask yang memaksimalkan modul gelap di baris terakhir dan kolom terakhir (dokumen 26, bagian 9), sehingga batas kanan-bawah tetap terlihat oleh reader. zxing-cpp mencari ulang ketiga sudut jauh sebelum menyampel. Bagian 10 mencatat mana di antaranya yang ternyata dibutuhkan library ini.

## 7. False positive dan gerbang-gerbang di depannya

Sebuah finder baru diterima sebagai simbol Micro setelah lolos, berurutan: tes timing di bagian 3, cek format di bagian 4, cek silang di bagian 5, dan decode penuh atas grid hasil sampel oleh `decodeMicroMatrix`, termasuk Reed-Solomon. Setiap gerbang menangkap apa yang diloloskan gerbang lain. Untuk finder yang sekelilingnya bukan simbol Micro, tes timing lolos kira-kira satu dari empat kali dan cek format kira-kira satu dari dua. Cek silang membandingkan 12 sampai 24 modul, jadi dengan toleransi dua ketidakcocokan, lingkungan yang acak lolos kira-kira satu dari lima puluh kali untuk M1 dan jauh lebih jarang untuk versi yang lebih besar.

Reed-Solomon adalah gerbang terakhir. Grid yang sepenuhnya acak lolos kira-kira satu dari 25.000 kali untuk M3-L, yang enam EC codeword-nya mengoreksi sampai tiga error, dan satu dari 65.536 kali untuk M1, yang dua EC codeword-nya hanya mendeteksi error; setiap tipe lain lolos kurang dari satu dari 140.000 kali, sampai sekitar satu dari 200 miliar untuk M4-Q. Parser segmen kemudian menolak banyak dari yang tersisa. Jika semua gerbang digabung, M1 adalah kasus terlemah, karena cek silangnya paling pendek, dan kasus itulah yang menjadi sasaran test negatif.

Urutan sama pentingnya dengan gerbang. Pencarian Micro hanya berjalan setelah deteksi QR gagal pada gambar hasil binarisasi yang sama, jadi sudut simbol QR yang terbaca tidak pernah ditawarkan kepadanya.

## 8. Di mana pencarian ini masuk

`Decode` menjalankan pipeline QR afin lebih dulu, tanpa perubahan (dokumen 24, bagian 8). Ketika itu gagal, percobaan connected-component me-label gambar hasil binarisasi sekali dan memakai finder quad-nya dua kali: pertama triple QR terbaik, lalu, kalau itu tidak ter-decode, setiap quad sebagai finder Micro, dari yang terbesar, dan yang pertama ter-decode yang menang. Ini dilakukan pada binarisasi Otsu dan, bila perlu, sekali lagi pada binarisasi Sauvola (dokumen 14). Karena labelling-nya dipakai bersama, gambar Micro yang bersih ditemukan pada binarisasi pertama tanpa pass tambahan.

Saat gagal total, `Decode` mengembalikan error dari percobaan afin, seperti sebelumnya, sehingga error untuk gambar QR tidak berubah. Satu-satunya pengecualian adalah gambar yang percobaan afinnya tidak menemukan triple finder tetapi punya finder yang lolos tes timing Micro: di situ error dari percobaan Micro yang dikembalikan, karena "format information tak terbaca" atau "terlalu banyak error" menggambarkan simbol Micro yang rusak lebih tepat daripada "finder pattern tidak ditemukan".

`DecodeAll` melakukan hal yang sama di dalam claim loop-nya (dokumen 25): setelah triple-triple QR mengklaim finder-nya, setiap quad yang belum diklaim dicoba sebagai finder Micro dan diklaim kalau berhasil di-decode. Hasil Micro melaporkan version-nya di field tersendiri, karena `Version` adalah tipe version QR.

## 9. Prior art

zxing-cpp menemukan simbol Micro dengan garis besar yang sama (core/src/qrcode/QRDetector.cpp, `SampleMQR`, dibaca di branch master-nya pada November 2026). Ia mencoba keempat rotasi quad sudut sebuah finder, mensyaratkan dua modul timing terdalam yang sama bernilai gelap, men-decode bit format terhadap ke-32 word dalam dua urutan bit (yang kedua untuk simbol yang di-mirror), menyimpan rotasi dengan jarak Hamming terkecil, dan mengambil version dari nomor simbol. Alih-alih cek silang timing, ia menyampel baris n dan kolom n tepat di luar simbol dan menolak kandidat kalau lebih dari sepertiga sampel itu gelap, sebagai pengaman terhadap sudut simbol QR. Sebelum menyampel, ia mencari ulang sudut kanan-atas, kiri-bawah, dan kanan-bawah lewat pencarian lokal. Reader-nya mencoba setiap finder pattern yang tidak dipakai simbol QR yang sudah ter-decode, setelah tahap QR. Quad sudutnya terletak di tengah cincin luar finder, di pusat modul-modul sudut, bukan di tepi luar cincin.

## 10. Validasi dan batasan

Sama seperti decoding pada sudut sembarang (dokumen 24, bagian 9), tidak ada tabel untuk mengecek sebuah detektor, jadi ia divalidasi secara empiris dan terhadap implementasi independen. Simbol-simbol zint yang tersimpan sebagai teks di `qrgen/testdata/thirdparty/micro_symbols.txt` dirender menjadi gambar pada beberapa ukuran modul, quiet zone, dan rotasi, dan masing-masing harus ter-decode menjadi teksnya; simbol-simbol itu berasal dari encoder yang tidak berbagi kode dengan library ini. Reader zxing-cpp dijalankan sekali pada scene sintetis yang sama, untuk memastikan detektor independen bisa membacanya dan untuk mengungkap scene yang bisa ia baca tetapi tidak bisa dibaca library ini. Kasus negatif mencakup gambar kosong, finder pattern tunggal, dan sudut-sudut simbol QR. Rentang rotasi, tilt, dan skala hasil pengukuran, beserta penyempurnaan yang ternyata diperlukan, dicatat di sini setelah milestone ketahanan (MD5) mengukurnya.

## 11. Penunjuk implementasi

Direncanakan di MD3 sampai MD6 (lihat `docs/plan-micro-qr-detection.md`):

- `qrgen/decode_micro.go`: winding sudut, homografi untuk tiap penetapan, tes timing, pembacaan format sisi gambar atas ke-32 word, cek silang timing, dan `decodeMicroQuad`.
- `qrgen/decode_finder_cc.go`: `allFinderQuads`, setiap finder quad tanpa jumlah minimum.
- `qrgen/decode_image.go`: tahap Micro setelah triple QR pada tiap binarisasi, dan kontrak error.
- `qrgen/decode_multi.go`: tahap Micro atas quad yang belum diklaim di `DecodeAll`, dan `Result.MicroVersion`.

## Referensi

- ISO/IEC 18004:2015 — tata letak Micro QR, timing pattern, format information, quiet zone, dan aturan masking.
- zxing-cpp, core/src/qrcode/QRDetector.cpp (`SampleMQR`), QRReader.cpp, dan QRFormatInformation.cpp (`DecodeMQR`) — prior art untuk deteksi satu finder, dan reader independen yang dipakai untuk validasi.
- `docs/theory/24-arbitrary-angle-detection.md` — detektor connected-component, ekstraksi sudut, dan homografi least-squares yang dipakai ulang di sini.
- `docs/theory/25-multi-symbol-decoding.md` — claim loop yang diikuti tahap Micro.
- `docs/theory/26-micro-qr.md` — simbol Micro itu sendiri: tata letak, format information, masking, dan decoder matriks yang menerima hasil detektor ini.
