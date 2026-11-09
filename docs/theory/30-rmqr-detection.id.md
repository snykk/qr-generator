# Deteksi rMQR

Dokumen 28 menjelaskan simbol rMQR dan bagaimana v0.17 men-decode grid rMQR yang bersih. Dokumen ini membahas cara menemukan grid itu di dalam gambar. rMQR berbagi masalah dengan Micro QR, yaitu hanya satu finder pattern di tempat QR punya tiga, dan menambah masalahnya sendiri: simbolnya bisa 20 kali lebih panjang daripada lebar finder-nya. Bagian-bagian berikut menunjukkan apa yang bisa dipastikan finder, apa yang tidak, bagaimana finder sub-pattern di sudut seberang menjadi jangkar untuk ujung yang lain, dan bagaimana pencarian ini bergabung ke pipeline setelah QR dan Micro QR.

> Versi bahasa Inggris: [30-rmqr-detection.md](30-rmqr-detection.md).

## 1. Apa yang belum ditentukan oleh satu finder

Detektor connected component (dokumen 24) menemukan finder rMQR seperti menemukan finder lainnya: cincin gelap yang mengelilingi cincin terang yang mengelilingi pusat gelap, dengan proporsi 7:5:3. Separator-nya memisahkan cincin itu dari bagian lain simbol, seperti pada Micro QR, sehingga empat sudut luar cincinnya bersih, dan koreksi dari dokumen 27 berlaku tanpa perubahan: setiap sudut digeser setengah piksel ke luar sampai batas cincin yang sebenarnya, dan sudut-sudutnya diurutkan dengan urutan tetap, sehingga mencocokkannya ke sudut modul (−0.5, −0.5), (6.5, −0.5), (6.5, 6.5), dan (−0.5, 6.5) hanya menyisakan empat pilihan, yaitu seperempat putaran.

Finder menetapkan sebuah homografi, tetapi hanya di atas 7 modul yang dicakupnya. Untuk Micro QR itu cukup untuk mencapai sudut jauh simbol berukuran 17 modul dengan bantuan refinement (dokumen 27, bagian 6). Simbol rMQR lebarnya sampai 139 modul, sehingga ujung jauhnya berada sampai 20 kali lebar finder, dan setiap galat di sudut finder membesar sebanding dengan jarak itu: sudut yang meleset sepersepuluh modul sudah cukup memiringkan prediksi tepi atas hingga meleset satu modul atau lebih di ujung jauh. Finder saja cukup untuk menentukan orientasi dan tempat membaca format information, tetapi tidak cukup untuk menentukan di mana simbolnya berakhir.

## 2. Orientasi dari timing pattern atas

Pada seperempat putaran yang benar, baris 0 di sebelah kanan finder terbaca dengan urutan tetap: kolom 7 adalah separator, terang, dan mulai kolom 8 timing pattern atas berjalan gelap di kolom genap dan terang di kolom ganjil. Pencarian membaca baris 0 pada kolom 7 sampai 11 lewat keempat kemungkinan penugasan dan menyimpan yang terbaca terang, gelap, terang, gelap, terang dengan paling banyak satu modul yang salah. Alignment pattern, yang menyela timing atas lebih jauh di kanan, berpusat di kolom 19 atau sesudahnya sehingga paling awal dimulai di kolom 18; ia tidak pernah menyentuh kelima modul ini.

Satu modul yang boleh salah itu hasil pengukuran, bukan asumsi. Baris 0 berbatasan dengan quiet zone, sehingga galat kecil di sudut finder mendorong sampelnya keluar dari simbol. Pada kemiringan 45 sampai 55 derajat di 4 dan 5 piksel per modul, seperempat putaran yang benar dari finder yang sudah ditemukan meleset di satu dari lima modul kira-kira dua kali lebih sering daripada di dua modul, dan menuntut kelimanya benar membuat simbol-simbol itu gugur di gate pertama ini. Dengan satu modul boleh salah, modul acak lolos enam dari 32 kali, bukan sekali, dan gate-gate sesudahnya lebih dari cukup untuk menampungnya.

Kelima modul itu berada sampai 11 modul dari finder, hampir dua kali lebarnya, dan ekstrapolasi sekecil itu pun sudah berpengaruh: lewat homografi yang ditetapkan oleh empat sudut finder, sampel dari finder yang kecil atau tampak menyusut karena perspektif sudah bisa bergeser dari modulnya. Karena itu tes ini dibaca lewat homografi itu dan lewat peta afine di bagian 4, dan cukup salah satunya yang lolos.

zxing-cpp menguji kolom 8 sampai 11 dengan cara yang sama, tetapi harus persis. Tes milik Micro QR (dokumen 27, bagian 2) membaca modul paling dalam dari kedua timing pattern, baris 0 dan kolom 0, tetapi simbol R7 tidak punya timing di kolom di bawah finder-nya, karena finder mengisi seluruh tingginya, jadi tes rMQR hanya memakai baris 0. Simbol Micro QR juga lolos tes ini: baris 0-nya adalah timing pattern dengan paritas yang sama, dan pada simbol M1 kolom 11 adalah quiet zone, yang terang. Yang membedakan keduanya adalah langkah-langkah sesudahnya.

## 3. Format information, sebelum ukurannya diketahui

Salinan format di samping finder berada di baris 1 sampai 5 pada kolom 8 sampai 10 dan baris 1 sampai 3 pada kolom 11 (dokumen 28, bagian 7), semuanya masih terjangkau dari finder. Ke-18 bitnya, setelah mask 0x1FAB2 dilepas, dibandingkan dengan 64 word valid, satu untuk tiap versi dan level, dan bacaan yang lebih dekat di antara kedua peta di bagian 2 yang dipakai. BCH(18,6) punya jarak minimum 8, sehingga word yang berjarak paling banyak 3 bit dari word valid ter-decode secara unik, dan versinya menyebut kedua dimensi simbol.

Kecocokan itu sendiri adalah bukti yang lemah. Setiap word valid punya 988 word yang berjarak paling banyak 3 bit darinya, sehingga ke-64 word itu mencakup 24 persen dari seluruh 2¹⁸ word yang mungkin: satu dari empat word acak ter-decode menjadi suatu versi. Digabung dengan tes orientasi, kira-kira satu dari 22 penugasan finder acak sampai sejauh ini. Bukti yang berarti datang sesudahnya, dari ujung jauh simbol.

## 4. Finder sub-pattern, dan di mana mencarinya

Finder sub-pattern adalah pola 5 kali 5 di sudut kanan-bawah: cincin gelap, cincin terang, dan satu pusat gelap di baris h − 3, kolom w − 3. Bentuknya persis alignment pattern QR.

**Bagian mana yang dicari.** Cincin gelapnya adalah bagian yang salah untuk dicari. Tidak seperti finder, sub-pattern tidak punya separator: salinan format di sampingnya menempati kolom di sebelah kirinya dan baris di atasnya, dan pada simbol R7 corner pattern kanan-atas menyentuh tepi atasnya. Di mana pun salah satu modul itu gelap, cincin gelap menyatu dengannya menjadi satu komponen yang lebih besar dan bentuknya tidak tetap. Cincin terangnya berbeda. Cincin gelap yang utuh mengurungnya sepenuhnya, sehingga ia selalu menjadi komponen tersendiri, berupa persegi berongga selebar 3 modul, dengan pusat gelap selebar satu modul sebagai komponen di dalamnya. Susunan bersarang itu, komponen terang yang sisinya kira-kira tiga kali sisi komponen gelap yang konsentris di dalamnya, adalah ciri sub-pattern. Finder QR tidak memenuhi ciri ini, karena cincin terangnya selebar 5 modul di sekeliling pusat selebar 3 modul.

`labelComponents` sudah melabeli komponen terang di seluruh gambar untuk pencarian finder, jadi kandidatnya hanya butuh satu kali lintasan atas keluarannya, dan lintasan yang linear: setiap komponen gelap kecil adalah calon pusat, dan komponen terang yang mengurungnya adalah komponen pemilik piksel tepat di atasnya, sehingga tidak ada pasangan komponen yang pernah dibandingkan. Komponen terang yang mencapai tepi gambar bukan cincin dan ditolak; kalau tidak, quiet zone di sekeliling sebuah pola yang berdiri sendiri bisa lolos sebagai cincin.

**Di mana mencarinya.** Ciri ini umum. Setiap alignment pattern QR memilikinya, begitu pula cincin terang yang kebetulan terbentuk oleh data di sekeliling satu modul gelap, sehingga sebuah kandidat hanya diterima di dekat tempat versinya meletakkan sub-pattern, dan memprediksi titik itu ternyata menjadi bagian yang sulit.

- **Bukan homografi finder.** Karena ditetapkan oleh empat sudut yang berjarak 7 modul, suku perspektifnya berayun liar oleh galat sudut sepecahan piksel saja, dan bila diekstrapolasi ke ujung jauh simbol yang panjang, prediksinya terukur meleset sampai 27 persen dari jarak hanya karena rotasi bidang. Fit afine atas keempat sudut yang sama, yang tidak punya suku perspektif, menjaga galat itu di bawah 7 persen.
- **Dikoreksi untuk perspektif.** Perspektif mengubah ukuran modul di sepanjang simbol, sedangkan peta afine menganggapnya tetap, sehingga di bawah kemiringan sub-pattern terletak sampai 79 persen dari jarak dari prediksi afine, selalu di sepanjang sumbu panjang simbol. Di sepanjang sebuah garis, pemetaan proyektif mengirim titik yang berjarak D modul ke D kali rata-rata geometris ukuran modul di kedua ujungnya; dengan r sebagai ukuran modul kandidat dibagi ukuran modul finder, sub-pattern berarti terletak √r kali lebih jauh daripada prediksi peta afine, dan setiap kandidat dinilai terhadap posisinya sendiri yang sudah dikoreksi. Koreksi itu membawa galat di sepanjang sumbu ke bawah 40 persen, tetapi tidak lebih rendah, karena binarisasi menebalkan modul gelap sehingga cincin terangnya terukur kira-kira seperlima lebih kecil.
- **Window yang lebih panjang daripada lebarnya.** Sumbunya sendiri tetap garis lurus di bawah perspektif, jadi galat yang melintanginya hanya berasal dari sudut finder: paling banyak 5 persen dari jarak di bawah rotasi dan 15 persen di bawah kemiringan. Window-nya 2 modul ditambah 60 persen dari jarak di sepanjang sumbu dan 18 persen melintanginya, dan ukuran modul kandidat boleh sampai 2,6 kali ukuran modul finder ke kedua arah, karena kemiringan membuat modul di ujung jauh sampai 2,8 kali lebih besar di sepanjang sumbu.

Di dalam window semacam itu beberapa kandidat bisa memenuhi syarat, jadi dua belas yang terdekat dicoba bergiliran, dan bukti di bagian 7 yang memutuskan di antara mereka.

Cincin terang juga memberi empat sudut lagi, di (w − 4.5, h − 4.5), (w − 1.5, h − 4.5), (w − 1.5, h − 1.5), dan (w − 4.5, h − 1.5) dalam koordinat modul. Sudut-sudut ini diekstrak seperti sudut finder, dikoreksi setengah piksel ke luar, dan dicocokkan ke sudut modulnya lewat pergeseran siklik yang paling dekat dengan prediksi homografi finder untuk masing-masing.

## 5. Geometri: delapan titik, lalu tepi-tepi panjang

Dengan empat sudut di tiap ujung simbol, homografi least-squares tidak lagi berekstrapolasi: ujung jauhnya diukur, bukan diprediksi. Namun pada simbol sepanjang 20 kali lebar finder, delapan titik yang berkumpul di dekat dua sudut tetap menyerahkan tepi-tepi panjangnya pada interpolasi, dan galat kecil di salah satu quad melengkungkan prediksi baris-baris di tengahnya.

Tepi-tepi panjang adalah pengukuran terbaik yang disediakan simbol ini. Setiap modul tepi yang ditetapkan gelap oleh tata letak, diambil dari tata letak itu sendiri sehingga modul finder, timing, alignment, corner, dan sub-pattern semuanya tercakup, menjadi titik awal: pencarian berjalan dari setiap modul itu ke luar sampai tepi simbol, seperti edge fit di dokumen 27, lalu mem-fit satu garis untuk tiap tepi, lewat puluhan titik pada tepi yang panjang. Keempat garis bertemu di keempat sudut simbol, yang bersama delapan sudut dari kedua pola menghasilkan homografi akhir; putaran kedua dimulai dari hasil putaran pertama.

Refinement ini sepadan dengan biayanya. Pada simbol terlebar dari tiap tinggi di 2 dan 3 piksel per modul, diputar per 10 derajat, homografi delapan titik saja membaca 31 dari 108 scene dan edge fit 73. Seperti di v0.16, setiap refinement ditawarkan sebagai varian alih-alih langsung dipercaya, edge fit lebih dulu dan homografi delapan titik paling akhir, dan Reed-Solomon yang memutuskan di antara mereka.

## 6. Salinan format kedua

Salinan format di samping sub-pattern, di baris h − 6 sampai h − 2 pada kolom w − 8 sampai w − 6 dan baris h − 6 pada kolom w − 5 sampai w − 3, dengan mask 0x20A7B, baru bisa dibaca setelah sebuah kandidat sub-pattern memberi ujung jauhnya. Ketika salinan di sisi finder tidak terbaca, pencarian mencoba setiap versi dari ke-32 versi, memasangkan finder dengan setiap kandidat di dekat tempat versi itu meletakkan sub-pattern, lalu membaca salinan di sisi sub-pattern lewat homografi hasilnya; pasangan yang salinannya menyebut versi itu lanjut ke cek di bagian 7. Jadi satu salinan saja sudah cukup untuk menemukan simbolnya, sama seperti satu salinan sudah cukup bagi `decodeRMQRMatrix` (dokumen 28, bagian 7).

Salinan ini sendiri tidak cukup untuk memastikan sebuah simbol. Sebuah word acak berjarak paling banyak 3 bit dari dua word milik satu versi tertentu dengan peluang 2 × 988 / 2¹⁸, di bawah 1 persen; tetapi jalur cadangan mencoba sampai dua belas kandidat untuk tiap versi dari 32 versi, ratusan bacaan, dan dalam pengujian salinan itu memang cocok secara kebetulan pada simbol yang kedua salinannya sudah dihancurkan. Konfirmasi butuh lebih banyak modul daripada 18.

## 7. Bukti, stage, dan error

Konfirmasinya adalah timing pattern di sepanjang kedua tepi panjang. Setiap modul fungsi di baris 0 dan baris h − 1, gelap atau terang, ditetapkan oleh tata letak, paling sedikit 54 modul pada versi tersempit. Simbol yang located adalah pasangan yang lewat salah satu homografinya paling banyak sepersepuluh dari modul-modul itu terbaca salah: simbol nyata yang dibaca lewat homografi yang baik membaca hampir semuanya dengan benar, sedangkan modul yang dibaca secara acak salah separuhnya, dan peluang lolos secara kebetulan bisa diabaikan.

Setiap finder mencatat sejauh mana ia lolos, sehingga ketika tidak ada yang ter-decode, kegagalan yang paling meyakinkan yang dilaporkan:

- **none**: tidak ada seperempat putaran yang lolos tes orientasi;
- **oriented**: tes orientasi lolos, tetapi tidak ada salinan format yang menyebut versi;
- **format**: sebuah salinan format menyebut versi, tetapi tidak ada pasangan di versi itu yang timing rows-nya terbaca;
- **located**: finder dan sub-pattern yang timing rows-nya terbaca benar; hasilnya diserahkan pada decoder matriks.

Finder QR atau Micro cukup sering sampai ke stage format secara kebetulan, tetapi tidak sampai located: dari setiap finder pada 224 simbol QR (versi 1 sampai 7, semua level dan mask) dan pada simbol Micro dari zint, tidak satu pun yang located. Jadi, seperti untuk Micro QR (dokumen 27, bagian 8), `Decode` melaporkan error rMQR sebagai pengganti "finder patterns not found" hanya untuk simbol yang sudah located, dan simbol rMQR yang located mengalahkan simbol Micro yang sekadar lolos cek timing milik Micro, karena finder rMQR setinggi 9 bisa lolos cek itu. Simbol yang kedua salinan format-nya tidak terbaca tidak menyebut versi apa pun dan dilaporkan tidak ditemukan.

## 8. Tempat pencarian ini bergabung ke pipeline

Pencarian rMQR datang paling akhir, pada finder quad yang sama. Pada setiap binarisasi, percobaan QR proyektif mengambil triple finder terbaik, pencarian Micro mencoba setiap finder sebagai finder Micro, dan baru setelah itu pencarian rMQR mencoba setiap finder sebagai finder rMQR. Di `DecodeAll`, triple QR mengklaim finder-nya lebih dulu, simbol Micro mengklaim miliknya berikutnya, dan finder yang belum diklaim ditawarkan ke pencarian rMQR. Simbol yang sudah ter-decode oleh pencarian sebelumnya tidak pernah dibaca ulang sebagai rMQR, sehingga hasil QR dan Micro QR tidak bisa berubah.

Urutan ini juga memisahkan simbologi-simbologinya. Finder Micro lolos tes orientasi rMQR dan bisa kebetulan menyebut suatu versi rMQR, tetapi tidak ada pasangan yang membaca timing rows rMQR di sekitarnya, dan ia sudah ter-decode sebagai Micro QR. Finder rMQR R9 bisa lolos cek timing milik Micro, tetapi Reed-Solomon milik Micro menolak apa yang dibacanya, dan tidak ada simbol rMQR dari zint, di seperempat putaran mana pun, yang terbaca sebagai Micro QR.

Simbol rMQR yang ter-decode melaporkan outline-nya lewat homografi akhirnya, yaitu sudut modul (−0.5, −0.5), (w − 0.5, −0.5), (w − 0.5, h − 0.5), dan (−0.5, h − 0.5), dan pusat finder-nya adalah satu-satunya finder yang dilaporkan `Result`.

Pencarian ini murah di tempat ia tidak menemukan apa-apa. Pada frame realistis tanpa simbol, ia tidak membuat perbedaan yang terukur pada `Decode`, dan pada kasus terburuk, frame berisi noise acak yang halus tempat setiap bintik menjadi kandidat, ia memakan kira-kira sepersepuluh waktunya, sisanya adalah pencarian finder itu sendiri (dokumen 24).

## 9. zxing-cpp sebagai prior art

zxing-cpp 2.3.0 membaca simbol rMQR di dalam gambar dengan `SampleRMQR`, yang dicoba pada setiap finder yang tidak dipakai simbol QR. Ia menjalankan tes orientasi di bagian 2 pada kolom 8 sampai 11, membaca salinan format di sisi finder pada setiap seperempat putaran yang lolos, dan memilih putaran yang word-nya paling dekat dengan word valid. Ia mencari sub-pattern dengan locator alignment pattern miliknya di sekitar titik yang diprediksi homografi finder, mengambil sudut-sudut kedua pola, dan membangun transformasi empat titik, dari sudut luar kedua pola untuk simbol yang tingginya sampai 9 modul dan dari pusat keduanya ditambah dua sudut hasil perpotongan garis untuk simbol yang lebih tinggi. Ia mengambil sampel grid dari transformasi itu. Ia tidak membaca salinan format di sisi sub-pattern, tidak melakukan refinement di sepanjang simbol, dan menandai kodenya "TODO: this is a WIP".

Pada 1680 scene yang sama, dibangun dari simbol-simbol zint (rotasi per 15 derajat di 3 dan 4 piksel per modul, serta kemiringan 30, 45, dan 55 derajat di 10 dan 5 piksel per modul dengan dua rotasi bidang), zxing-cpp membaca 409 dan detektor ini 1674. zxing-cpp membaca setiap scene yang tegak, tetapi hanya sedikit yang terputar atau miring, sesuai dengan yang diperkirakan dari transformasi empat titik tanpa refinement di sepanjang simbol yang panjang; ia tidak membaca satu pun scene yang terlewat oleh detektor ini, dan tidak pernah mengembalikan teks yang berbeda.

## 10. Cara pengecekannya

Detektor ini diukur terhadap materi yang tidak berasal dari encoder kita sendiri: 70 simbol zint dari v0.17 yang dirender ke gambar, reader zxing-cpp pada scene yang sama, komposit simbol QR, Micro QR, dan rMQR, serta finder dari simbol QR dan Micro QR, yang tidak boleh pernah terbaca sebagai rMQR.

- **Seperempat putaran.** Setiap simbol, dilukis di 3 dan 5 piksel per modul, terbaca di setiap seperempat putaran.
- **Rotasi.** Dengan langkah 5 derajat atas ke-70 simbol, 1255 dari 1260 scene terbaca di 4 piksel per modul, 1228 di 3, dan 984 di 2; yang terlewat di 4 adalah simbol-simbol terpanjang di dekat 45 derajat.
- **Kemiringan.** Di bawah empat rotasi bidang, setiap scene terbaca sampai kemiringan 55 derajat di 10 piksel per modul (278 dari 280 di 60), setiap scene di 30 derajat di 5, dan 262 dari 280 di 45.
- **Quiet zone.** Setiap scene terbaca dengan quiet zone dari nol sampai empat modul, karena setiap gate membaca modul milik simbol itu sendiri.
- **Kerusakan.** Dengan salinan format sisi finder dihancurkan, setiap simbol terbaca lewat salinan yang lain; dengan datanya dihancurkan, simbolnya tetap located dan melaporkan error Reed-Solomon-nya.
- **Kekeliruan.** Tidak ada finder QR atau Micro yang located sebagai rMQR, tidak ada simbol rMQR yang terbaca sebagai Micro QR, dan `DecodeAll` melaporkan scene yang mencampur ketiga jenis itu masing-masing sekali, sebagai jenisnya sendiri.

## 11. Penunjuk implementasi

Dibangun di RD3 sampai RD6 (lihat `docs/plan-rmqr-detection.md`):

- `qrgen/decode_rmqr.go`: tes orientasi (`rmqrOriented`), kedua pembacaan format (`nearestRMQRFormat`, `rmqrSubFormat`), prediksi afine dan window-nya (`finderAffine`, `subPatternsNear`), homografi delapan titik (`rmqrHomography`), edge fit (`edgeFitRMQRHomography`), cek timing (`rmqrTimingAgrees`), stage-stage, dan decoder untuk satu finder (`locateRMQR`, `decodeRMQRQuad`).
- `qrgen/decode_finder_cc.go`: satu pelabelan yang dipakai bersama oleh setiap pencarian (`scanComponents`), serta kandidat sub-pattern dan sudut cincinnya (`subPatterns`, `ringCorners`).
- `qrgen/decode_image.go`: sampling persegi panjang (`sampleGrid`), dan pass rMQR setelah pass Micro beserta error contract-nya.
- `qrgen/decode_multi.go`: pass rMQR di `DecodeAll`, dan `Result.RMQRVersion`.

## Referensi

- ISO/IEC 23941:2022 — tata letak rMQR, lewat source zint dan zxing-cpp (dokumen 28).
- zxing-cpp 2.3.0, core/src/qrcode/QRDetector.cpp (`SampleRMQR`) dan QRReader.cpp — prior art, dan reader independen yang dipakai untuk validasi.
- `docs/theory/24-arbitrary-angle-detection.md` — detektor connected component, ekstraksi sudut, dan homografi least-squares.
- `docs/theory/25-multi-symbol-decoding.md` — claim loop tempat pass rMQR bergabung.
- `docs/theory/27-micro-qr-detection.md` — pencarian satu finder, koreksi setengah piksel, dan edge fit yang diperluas oleh detektor ini.
- `docs/theory/28-rmqr.md` — simbol rMQR: tata letak, format information, dan decoder matriks yang menerima hasil detektor ini.
