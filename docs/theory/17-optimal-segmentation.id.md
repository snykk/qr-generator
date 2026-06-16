# Optimal Mode Segmentation

Encoder hingga v0.5 memilih **satu** mode untuk seluruh payload: mode paling restriktif yang menutupi setiap karakter (numeric kalau semua digit, selain itu alphanumeric kalau semua di set alphanumeric, selain itu byte). Greedy single-mode itu sederhana dan sering memadai, tapi dapat meninggalkan bit yang terbuang ketika payload mencampur kelas karakter — misalnya digit run panjang yang terkubur di dalam teks yang sebagian besar byte. v0.6 mengganti pilihan greedy dengan **dynamic-programming optimal segmentation** yang memecah input menjadi urutan mode segment yang meminimalkan total panjang bit ter-encode. Dokumen ini membahas cost model, DP-nya, interplay version-group, aturan batas UTF-8, dan invariant yang menjaga input homogen tetap byte-for-byte tak berubah.

> Versi Inggris: [17-optimal-segmentation.md](17-optimal-segmentation.md).

## 1. Kenapa satu mode meninggalkan bit terbuang

Tiap mode mengemas karakter pada densitas berbeda:

| Mode | Bit per unit | Bit/char efektif |
| --- | --- | --- |
| Numeric | 10 bit per 3 digit | ~3.33 / digit |
| Alphanumeric | 11 bit per 2 char | 5.5 / char |
| Byte | 8 bit per byte | 8 / byte |

Digit jauh lebih murah di numeric mode (3.33 bit) dibanding alphanumeric (5.5) atau byte (8). Jadi ketika payload mengandung run digit yang dikelilingi karakter yang memaksa mode lebih longgar, meng-encode keseluruhannya di mode longgar itu memboroskan bit pada digit-nya. Memecah run digit ke numeric segment-nya sendiri memulihkan bit itu — tapi tiap segment baru membayar header segar (mode indicator 4-bit plus character-count indicator), sehingga sebuah split baru menguntungkan ketika run-nya cukup panjang untuk memulihkan lebih dari biaya header-nya.

Spec QR secara eksplisit mengizinkan satu simbol memuat beberapa mode segment berurutan (ISO/IEC 18004 klausa 7.4.1); decoder membaca header segment sampai bertemu terminator. Jadi output mixed-mode sepenuhnya spec-conformant dan tidak butuh kerja sama decoder di luar yang sudah ada.

## 2. Cost model per-segment

Sebuah segment yang meng-encode substring `s` di mode `m` pada versi `v` berbiaya:

```text
cost(m, s, v) = 4                       (mode indicator)
              + CharCountBits(m, v)      (character-count indicator, bergantung version-group)
              + payloadBits(m, s)        (data-nya sendiri)
```

di mana `payloadBits` adalah packing per-mode yang familiar:

```text
numeric(n digit):       floor(n/3)*10 + {0:0, 1:4, 2:7}[n mod 3]
alphanumeric(n char):   floor(n/2)*11 + {0:0, 1:6}[n mod 2]
byte(n byte):           n*8
```

Total biaya sebuah segmentation adalah jumlah atas segment-segmentnya. Terminator, bit padding, dan pad bytes ditambahkan sekali di akhir (mereka tidak bervariasi dengan segmentation), jadi DP meminimalkan hanya jumlah biaya segment.

## 3. Dynamic program-nya

Berjalan dari kiri ke kanan atas input. Misalkan `dp[i][m]` adalah biaya total minimum untuk meng-encode `i` karakter pertama sedemikian sehingga segment yang menutupi karakter `i-1` berada di mode `m`. Karakter di posisi `i` dapat **memperpanjang** segment saat ini (tetap di mode `m`, hanya membayar payload bit inkremental karakter itu) atau **memulai segment baru** di mode eligible berbeda (membayar header segar `4 + CharCountBits(m, v)`). Base case menyemai tiap mode di posisi 0 dengan biaya header-nya; jawabannya adalah `min atas m dari dp[n][m]`, dan back-pointer per cell merekonstruksi segment terpilih.

Dua kemudahan implementasi membuat ini bersih:

- Karena `payloadBits` tidak linear per karakter (numeric mengemas dalam group 3, alphanumeric dalam pasangan), paling mudah menghitung biaya tiap kandidat segment dengan closed-form `payloadBits` di atas alih-alih delta per-karakter inkremental. Formulasi benar yang sederhana menghitung, untuk tiap cut, biaya segment yang berakhir di cut itu di tiap mode — `O(n²)` di kasus terburuk tapi sepele cepatnya untuk payload seukuran QR. Formulasi Nayuki mencapai `O(n)` dengan melacak running cost per-mode; keduanya oke di sini, dan catatan implementasi merekam mana yang kita pilih.
- Sebuah karakter *eligible* untuk sebuah mode hanya kalau mode tersebut dapat merepresentasikannya: digit eligible untuk ketiga mode, karakter alphanumeric untuk alphanumeric dan byte, selainnya untuk byte saja.

## 4. Contoh terkerjakan: `"Order #1234567890"`

Ambil `"Order #1234567890"` pada V1 (lebar character-count: numeric 10, alphanumeric 9, byte 8). Huruf kecil memaksa byte mode di bawah greedy analyzer, jadi seluruh string 17-byte ter-encode sebagai satu byte segment:

```text
greedy byte:  header 4 + 8 = 12,  payload 17*8 = 136,  total 148 bit
```

Segmentation optimal memecah di run digit — `"Order #"` tetap byte (ia mengandung huruf kecil dan `#`, keduanya tak terrepresentasi di mode compact), dan `"1234567890"` menjadi numeric:

```text
byte "Order #":     header 12,  payload 7*8 = 56,   subtotal 68
numeric "1234567890": header 4 + 10 = 14,  payload floor(10/3)*10 + 4 = 34,  subtotal 48
total optimal: 68 + 48 = 116 bit
```

Itu penghematan 32-bit (148 → 116), yang di batas V1-M dapat menjadi pembeda antara muat di simbol lebih kecil dan naik satu versi.

**Sebuah counter-example yang layak diresapi:** `"PHONE: 12345"` *sepenuhnya alphanumeric* (`:` dan spasi ada di set alphanumeric), jadi greedy analyzer sudah meng-encode-nya sebagai satu alphanumeric segment — 13 + 66 = 79 bit pada V1. Memecah `"12345"` ke numeric akan berbiaya 13 + 39 (alpha `"PHONE: "`) + 14 + 17 (numeric) = 83 bit, yang *lebih buruk*. Run 5-digit terlalu pendek untuk memulihkan ~14-bit header ekstra. Break-even-nya kira-kira **7 digit** ketika menarik run keluar dari alphanumeric dan kira-kira **4 digit** keluar dari byte. DP menemukan ini secara otomatis dan cukup mempertahankan single alphanumeric segment — optimal segmentation tidak pernah menghasilkan hasil lebih buruk dari greedy, by construction.

## 5. Interplay version-group

`CharCountBits(m, v)` bergantung pada version group:

| Versi | Numeric | Alphanumeric | Byte |
| --- | --- | --- | --- |
| 1–9 | 10 | 9 | 8 |
| 10–26 | 12 | 11 | 16 |
| 27–40 | 14 | 13 | 16 |

Karena biaya header tiap segment berubah lintas group, segmentation *optimal* dapat berbeda per group: character-count indicator yang lebih lebar membuat segment ekstra lebih mahal, kadang membalik split marginal kembali ke single segment. Ini menciptakan sirkularitas — segmentation optimal bergantung pada versi, tapi versi yang kita butuhkan bergantung pada panjang ter-encode, yang bergantung pada segmentation.

Kita menyelesaikannya dengan cara yang sama seperti encoder single-mode sudah menyelesaikan dependensi analog untuk lebar character-count: iterasi. Untuk tiap versi kandidat dari 1 ke atas, hitung segmentation optimal *untuk versi itu* dan total panjangnya, lalu pilih versi terkecil yang panjangnya muat di kapasitas data. Menjalankan DP hingga 40 kali murah untuk input seukuran QR; karena biayanya hanya benar-benar berubah di dua batas group (9→10 dan 26→27), hasilnya dapat di-cache menjadi tiga komputasi kalau benchmark menunjukkan itu penting.

## 6. UTF-8 dan aturan batas rune

Karakter numeric dan alphanumeric adalah ASCII single-byte. Karakter lain mana pun — huruf beraksen, ideograf CJK, emoji — adalah UTF-8 multi-byte dan hanya dapat tinggal di byte segment, di mana ia menyumbang panjang byte UTF-8 penuhnya ke character count byte-mode (byte mode menghitung byte, bukan rune). Dua aturan menjaga ini benar:

- DP iterasi atas **rune**, tidak pernah memecah rune multi-byte lintas batas segment. Rune multi-byte eligible untuk byte mode saja.
- `payloadBits` dan character count sebuah byte segment dihitung dari panjang **byte** substring-nya (`len(string(runes))`), bukan jumlah rune.

Ini cocok dengan perilaku byte-mode encoder yang ada (UTF-8 passthrough) dan menjaga round trip tetap eksak untuk payload Unicode sembarang.

## 7. Jaminan identitas input-homogen

Properti kebenaran terpenting: untuk input yang seluruhnya satu mode — semua digit, semua alphanumeric, atau string apa pun yang greedy analyzer akan encode sebagai satu byte segment — DP harus mengembalikan persis **satu** segment, byte-for-byte identik dengan output pra-segmentation. Ini benar karena single segment menanggung persis satu header, dan menambah split internal apa pun menambah setidaknya satu header lagi tanpa penghematan payload yang mengimbangi (karakter-karakternya sudah di mode penutup termurahnya). DP, yang meminimalkan total biaya, karena itu mempertahankan single segment. Jaminan inilah yang membuat golden fixture v0.1 dan round-trip gozxing tetap hijau tanpa perubahan — segmentation hanya pernah mengubah output payload yang genuinely campuran.

## 8. Pointer Implementasi

- `qrgen/segment.go` (baru di v0.6) menampung type `segment`, `segmentText(text string, v Version) []segment`, dan `segmentsBitLength`.
- `qrgen/encode.go` `selectVersion` mensize segmentation optimal per versi kandidat; `encodeText` menulis tiap `[mode indicator][char count][payload]` segment via `writeNumeric/Alphanumeric/Byte` yang ada, lalu terminator + padding tunggal yang dibagi.
- `analyzeMode` tetap sebagai helper internal dan special case terdokumentasi (ia sama dengan output DP untuk input homogen), berguna untuk tes dan keterbacaan.
- Decoder tidak tersentuh: bit-stream parser-nya sudah loop atas header segment, jadi output ter-segmentasi decode tanpa perubahan.

## Referensi

- ISO/IEC 18004:2015 — klausa 7.4 (data encoding), klausa 7.4.1 (beberapa mode segment dalam satu simbol).
- Project Nayuki — *Optimal text segmentation for QR Codes*: <https://www.nayuki.io/page/optimal-text-segmentation-for-qr-codes>. Formulasi DP yang diadopsi di sini, termasuk varian running-cost `O(n)`.
- `docs/theory/02-data-encoding.md` — packing per-mode dan lebar character-count yang menjadi dasar ini.
- `docs/theory/09-data-tables.md` — tabel `CharCountBits` lengkap dan map nilai alphanumeric.
