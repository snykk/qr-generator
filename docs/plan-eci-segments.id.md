# QR Encoder — Rencana ECI Segments

Dokumen ini menjelaskan rencana implementasi **ECI (Extended Channel Interpretation) segments** untuk rilis minor `v0.9.0`. Ini menutup limitation pertama yang tercatat — payload byte-mode saat ini dikeluarkan sebagai UTF-8 mentah tanpa deklarasi charset, dan decoder memperlakukan byte segment sebagai UTF-8 secara implisit — dengan menambahkan mekanisme spec untuk mendeklarasikan character set sebuah byte segment di sisi encoder maupun decoder.

> Status: **draf / dokumen hidup.** Milestone ECI1..ECI5 mendarat bertahap di branch `eci-segments`; tiap milestone adalah commit fokus (atau deret commit kecil) berikut test, mengikuti irama milestone M, D, T, R, S, MM, P, dan TR.

> Versi Inggris: [docs/plan-eci-segments.md](plan-eci-segments.md).

---

## 1. Visi & Tujuan

- Mengimplementasikan **ECI segment** secara end-to-end: encoder bisa menambahkan ECI designator di depan yang mendeklarasikan character set dari byte data yang menyusul, dan decoder mem-parse ECI segment, melacak charset aktif, lalu men-decode byte segment berikutnya sesuai charset itu.
- Membuat perubahan ini **opt-in dan byte-identical secara default.** Tanpa ECI diminta, encoder mengeluarkan bit stream yang persis sama seperti sekarang, sehingga setiap golden fixture yang ada dan round-trip gozxing pihak ketiga tetap tak berubah. ECI adalah sesuatu yang diminta pemanggil, bukan perubahan perilaku otomatis.
- Membuat **encoding designator persis benar.** Nomor assignment ECI di-encode dalam 1, 2, atau 3 byte dengan prefix panjang yang self-describing; ini satu-satunya bagian yang benar-benar rewel dan justru pas untuk table-driven test.
- Menjaga **zero runtime dependency.** Hanya character set yang bisa di-transcode dengan standard library yang didukung: UTF-8 (string Go sudah UTF-8) dan ISO-8859-1 / Latin-1 (tiap rune paling besar `0xFF` memetakan ke satu byte). Code page sembarang seperti Shift-JIS akan butuh `golang.org/x/text` dan sengaja di luar scope.
- Filosofinya sama dengan tiap milestone sebelumnya: pure Go, reference-first dengan dokumen bilingual, table-driven test plus round-trip encode-decode dan cross-check gozxing supaya decoder independen setuju.

## 2. Prinsip Desain

1. **ECI opt-in lewat option bertipe.** Tipe `ECI` baru dengan konstanta untuk assignment yang didukung (`ECIUTF8` = 26, `ECILatin1` = 3) plus `WithECI(ECI) Option`. Zero value berarti "tidak ada ECI dideklarasikan", yang mempertahankan perilaku implicit-UTF-8 sekarang dan output bit-for-bit yang ada.
2. **Codec designator tinggal di satu tempat teruji.** File kecil `eci.go` memuat tipe `ECI`, encode dan decode designator 1/2/3-byte, serta helper transcoding stdlib. Encoder dan decoder sama-sama memanggilnya; layout byte di-unit-test terhadap ketiga kelas panjang spec di batas-batasnya.
3. **Mekanismenya general; charset yang didukung dibatasi.** Decoder mem-parse designator ECI *apa pun* (jadi tak pernah tersedak pada simbol valid), tetapi hanya ECI 3 dan 26 yang membawa transcoder. Payload byte itu sendiri selalu string Go yang di-round-trip lewat charset yang dideklarasikan; numeric dan alphanumeric segment charset-independent dan tak tersentuh.
4. **Version selection memperhitungkan header ECI.** ECI segment menambah `4 + 8/16/24` bit di depan data, jadi perhitungan needed-bits yang dipakai `selectVersion` dan cek kapasitas force-version menyertakan overhead ECI. Kalau tidak, payload yang pas-pasan bisa overflow begitu designator ditambahkan.
5. **Penempatan setia-spec.** ECI segment (mode indicator `0111` diikuti designator) dikeluarkan sekali di kepala data, sebelum deret mixed-mode segment, sesuai ISO/IEC 18004. Ia mengubah cara byte segment ditafsirkan; ia tidak mengubah decoding numeric/alphanumeric.
6. **Test lebih dulu**, termasuk round-trip: encode dengan ECI, decode, pastikan teks yang dipulihkan sama dengan input untuk UTF-8 dan Latin-1, plus cross-check gozxing yang memastikan decoder independen membaca simbol ber-ECI secara identik.

## 3. Cakupan

### Termasuk dalam v0.9.0

- Tipe `ECI` dengan `ECIUTF8` (26) dan `ECILatin1` (3); `WithECI(ECI) Option`.
- Encoder: mengeluarkan ECI segment (`0111` + designator 1/2/3-byte) saat ECI diminta, men-transcode payload byte ke charset yang dideklarasikan (UTF-8 passthrough, Latin-1 lewat penyempitan per-rune dengan error pada rune di atas `0xFF`), dan menyertakan overhead ECI dalam version selection.
- Decoder: mem-parse ECI segment di `decodeText` (`case 0b0111`), membaca designator, menetapkan charset aktif, lalu men-decode byte segment berikutnya lewatnya; ECI 3 → Latin-1, 26 → UTF-8.
- Encode/decode designator 1/2/3-byte dengan cakupan batas penuh (127/128, 16383/16384).
- Dokumen referensi `docs/theory/20-eci-segments.md` (EN + ID), contoh runnable, dan bagian penggunaan di README.

### Belum termasuk (masih)

- **Code page sembarang** (Shift-JIS, keluarga ISO-8859-* selain Latin-1, code page Windows). Ini butuh `golang.org/x/text`, yang akan melanggar aturan zero-runtime-dependency. Mekanismenya mendukung mereka secara struktural; hanya transcoder-nya yang dihilangkan.
- **Mode Kanji.** Item roadmap terpisah; ECI prasyaratnya tetapi tak digandeng dengannya di v0.9.
- **ECI otomatis.** Encoder tak pernah menyisipkan ECI sendiri; UTF-8 tanpa deklarasi tetap default sehingga output tak berubah bagi pemanggil yang tidak meminta.
- **Pergantian ECI per-segment di tengah payload.** v0.9 mengeluarkan satu ECI di kepala. Beberapa pergantian ECI dalam satu simbol valid secara spec tetapi tak dibutuhkan untuk kasus umum dan ditunda.

---

## 4. Milestone

Milestone mendarat berurutan. **Checkpoint A** (setelah ECI3) memberi encoder yang setia-spec dengan jalur default byte-identical. **Checkpoint B** (ECI5) adalah rilis `v0.9.0`.

### ECI1 — Plan Doc `(S)`

- [x] `docs/plan-eci-segments.md` dan padanan Indonesianya, mencakup visi, prinsip, cakupan, milestone ECI1..ECI5, delta tata letak file, risiko, referensi, dan pertanyaan terbuka.

### ECI2 — Dokumen Referensi ECI `(S)`

Tujuan: mendokumentasikan mekanisme ECI dan scope charset yang dibatasi sebelum kode apa pun mendarat.

- [ ] `docs/theory/20-eci-segments.md` — apa itu ECI dan masalah yang dipecahkannya (ketidaksesuaian implicit-UTF-8); mode indicator `0111`; encoding designator 1/2/3-byte berikut prefix panjang `0` / `10` / `110`-nya dan rentang nilainya; nomor assignment umum (3 = ISO-8859-1, 26 = UTF-8); di mana ECI segment duduk dalam data stream; batas transcoding zero-dependency (kenapa hanya UTF-8 dan Latin-1); serta contoh kerja. Ditutup dengan penunjuk implementasi.
- [ ] Padanan Indonesia `docs/theory/20-eci-segments.id.md`.
- [ ] Memperbarui `docs/theory/README.md` dan `.id.md`: entri 20 plus baris pemetaan kode yang menunjuk ke `qrgen/eci.go`.

### ECI3 — Encoder + Codec Designator `(M)`

Tujuan: sisi encoder, dengan jalur default terbukti tak berubah.

- [ ] `qrgen/eci.go` dengan tipe `ECI`, `ECIUTF8`/`ECILatin1`, `appendECIDesignator`/`readECIDesignator` (1/2/3-byte), dan helper `transcodeTo`/`transcodeFrom` (UTF-8 passthrough, penyempitan Latin-1). `ModeECI` (`0111`) ditambahkan ke `mode.go` demi simetri.
- [ ] `WithECI(ECI)` di `qrgen/options.go` (zero value = none); `encodeText` mengeluarkan ECI segment saat di-set dan men-transcode payload byte; `selectVersion`/`segmentsBitLength` menambah overhead ECI.
- [ ] Test: codec designator di batas 127/128 dan 16383/16384; input all-numeric dengan `WithECI` tetap memilih version yang benar; payload Latin-1 dengan rune di atas `0xFF` mengembalikan error yang jelas; dan guard test yang memastikan output tanpa-ECI byte-identical dengan encoder pra-perubahan untuk input representatif.

### Checkpoint A — encoder mengeluarkan ECI setia-spec; jalur default tanpa-ECI byte-identical.

### ECI4 — Decoder + Round-Trip `(M)`

Tujuan: mem-parse ECI saat decode dan membuktikan round-trip, termasuk pada decoder independen.

- [ ] `decodeText` mendapat `case 0b0111`: membaca designator, menetapkan charset aktif, lalu men-decode byte segment berikutnya lewatnya (3 → Latin-1, 26 → UTF-8). Menentukan dan mendokumentasikan perilaku ECI-tak-dikenal (lihat pertanyaan terbuka).
- [ ] `TestECIRoundTrip` meng-encode payload UTF-8 (ECI 26) dan Latin-1 (ECI 3), men-decode-nya, dan memastikan teks persisnya kembali; cakupan kelas designator lewat payload yang memaksa designator 1- dan 2-byte sejauh praktis.
- [ ] `TestRoundTripWithThirdPartyDecoder` mendapat satu kasus UTF-8 ber-ECI; gozxing membacanya secara identik, mengonfirmasi kesesuaian spec.

### ECI5 — Polish & Rilis `(S)`

Tujuan: memotong `v0.9.0`.

- [ ] README: catatan penggunaan ECI; baris `WithECI` plus konstanta `ECI` di ringkasan API; butir Limitations "No ECI segment" diperbarui mencerminkan dukungan opt-in baru dan scope charset-nya yang dibatasi; Scope dan Roadmap diperbarui.
- [ ] Contoh runnable `examples/encode/eci/main.go` (explicit-UTF-8 dan satu payload Latin-1).
- [ ] Entri `v0.9.0` di `CHANGELOG.md` plus anchor compare/tag ditulis; dibiarkan unstaged di working tree agar maintainer yang commit bersama rilis (meniru v0.6, v0.7, dan v0.8).
- [ ] `go test -race ./...` bersih, gofmt bersih.
- [ ] Tag `v0.9.0` (diserahkan ke maintainer sesuai alur git/rilis yang sudah mapan; anotasi direkomendasikan di percakapan rilis).

---

## 5. Usulan Delta Tata Letak File

```
qrgen/
├── eci.go                # baru — tipe ECI, codec designator, transcoder stdlib
├── eci_test.go           # baru — batas designator, round-trip encode/decode
├── mode.go               # +ModeECI (0111)
├── options.go            # +WithECI (+field)
├── encode.go             # emit ECI segment, akunting overhead di version selection
├── decode_matrix.go      # decodeText: case 0b0111, pelacakan charset aktif
├── roundtrip_test.go     # +kasus ECI gozxing
docs/
├── plan-eci-segments.md      # file ini
├── plan-eci-segments.id.md   # padanan Indonesia
└── theory/
    ├── 20-eci-segments.md     # baru
    └── 20-eci-segments.id.md  # baru
examples/encode/eci/
└── main.go               # baru — demo explicit-UTF-8 + Latin-1
```

## 6. Risiko & Catatan Teknis

- **Output default tak boleh bergeser.** Seluruh perubahan dipagari `WithECI`; jalur tanpa-ECI mengeluarkan bit stream yang identik. Guard test membandingkan output tanpa-ECI terhadap encoding yang sudah diketahui benar sehingga regresi langsung tertangkap, dan round-trip gozxing yang ada tetap hijau tanpa disentuh.
- **Encoding designator bagian yang rewel.** Bentuk 1/2/3-byte dengan prefix `0` / `10` / `110` harus persis, dan batas-batasnya (127 vs 128, 16383 vs 16384) tempat bug off-by-one bersembunyi. Kedua arah di-unit-test di batas-batas itu.
- **Akunting version selection.** Lupa menyertakan `4 + 8/16/24` bit ECI akan membiarkan payload yang pas-pasan overflow begitu designator ditambahkan. Jalur needed-bits menambah overhead sebelum memilih atau memvalidasi version.
- **Batas transcoding zero-dependency.** Hanya UTF-8 (native) dan Latin-1 (penyempitan per-rune) yang bisa di-transcode tanpa charset library. Permintaan Latin-1 dengan rune di atas `0xFF` adalah error pemanggil yang jelas, bukan mangle diam-diam. Dokumen menyatakan terang bahwa code page lain tak didukung by design.
- **ECI tak dikenal saat decode.** Decoder tetap harus mem-parse designator sebuah ECI yang tak punya transcoder, alih-alih salah membaca bit-nya sebagai data. Apa yang dilakukannya pada byte berikutnya (typed error vs best-effort UTF-8) diselesaikan di pertanyaan terbuka dan didokumentasikan.

## 7. Referensi

- ISO/IEC 18004:2015 — mode ECI (mode indicator `0111`), encoding ECI designator 1/2/3-byte, dan struktur segment.
- AIM ITS/04-001 — registry assignment *Extended Channel Interpretations*: 3 = ISO-8859-1, 26 = UTF-8, 20 = Shift-JIS, dst.
- `docs/theory/02-data-encoding.md` dan `docs/theory/09-data-tables.md` — catatan mode-indicator dan character-count yang ada, yang ECI perluas.
- Go standard library — `unicode/utf8` (validitas UTF-8) dan penyempitan Latin-1 yang trivial; sengaja bukan `golang.org/x/text`.

## 8. Pertanyaan Terbuka

Dijawab sebelum milestone terkait dimulai:

- **Bentuk option.** `WithECI(ECI)` dengan tipe `ECI` dan konstanta `ECIUTF8`/`ECILatin1` (diusulkan) versus option charset-string. Bentuk bertipe tak ambigu dan menjaga himpunan yang didukung tetap eksplisit.
- **Perilaku decode ECI-tak-dikenal.** `ErrUnsupportedECI` bertipe (jujur, gagal lantang) versus decoding best-effort UTF-8 atas byte berikutnya (longgar, bisa menghasilkan mojibake). Diusulkan: parse dan lewati designator, lalu decode sebagai UTF-8 best-effort dengan situasinya didokumentasikan, karena error keras akan menolak simbol yang sisanya terbaca. Dikonfirmasi di ECI4.
- **Emit ECI hanya saat ada byte segment.** Payload pure-numeric tak dapat apa-apa dari ECI. Diusulkan: emit saat diminta apa pun keadaannya (paling sederhana, valid-spec), dan catat optimasi tanpa-byte-segment sebagai tweak lanjutan.
- **Arah transcoding Latin-1.** Encode menyempitkan tiap rune ke satu byte (error di atas `0xFF`); decode melebarkan tiap byte ke satu rune. Konfirmasi bahwa ini batas penuh dukungan Latin-1 dan tak ada normalisasi yang dicoba.
