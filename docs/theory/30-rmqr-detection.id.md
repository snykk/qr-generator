# Deteksi rMQR

Dokumen 28 menjelaskan simbol rMQR dan bagaimana v0.17 men-decode grid rMQR yang bersih. Dokumen ini membahas cara menemukan grid itu di dalam gambar. rMQR berbagi masalah dengan Micro QR, yaitu hanya satu finder pattern di tempat QR punya tiga, dan menambah masalahnya sendiri: simbolnya bisa 20 kali lebih panjang daripada lebar finder-nya. Bagian-bagian berikut menunjukkan apa yang bisa dipastikan finder, apa yang tidak, bagaimana finder sub-pattern di sudut seberang menjadi jangkar untuk ujung yang lain, dan bagaimana pencarian ini bergabung ke pipeline setelah QR dan Micro QR.

> Versi bahasa Inggris: [30-rmqr-detection.md](30-rmqr-detection.md).

## 1. Apa yang belum ditentukan oleh satu finder

Detektor connected component (dokumen 24) menemukan finder rMQR seperti menemukan finder lainnya: cincin gelap yang mengelilingi cincin terang yang mengelilingi pusat gelap, dengan proporsi 7:5:3. Separator-nya memisahkan cincin itu dari bagian lain simbol, seperti pada Micro QR, sehingga empat sudut luar cincinnya bersih, dan koreksi dari dokumen 27 berlaku tanpa perubahan: setiap sudut digeser setengah piksel ke luar sampai batas cincin yang sebenarnya, dan sudut-sudutnya diurutkan dengan urutan tetap, sehingga mencocokkannya ke sudut modul (−0.5, −0.5), (6.5, −0.5), (6.5, 6.5), dan (−0.5, 6.5) hanya menyisakan empat pilihan, yaitu seperempat putaran.

Finder menetapkan sebuah homografi, tetapi hanya di atas 7 modul yang dicakupnya. Untuk Micro QR itu cukup untuk mencapai sudut jauh simbol berukuran 17 modul dengan bantuan refinement (dokumen 27, bagian 6). Simbol rMQR lebarnya sampai 139 modul, sehingga ujung jauhnya berada sampai 20 kali lebar finder, dan setiap galat di sudut finder membesar sebanding dengan jarak itu: sudut yang meleset sepersepuluh modul sudah cukup memiringkan prediksi tepi atas hingga meleset satu modul atau lebih di ujung jauh. Finder saja cukup untuk menentukan orientasi dan tempat membaca format information, tetapi tidak cukup untuk menentukan di mana simbolnya berakhir.

## 2. Orientasi dari timing pattern atas

Pada seperempat putaran yang benar, baris 0 di sebelah kanan finder terbaca dengan urutan tetap: kolom 7 adalah separator, terang, dan mulai kolom 8 timing pattern atas berjalan gelap di kolom genap dan terang di kolom ganjil. Pencarian membaca baris 0 pada kolom 7 sampai 11 lewat keempat kemungkinan penugasan dan menyimpan yang terbaca terang, gelap, terang, gelap, terang. Modul acak lolos tes ini satu dari 32 kali. Alignment pattern, yang menyela timing atas lebih jauh di kanan, berpusat di kolom 19 atau sesudahnya sehingga paling awal dimulai di kolom 18; ia tidak pernah menyentuh kelima modul ini.

zxing-cpp menguji kolom 8 sampai 11 dengan cara yang sama. Tes milik Micro QR (dokumen 27, bagian 2) membaca modul paling dalam dari kedua timing pattern, baris 0 dan kolom 0, tetapi simbol R7 tidak punya timing di kolom di bawah finder-nya, karena finder mengisi seluruh tingginya, jadi tes rMQR hanya memakai baris 0.

Simbol Micro QR juga lolos tes ini: baris 0-nya adalah timing pattern dengan paritas yang sama, dan pada simbol M1 kolom 11 adalah quiet zone, yang terang. Yang membedakan keduanya adalah format information dan sub-pattern, bukan tes orientasi.

## 3. Format information, sebelum ukurannya diketahui

Salinan format di samping finder berada di baris 1 sampai 5 pada kolom 8 sampai 10 dan baris 1 sampai 3 pada kolom 11 (dokumen 28, bagian 7), semuanya masih terjangkau homografi finder. Ke-18 bitnya, setelah mask 0x1FAB2 dilepas, dibandingkan dengan 64 word valid, satu untuk tiap versi dan level. BCH(18,6) punya jarak minimum 8, sehingga word yang berjarak paling banyak 3 bit dari word valid ter-decode secara unik, dan versinya menyebut kedua dimensi simbol.

Kecocokan itu sendiri adalah bukti yang lemah. Setiap word valid punya 988 word yang berjarak paling banyak 3 bit darinya, sehingga ke-64 word itu mencakup 24 persen dari seluruh 2¹⁸ word yang mungkin: satu dari empat word acak ter-decode menjadi suatu versi. Digabung dengan tes orientasi, kira-kira satu dari 130 penugasan finder acak sampai sejauh ini. Bukti yang berarti datang sesudahnya, dari ujung jauh simbol.

## 4. Finder sub-pattern, dan bagian mana yang dicari

Finder sub-pattern adalah pola 5 kali 5 di sudut kanan-bawah: cincin gelap, cincin terang, dan satu pusat gelap di baris h − 3, kolom w − 3. Bentuknya persis alignment pattern QR.

Cincin gelapnya adalah bagian yang salah untuk dicari. Tidak seperti finder, sub-pattern tidak punya separator: salinan format di sampingnya menempati kolom di sebelah kirinya dan baris di atasnya, dan pada simbol R7 corner pattern kanan-atas menyentuh tepi atasnya. Di mana pun salah satu modul itu gelap, cincin gelap menyatu dengannya menjadi satu komponen yang lebih besar dan bentuknya tidak tetap. Cincin terangnya berbeda. Cincin gelap yang utuh mengurungnya sepenuhnya, sehingga ia selalu menjadi komponen tersendiri, berupa persegi berongga selebar 3 modul, dengan pusat gelap selebar satu modul sebagai komponen di dalamnya. Susunan bersarang itu, komponen terang yang sisinya kira-kira tiga kali sisi komponen gelap yang konsentris di dalamnya, adalah ciri sub-pattern. `labelComponents` sudah melabeli komponen terang di seluruh gambar untuk pencarian finder, jadi kandidatnya hanya butuh satu kali lintasan atas keluarannya. Finder QR tidak memenuhi ciri ini, karena cincin terangnya selebar 5 modul di sekeliling pusat selebar 3 modul.

Ciri ini umum. Setiap alignment pattern QR memilikinya, begitu pula cincin terang yang kebetulan terbentuk oleh data di sekeliling satu modul gelap. Sebuah kandidat hanya diterima di tempat simbol membutuhkannya:

- pusatnya berada dekat titik yang diprediksi homografi finder untuk (w − 3, h − 3), dengan toleransi yang membesar sesuai jarak dari finder, karena begitulah galat prediksinya membesar;
- ukuran modulnya, sepertiga sisi cincin terangnya, dekat dengan ukuran modul finder.

Kandidat terdekat yang memenuhi syarat yang dipilih. zxing-cpp justru mencari di sekitar titik prediksi dengan locator alignment pattern miliknya; daftar kandidat yang dibangun sekali per gambar menjadikan toleransi sebagai parameter alih-alih radius pencarian, dan memakai ulang pelabelan yang sudah dibayar oleh pencarian finder.

Cincin terang juga memberi empat sudut lagi, di (w − 4.5, h − 4.5), (w − 1.5, h − 4.5), (w − 1.5, h − 1.5), dan (w − 4.5, h − 1.5) dalam koordinat modul. Sudut-sudut ini diekstrak seperti sudut finder, dikoreksi setengah piksel ke luar, dan dicocokkan ke sudut modulnya lewat prediksi homografi finder untuk masing-masing.

## 5. Geometri: delapan titik, lalu tepi-tepi panjang

Dengan empat sudut di tiap ujung simbol, homografi least-squares tidak lagi berekstrapolasi: ujung jauhnya diukur, bukan diprediksi. Namun pada simbol sepanjang 20 kali lebar finder, delapan titik yang berkumpul di dekat dua sudut tetap menyerahkan tepi-tepi panjangnya pada interpolasi, dan galat kecil di salah satu quad melengkungkan prediksi baris-baris di tengahnya.

Tepi-tepi panjang adalah pengukuran terbaik yang disediakan simbol ini. Baris 0 gelap di sepanjang finder, di setiap kolom genap timing pattern atas, di sepanjang baris atas setiap alignment pattern, dan di corner pattern kanan-atas; baris h − 1 gelap di corner pattern kiri-bawah, di setiap kolom genap timing pattern bawah, di sepanjang baris bawah setiap alignment pattern, dan di sepanjang bagian bawah sub-pattern. Pencarian berjalan dari setiap modul itu ke luar sampai tepi simbol, seperti edge fit di dokumen 27, lalu mem-fit satu garis untuk tiap tepi panjang lewat puluhan titik. Tepi-tepi pendeknya memakai kolom luar finder, modul-modul corner, dan kolom luar sub-pattern. Keempat garis bertemu di keempat sudut simbol, yang bersama delapan sudut dari kedua pola menghasilkan homografi akhir.

Seperti di v0.16, setiap refinement ditawarkan sebagai varian alih-alih langsung dipercaya: edge fit lebih dulu, lalu homografi delapan titik yang menjadi titik awalnya, dan Reed-Solomon yang memutuskan di antara keduanya. Finder yang tidak punya sub-pattern di tempat yang dibutuhkan versinya tidak di-decode sama sekali.

## 6. Salinan format kedua

Salinan format di samping sub-pattern, di baris h − 6 sampai h − 2 pada kolom w − 8 sampai w − 6 dan baris h − 6 pada kolom w − 5 sampai w − 3, dengan mask 0x20A7B, baru bisa dibaca setelah ujung jauhnya ditemukan. Salinan ini punya dua peran.

- **Konfirmasi.** Sebuah word acak berjarak paling banyak 3 bit dari dua word milik satu versi tertentu dengan peluang 2 × 988 / 2¹⁸, di bawah 1 persen. Ketika salinan di sisi finder sudah menyebut versinya, salinan di sisi sub-pattern yang cocok dengannya adalah bukti kuat bahwa simbolnya nyata, dan level-nya juga harus cocok.
- **Cadangan.** Ketika salinan di sisi finder tidak terbaca, pencarian mencoba setiap versi dari ke-32 versi yang posisi sub-pattern-nya berisi kandidat, lalu membaca salinan di sisi sub-pattern lewat homografi yang diberikan kandidat itu. Versi yang salinannya sendiri cocok diterima. Jadi satu salinan saja sudah cukup untuk menemukan simbolnya, sama seperti satu salinan sudah cukup bagi `decodeRMQRMatrix` (dokumen 28, bagian 7).

## 7. Bukti, stage, dan error

Setiap finder mencatat sejauh mana ia lolos, sehingga ketika tidak ada yang ter-decode, kegagalan yang paling meyakinkan yang dilaporkan:

- **none**: tidak ada seperempat putaran yang lolos tes orientasi;
- **oriented**: tes orientasi lolos, tetapi tidak ada salinan format yang menyebut versi;
- **format**: sebuah versi disebut, tetapi tidak ada sub-pattern di tempat yang ditentukannya;
- **located**: finder, sub-pattern di tempat yang dibutuhkan versinya, dan salinan format yang cocok; hasilnya diserahkan pada decoder matriks.

Hanya simbol yang sudah located yang meyakinkan. Finder QR atau Micro bisa sampai ke stage format secara kebetulan (bagian 3), tetapi sub-pattern di satu-satunya tempat yang ditentukan versi tertentu, dengan ukuran modul yang tepat, bukan kebetulan. Jadi, seperti untuk Micro QR (dokumen 27, bagian 8), `Decode` melaporkan error rMQR sebagai pengganti "finder patterns not found" hanya untuk simbol yang sudah located, dan simbol rMQR yang located mengalahkan simbol Micro yang sekadar lolos cek timing milik Micro, karena finder rMQR setinggi 9 bisa lolos cek itu.

## 8. Tempat pencarian ini bergabung ke pipeline

Pencarian rMQR datang paling akhir, pada finder quad yang sama. Pada setiap binarisasi, percobaan QR proyektif mengambil triple finder terbaik, pencarian Micro mencoba setiap finder sebagai finder Micro, dan baru setelah itu pencarian rMQR mencoba setiap finder sebagai finder rMQR. Di `DecodeAll`, triple QR mengklaim finder-nya lebih dulu, simbol Micro mengklaim miliknya berikutnya, dan finder yang belum diklaim ditawarkan ke pencarian rMQR. Simbol yang sudah ter-decode oleh pencarian sebelumnya tidak pernah dibaca ulang sebagai rMQR, sehingga hasil QR dan Micro QR tidak bisa berubah.

Urutan ini juga memisahkan simbologi-simbologinya. Finder Micro lolos tes orientasi rMQR dan bisa kebetulan menyebut suatu versi rMQR, tetapi tidak ada sub-pattern di tempat yang ditentukan versi itu, dan ia sudah ter-decode sebagai Micro QR. Finder rMQR R9 bisa lolos cek timing milik Micro, tetapi Reed-Solomon milik Micro menolak apa yang dibacanya, lalu pencarian rMQR membacanya.

Simbol rMQR yang ter-decode melaporkan outline-nya lewat homografi akhir: sudut modul (−0.5, −0.5), (w − 0.5, −0.5), (w − 0.5, h − 0.5), dan (−0.5, h − 0.5). Pusat finder-nya adalah satu-satunya finder yang dilaporkan `Result`.

## 9. zxing-cpp sebagai prior art

zxing-cpp 2.3.0 membaca simbol rMQR di dalam gambar dengan `SampleRMQR`, yang dicoba pada setiap finder yang tidak dipakai simbol QR. Ia menjalankan tes orientasi di bagian 2 pada kolom 8 sampai 11, membaca salinan format di sisi finder pada setiap seperempat putaran yang lolos, dan memilih putaran yang word-nya paling dekat dengan word valid. Ia mencari sub-pattern dengan locator alignment pattern miliknya di sekitar titik yang diprediksi homografi finder, mengambil sudut-sudut kedua pola, dan membangun transformasi empat titik, dari sudut luar kedua pola untuk simbol yang tingginya sampai 9 modul dan dari pusat keduanya ditambah dua sudut hasil perpotongan garis untuk simbol yang lebih tinggi. Ia mengambil sampel grid dari transformasi itu. Ia tidak membaca salinan format di sisi sub-pattern, tidak melakukan refinement di sepanjang simbol, dan menandai kodenya "TODO: this is a WIP". Ia adalah reader pembanding untuk implementasi ini.

## 10. Cara pengecekannya

Detektor ini diukur terhadap materi yang tidak berasal dari encoder kita sendiri: 70 simbol zint dari v0.17, dirender ke gambar di beberapa skala, di setiap seperempat putaran, dengan rotasi bidang dan kemiringan perspektif, serta dengan quiet zone dari nol sampai empat modul; reader zxing-cpp pada scene yang sama; komposit simbol QR, Micro QR, dan rMQR, yang masing-masing harus dilaporkan sekali sebagai jenisnya sendiri; dan setiap finder simbol QR dan Micro QR, yang tidak satu pun boleh ter-decode sebagai rMQR. Setiap langkah geometri hanya dipertahankan kalau ia membaca paling tidak sebanyak scene yang dibaca langkah sebelumnya. Rentang terukurnya dicatat di sini setelah diukur.

## 11. Penunjuk implementasi

Direncanakan di RD3 sampai RD5 (lihat `docs/plan-rmqr-detection.md`):

- `qrgen/decode_rmqr.go`: tes orientasi, kedua pembacaan format, kandidat dan pemasangan sub-pattern, homografi delapan titik, edge fit, stage-stage, dan decoder untuk satu finder.
- `qrgen/decode_finder_cc.go`: kandidat sub-pattern dari komponen terang hasil pelabelan.
- `qrgen/decode_image.go`: sampling persegi panjang, dan pass rMQR setelah pass Micro beserta error contract-nya.
- `qrgen/decode_multi.go`: pass rMQR di `DecodeAll`, dan `Result.RMQRVersion`.

## Referensi

- ISO/IEC 23941:2022 — tata letak rMQR, lewat source zint dan zxing-cpp (dokumen 28).
- zxing-cpp 2.3.0, core/src/qrcode/QRDetector.cpp (`SampleRMQR`) dan QRReader.cpp — prior art, dan reader independen yang dipakai untuk validasi.
- `docs/theory/24-arbitrary-angle-detection.md` — detektor connected component, ekstraksi sudut, dan homografi least-squares.
- `docs/theory/25-multi-symbol-decoding.md` — claim loop tempat pass rMQR bergabung.
- `docs/theory/27-micro-qr-detection.md` — pencarian satu finder, koreksi setengah piksel, dan edge fit yang diperluas oleh detektor ini.
- `docs/theory/28-rmqr.md` — simbol rMQR: tata letak, format information, dan decoder matriks yang menerima hasil detektor ini.
