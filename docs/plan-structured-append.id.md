# QR Encoder — Rencana Structured Append

Dokumen ini menjelaskan rencana implementasi **structured append** untuk rilis minor `v0.10.0`. Ini menutup limitation bahwa sebuah payload harus muat dalam satu simbol (V40 mentok di ~2.953 byte) dengan membiarkan encoder memecah pesan panjang ke maksimal 16 simbol QR yang saling tertaut, dan decoder menyatukannya kembali menjadi teks asli.

> Status: **draf / dokumen hidup.** Milestone SA1..SA5 mendarat bertahap di branch `structured-append`; tiap milestone adalah commit fokus (atau deret commit kecil) berikut test, mengikuti irama milestone M, D, T, R, S, MM, P, TR, dan ECI.

> Versi Inggris: [docs/plan-structured-append.md](plan-structured-append.md).

---

## 1. Visi & Tujuan

- Mengimplementasikan **structured append** secara end-to-end: encoder membagi pesan menjadi set 2..16 simbol, masing-masing membawa structured-append header sehingga decoder tahu posisinya, total jumlahnya, dan sebuah parity byte; decoder menyatukan simbol secara berurutan dan memverifikasi mereka berasal dari set yang sama.
- Menjaga perubahan ini **murni aditif.** Structured append adalah fungsi publik baru (`EncodeStructuredAppend`, `DecodeStructuredAppend`); jalur single-symbol `Encode`/`Decode`/`Matrix` tak tersentuh dan output-nya byte-for-byte tak berubah.
- Membuat **header persis benar.** Tiap simbol dimulai dengan mode indicator `0011`, symbol sequence indicator 8-bit (posisi 4-bit + count-minus-one 4-bit), dan parity byte 8-bit yang sama untuk seluruh set. Ini satu-satunya bagian yang rewel dan justru pas untuk table-driven test.
- Menjaga **zero runtime dependency.** Structured append murni algoritmik — bit header, pembagian pesan, dan parity XOR — tanpa tabel karakter atau modul eksternal, tak seperti Kanji.
- Filosofinya sama dengan tiap milestone sebelumnya: pure Go, reference-first dengan dokumen bilingual, table-driven test plus round-trip split-lalu-reassemble dan cross-check gozxing per-simbol.

## 2. Prinsip Desain

1. **Fungsi baru, aditif.** `EncodeStructuredAppend(text string, parts int, opts ...Option) ([][]byte, error)` mengembalikan `parts` simbol PNG; `DecodeStructuredAppend(symbols [][]byte) (string, error)` membacanya kembali. Kasus tanpa structured-append cukup dengan tidak memanggil fungsi ini — `Encode` dan `DecodeBytes` mempertahankan perilaku single-symbol-nya persis.
2. **Header adalah hal pertama di tiap simbol.** Mode indicator `0011` (4 bit) + symbol sequence indicator (8 bit: nibble atas = posisi 0-based `m`, nibble bawah = total `n` dikurangi satu) + parity (8 bit). Ia mendahului ECI header apa pun dan data mode segment. Nibble count 4-bit membatasi satu set hingga 16 simbol.
3. **Parity adalah XOR atas seluruh pesan asli.** Parity byte adalah XOR setiap byte dari data UTF-8 asli (sebelum dibagi); tiap simbol dalam set membawa nilai yang sama. Saat reassemble, decoder menghitung ulang dari teks gabungan dan mengecek kecocokannya, menangkap simbol dari set yang salah atau salah-baca.
4. **Split di batas rune, semerata mungkin.** Pesan dibagi menjadi `parts` chunk kontigu pada batas rune (tak pernah memotong rune multi-byte), tiap chunk dijalankan independen lewat mixed-mode segmenter yang ada. Pembagiannya deterministik sehingga input sama selalu menghasilkan set yang sama.
5. **Satu version umum untuk set.** Semua simbol diukur ke satu version yang dipilih untuk memuat chunk terbesar (plus header-nya), sehingga set tampak seragam, sebagaimana lazimnya set structured-append. `WithVersion` yang dipaksa berlaku untuk tiap simbol.
6. **Test lebih dulu**, termasuk round-trip split-lalu-reassemble (`EncodeStructuredAppend` -> `DecodeStructuredAppend` -> asli persis) dan cross-check gozxing yang memastikan tiap simbol di-decode ke chunk-nya dengan metadata sequence yang benar.

## 3. Cakupan

### Termasuk dalam v0.10.0

- `EncodeStructuredAppend(text string, parts int, opts ...Option) ([][]byte, error)` — `parts` di 2..16, output PNG, satu version umum.
- `DecodeStructuredAppend(symbols [][]byte) (string, error)` — decode tiap simbol, urutkan berdasarkan posisi, gabungkan, verifikasi parity bersama dan total count yang konsisten; toleran terhadap simbol yang diberikan dalam urutan apa pun.
- Codec structured-append header: mode `0011`, sequence indicator posisi/count, dan parity XOR, dikeluarkan encoder dan di-parse decoder.
- Ketahanan single-symbol decode: `DecodeBytes`/`DecodeMatrix` atas satu simbol structured-append mem-parse dan melewati header lalu mengembalikan chunk simbol itu alih-alih error.
- Dokumen referensi `docs/theory/21-structured-append.md` (EN + ID), contoh runnable, dan bagian penggunaan di README.

### Belum termasuk (masih)

- **Penentuan otomatis jumlah simbol.** v0.10 menerima `parts` eksplisit; memecah ke jumlah minimum yang muat pada version target adalah kemungkinan kemudahan kemudian.
- **Structured append digabung dengan ECI atau Kanji.** Header mendukung mendahului ECI header secara struktural, tetapi v0.10 meng-encode tiap chunk sebagai mixed-mode UTF-8 default; SA + ECI ditunda.
- **Varian structured-append SVG dan terminal.** v0.10 mengembalikan PNG bytes; memaparkan matrix per-simbol untuk renderer lain adalah kemungkinan follow-up.
- **Structured append Micro QR.** Hanya keluarga standar 40-version yang didukung, seperti di tempat lain.
- **Lebih dari 16 simbol.** Nibble count 4-bit spec membatasi satu set ke 16; pesan lebih besar di luar jangkauan by design.

---

## 4. Milestone

Milestone mendarat berurutan. **Checkpoint A** (setelah SA3) memberi encoder yang menghasilkan set valid dan bisa dibaca independen. **Checkpoint B** (SA5) adalah rilis `v0.10.0`.

### SA1 — Plan Doc `(S)`

- [x] `docs/plan-structured-append.md` dan padanan Indonesianya, mencakup visi, prinsip, cakupan, milestone SA1..SA5, delta tata letak file, risiko, referensi, dan pertanyaan terbuka.

### SA2 — Dokumen Referensi Structured Append `(S)`

Tujuan: mendokumentasikan mekanisme sebelum kode apa pun mendarat.

- [x] `docs/theory/21-structured-append.md` — apa itu structured append dan limitation yang diangkatnya; layout header (mode `0011`, sequence indicator posisi/count, parity byte) dengan detail bit-level dan cap 16-simbol; bagaimana parity dihitung (XOR atas seluruh pesan asli) dan kenapa penting; di mana header duduk relatif terhadap ECI dan data segment; model pembagian pesan dan reassembly; serta contoh kerja. Ditutup dengan penunjuk implementasi.
- [x] Padanan Indonesia `docs/theory/21-structured-append.id.md`.
- [x] Memperbarui `docs/theory/README.md` dan `.id.md`: entri 21 plus baris pemetaan kode yang menunjuk ke `qrgen/structured_append.go`.

### SA3 — Encoder + Codec Header `(M)`

Tujuan: sisi encoder, menghasilkan set yang valid.

- [x] `qrgen/structured_append.go` dengan codec header (sequence indicator + parity), logika pembagian pesan, dan `ModeStructuredAppend` (`0011`) ditambahkan ke `mode.go`.
- [x] `EncodeStructuredAppend` di `qrgen/api.go`: validasi `parts` (2..16), bagi teks, pilih satu version umum, lalu keluarkan header + chunk tiap simbol lewat pipeline yang ada.
- [x] Test: encoding sequence-indicator untuk posisi/count representatif; parity sama dengan XOR byte asli; `parts` di luar rentang error; tiap simbol membawa header dan parity yang sama; chunk yang terlalu besar untuk V40 error.

### Checkpoint A — encoder menghasilkan set structured-append yang valid-spec.

### SA4 — Decoder + Round-Trip `(M)`

Tujuan: mem-parse header, reassemble, dan membuktikan round-trip termasuk pada decoder independen.

- [x] `decodeText` mendapat `case 0b0011`: baca sequence indicator dan parity, catat, lalu decode segment sisanya sebagai chunk simbol (single-symbol decode mengembalikan chunk).
- [x] `DecodeStructuredAppend` di `qrgen/decode.go`: decode tiap simbol, urutkan berdasarkan posisi, gabungkan, dan verifikasi parity bersama serta total count; kembalikan typed error pada set yang tak cocok atau tak lengkap.
- [x] `TestStructuredAppendRoundTrip` memecah payload panjang menjadi beberapa simbol dan memastikan `DecodeStructuredAppend` memulihkan asli persis, termasuk input urutan-acak; `TestRoundTripWithThirdPartyDecoder` memastikan gozxing membaca chunk tiap simbol.

### SA5 — Polish & Rilis `(S)`

Tujuan: memotong `v0.10.0`.

- [x] README: bagian penggunaan structured-append; baris `EncodeStructuredAppend`/`DecodeStructuredAppend` di ringkasan API; butir Limitations "No structured-append" diperbarui; Scope dan Roadmap diperbarui.
- [x] Contoh runnable `examples/encode/structured-append/main.go`.
- [x] Entri `v0.10.0` di `CHANGELOG.md` plus anchor compare/tag ditulis; dibiarkan unstaged di working tree agar maintainer yang commit bersama rilis (meniru v0.6 sampai v0.9).
- [x] `go test -race ./...` bersih, gofmt bersih.
- [ ] Tag `v0.10.0` (diserahkan ke maintainer sesuai alur git/rilis yang sudah mapan; anotasi direkomendasikan di percakapan rilis).

---

## 5. Usulan Delta Tata Letak File

```
qrgen/
├── structured_append.go       # baru — codec header, parity, pembagian pesan
├── structured_append_test.go  # baru — header + parity + round-trip
├── mode.go                     # +ModeStructuredAppend (0011)
├── api.go                      # +EncodeStructuredAppend
├── decode.go                   # +DecodeStructuredAppend
├── decode_matrix.go            # decodeText: case 0b0011 (parse header, return chunk)
├── encode.go                   # emisi SA header di depan data segment
├── roundtrip_test.go           # +kasus structured-append gozxing
docs/
├── plan-structured-append.md       # file ini
├── plan-structured-append.id.md    # padanan Indonesia
└── theory/
    ├── 21-structured-append.md      # baru
    └── 21-structured-append.id.md   # baru
examples/encode/structured-append/
└── main.go                     # baru — pecah payload panjang, decode kembali
```

## 6. Risiko & Catatan Teknis

- **Single-symbol decode tak boleh regresi.** Saat ini `decodeText` error pada mode indicator `0011`. Setelah SA4 ia mem-parse header dan mengembalikan chunk simbol, jadi perilaku single-symbol yang ada tak berubah untuk simbol non-SA dan simbol SA di-decode dengan anggun alih-alih gagal.
- **Konvensi parity.** Parity adalah XOR byte UTF-8 asli; encoder dan decoder harus sepakat, dan cross-check gozxing membaca chunk tiap simbol (gozxing memunculkan metadata sequence/parity tetapi menyerahkan reassembly ke pemanggil, seperti kita). Test mismatch menjaga terhadap simbol set-yang-salah.
- **Cap 16 simbol.** Nibble count 4-bit membatasi satu set ke 16 simbol; `parts` di luar 2..16 adalah error pemanggil yang jelas, dan pesan yang terlalu besar untuk 16 simbol V40 tak bisa di-encode.
- **Overhead header di version selection.** Data tiap simbol adalah structured-append header 20-bit-nya plus segment chunk-nya; perhitungan bit version-selection per-chunk menyertakan header sehingga chunk yang pas-pasan tak bisa overflow.
- **Pembagian batas-rune.** Split tak boleh memotong rune multi-byte, atau chunk akan memegang UTF-8 invalid dan teks gabungan akan berbeda. Pembagian pada batas rune dan round-trip test menguji konten multi-byte.

## 7. Referensi

- ISO/IEC 18004:2015 — mode indicator Structured Append `0011`, symbol sequence indicator (posisi + count), dan parity-data byte serta derivasi XOR-nya.
- `docs/theory/02-data-encoding.md` dan `docs/theory/20-eci-segments.md` — catatan mode-indicator dan header yang structured append perluas; SA header mendahului ECI header saat keduanya ada.
- ZXing / gozxing — penanganan structured-append di decoder independen, dipakai sebagai oracle cross-check.

## 8. Pertanyaan Terbuka

Dijawab sebelum milestone terkait dimulai:

- **`parts` eksplisit vs auto-count.** `EncodeStructuredAppend(text, parts, opts...)` diusulkan demi determinisme; varian auto-count yang memecah ke jumlah minimum yang muat pada version target bisa ditambah kemudian. Konfirmasi signature di SA3.
- **Version umum vs per-simbol.** Diusulkan: satu version umum diukur ke chunk terbesar, demi set seragam. Alternatifnya (tiap simbol di-auto-size independen) rata-rata lebih kecil tetapi tampak tak rata. Konfirmasi di SA3.
- **Format output.** v0.10 mengembalikan `[][]byte` (PNG). Memaparkan matrix per-simbol (untuk SVG/terminal) ditunda; konfirmasi PNG-saja diterima untuk cut pertama.
- **Kembalian single-symbol decode.** Diusulkan: `DecodeBytes` atas satu simbol SA mengembalikan chunk simbol itu (header di-parse dan diabaikan). Alternatifnya typed error "ini bagian dari set"; perilaku mengembalikan-chunk lebih ramah dan lossless. Konfirmasi di SA4.
- **Parity atas asli vs per-chunk.** Diusulkan: XOR atas seluruh pesan asli (setia-spec, dibagi ke semua simbol). Konfirmasi basis byte-nya (byte UTF-8 string asli) di SA2.
