# QR Encoder — Rencana Kanji Mode

Dokumen ini menjelaskan rencana implementasi **Kanji mode** untuk rilis minor `v0.11.0`. Ini menutup limitation "No Kanji mode" dan melengkapi himpunan mode standar encoder: setelah numeric, alphanumeric, byte, mixed-mode segmentation, dan ECI, Kanji adalah mode data terakhir yang didefinisikan ISO/IEC 18004. Teks Jepang yang saat ini jatuh ke byte mode (tiga byte UTF-8, 24 bit per karakter BMP) bisa di-encode pada 13 bit per karakter native menurut spec.

> Status: **draf / dokumen hidup.** Milestone K1..K6 mendarat bertahap di branch `kanji-mode`; tiap milestone adalah commit fokus (atau deret commit kecil) berikut test, mengikuti irama milestone M, D, T, R, S, MM, P, TR, ECI, dan SA.

> Versi Inggris: [docs/plan-kanji-mode.md](plan-kanji-mode.md).

---

## 1. Visi & Tujuan

- Mengimplementasikan **Kanji mode** secara end-to-end: encoder mengenali karakter Shift-JIS (JIS X 0208) dan meng-encode tiap satu sebagai nilai 13-bit di bawah mode indicator `1000`; decoder membalikkannya kembali ke rune asli.
- Menjadikannya **bagian dari optimal segmentation.** Kanji bergabung dengan numeric, alphanumeric, dan byte sebagai mode keempat di DP segmenter, sehingga payload campuran Jepang/ASCII terpecah antar-mode untuk meminimalkan ukuran simbol alih-alih jatuh sepenuhnya ke byte mode.
- Menjaga **zero runtime dependency.** Pemetaan Unicode-ke-Shift-JIS di-generate saat development ke tabel Go murni yang di-commit (`kanji_table.go`); generator-nya program `//go:build ignore` yang dijalankan via `go generate` dan satu-satunya yang menyentuh `golang.org/x/text` — yang sudah menjadi modul indirect (test-only) dan tak pernah masuk ke runtime atau build test normal library.
- **Jujur soal densitas.** Kanji mode adalah 13 bit per karakter versus 24 bit untuk kanji BMP di byte mode UTF-8 — kira-kira 1,8x lebih padat, bukan "4x" seperti klaim limitation README sekarang. Angka itu dikoreksi sebagai bagian milestone ini.
- Filosofinya sama dengan tiap milestone sebelumnya: pure Go, reference-first dengan dokumen bilingual, table-driven test plus round-trip encode-decode dan cross-check gozxing.

## 2. Prinsip Desain

1. **Tabel pemetaan yang di-generate dan di-commit.** `kanji_table.go` memuat peta Unicode-ke-Shift-JIS dan sebaliknya untuk JIS X 0208. Ia diproduksi oleh `gen_kanji_table.go` (`//go:build ignore`, memakai `x/text/encoding/japanese.ShiftJIS`) dan di-check in, sehingga membangun atau meng-test library tak butuh modul eksternal. Regenerasi adalah langkah `go generate` yang terdokumentasi, bukan bagian CI.
2. **Codec Kanji tinggal di satu tempat teruji.** `qrgen/kanji.go` memuat encode rune-ke-13-bit dan decode 13-bit-ke-rune (aritmetika offset Shift-JIS), plus eligibility (apakah sebuah rune ada di tabel). Kedua arah di-unit-test di batas-batas range.
3. **Kanji adalah mode segmentation kelas satu.** DP segmenter mendapat `ModeKanji` dengan cost per-karakter 13 bit dan prefix Kanji-eligibility (sebuah rune dihitung eligible hanya jika tabel memetakannya). Input non-Kanji homogen tak terpengaruh, jadi golden fixture yang ada tetap byte-for-byte identik.
4. **Layout bit setia-spec.** Mode indicator `1000`; character-count indicator 8 / 10 / 12 bit untuk version group 1-9 / 10-26 / 27-40 (lebar Kanji, tersempit di antara semua mode); tiap karakter 13 bit via derivasi Shift-JIS standar (bagian di bawah).
5. **Hanya JIS X 0208.** Satu-satunya set double-byte yang disasar QR Kanji mode. Karakter di luarnya (JIS X 0212, rune tak terpetakan) sekadar tidak Kanji-eligible dan jatuh ke byte mode seperti sekarang; bukan error, sekadar pilihan mode.
6. **Test lebih dulu**, termasuk round-trip: encode teks Jepang, decode, pastikan rune persisnya kembali, plus cross-check gozxing supaya decoder independen setuju, dan round-trip tabel atas tiap code point yang terpetakan.

## 3. Encoding Kanji (referensi)

Untuk kode dua-byte Shift-JIS `C`:

- Jika `0x8140 <= C <= 0x9FFC`: `D = C - 0x8140`.
- Jika `0xE040 <= C <= 0xEBBF`: `D = C - 0xC140`.
- Lalu nilai 13-bit adalah `(D >> 8) * 0xC0 + (D & 0xFF)`.

Decoding membalikkannya: dari nilai 13-bit `V`, `D = ((V / 0xC0) << 8) | (V % 0xC0)`, lalu tambahkan kembali `0x8140` atau `0xC140` tergantung range mana `D` jatuh. Library bekerja dalam rune Unicode, jadi encode adalah rune -> Shift-JIS (tabel) -> 13-bit, dan decode adalah 13-bit -> Shift-JIS -> rune (tabel).

## 4. Cakupan

### Termasuk dalam v0.11.0

- `qrgen/kanji.go`: codec rune/Shift-JIS/13-bit dan Kanji eligibility, plus `ModeKanji` (`1000`) di `mode.go` dan lebar character-count 8/10/12-nya.
- `kanji_table.go` (di-generate, di-commit) dan `gen_kanji_table.go` (generator `//go:build ignore`) dengan direktif `go generate`.
- Encoder: `ModeKanji` di DP segmenter, `writeKanji`, dan kontribusi Kanji ke perhitungan bit version-selection.
- Decoder: `decodeText` mendapat `case 0b1000`, men-decode Kanji segment kembali ke rune.
- Dokumen referensi `docs/theory/22-kanji-mode.md` (EN + ID), contoh runnable, dan bagian penggunaan README plus koreksi densitas.

### Belum termasuk (masih)

- **Character set selain JIS X 0208** (JIS X 0212, ekstensi Shift-JIS, set nasional lain). Rune di luar JIS X 0208 tetap di byte mode.
- **Kanji digabung dengan ECI non-default.** v0.11 memakai Kanji di bawah interpretasi default; memasangkan Kanji segment dengan ECI eksplisit ditunda.
- **Kanji Micro QR.** Hanya keluarga standar 40-version, seperti di tempat lain.
- **Mengirim x/text sebagai runtime dependency.** Ia hanya dipakai generator yang diabaikan; library dan test normalnya tak pernah meng-import-nya.

---

## 5. Milestone

Milestone mendarat berurutan. **Checkpoint A** (setelah K4) memberi encoder yang mengeluarkan Kanji segment dengan jalur default tetap byte-identical. **Checkpoint B** (K6) adalah rilis `v0.11.0`.

### K1 — Plan Doc `(S)`

- [ ] `docs/plan-kanji-mode.md` dan padanan Indonesianya, mencakup visi, prinsip, referensi encoding, cakupan, milestone K1..K6, delta tata letak file, risiko, referensi, dan pertanyaan terbuka.

### K2 — Dokumen Referensi Kanji `(S)`

Tujuan: mendokumentasikan mode dan strategi tabel sebelum kode apa pun mendarat.

- [ ] `docs/theory/22-kanji-mode.md` — mode indicator `1000` dan lebar count 8/10/12; derivasi dua-byte-Shift-JIS-ke-13-bit dengan contoh kerja; cakupan JIS X 0208; angka densitas akurat versus byte mode; dan strategi tabel-di-generate (kenapa tabel yang di-commit menjaga runtime zero-dependency). Ditutup dengan penunjuk implementasi.
- [ ] Padanan Indonesia `docs/theory/22-kanji-mode.id.md`.
- [ ] Memperbarui `docs/theory/README.md` dan `.id.md`: entri 22 plus baris pemetaan kode yang menunjuk ke `qrgen/kanji.go`.

### K3 — Tabel Shift-JIS + Codec `(M)`

Tujuan: tabel pemetaan dan codec standalone, tercakup penuh.

- [ ] `gen_kanji_table.go` (`//go:build ignore`) yang men-generate `kanji_table.go` dari `x/text/encoding/japanese.ShiftJIS`, dengan direktif `//go:generate`; tabel yang di-commit adalah Go murni tanpa import.
- [ ] `qrgen/kanji.go`: `kanjiEligible(rune) bool`, `kanjiValue(rune) (uint16, bool)` (rune -> 13-bit), dan `kanjiRune(uint16) (rune, bool)` (13-bit -> rune).
- [ ] Test: aritmetika offset di batas kedua range Shift-JIS; round-trip tabel penuh (tiap rune terpetakan encode dan decode kembali); sebuah rune non-Kanji melaporkan tak-eligible.

### K4 — Integrasi Encoder `(M)`

Tujuan: Kanji di segmenter dan writer, jalur default tak berubah.

- [ ] `ModeKanji` ditambahkan ke himpunan mode DP segmenter dengan cost per-karakter 13-bit dan prefix Kanji-eligibility; `writeKanji` mengeluarkan nilai 13-bit; penanganan character-count diperluas untuk lebar 8/10/12 Kanji; version selection memperhitungkan Kanji segment.
- [ ] Test: string pure-Kanji ter-encode di Kanji mode pada panjang bit yang diharapkan; string campuran Jepang/ASCII tersegmen antar Kanji dan byte/alphanumeric secara optimal; guard test yang memastikan input non-Kanji byte-for-byte identik dengan encoder pra-perubahan.

### Checkpoint A — encoder mengeluarkan Kanji segment setia-spec; output non-Kanji byte-identical.

### K5 — Decoder + Round-Trip `(M)`

Tujuan: men-decode Kanji dan membuktikan round-trip, termasuk pada decoder independen.

- [ ] `decodeText` mendapat `case 0b1000`: baca character count Kanji, lalu tiap nilai 13-bit kembali ke rune via tabel.
- [ ] `TestKanjiRoundTrip` meng-encode payload pure-Kanji dan campuran Jepang/ASCII dan memastikan `DecodeBytes` memulihkan teks persisnya; `TestRoundTripWithThirdPartyDecoder` mendapat satu kasus Kanji; gozxing membacanya secara identik.

### K6 — Polish & Rilis `(S)`

Tujuan: memotong `v0.11.0`.

- [ ] README: catatan penggunaan Kanji; butir Limitations "No Kanji mode" diganti (dan angka densitas "4x"-nya yang keliru dikoreksi); Scope dan Roadmap diperbarui; contoh runnable `examples/encode/kanji`.
- [ ] Entri `v0.11.0` di `CHANGELOG.md` plus anchor compare/tag ditulis; dibiarkan unstaged di working tree untuk maintainer (meniru v0.6 sampai v0.10).
- [ ] `go test -race ./...` bersih, gofmt bersih, dan `go generate` mereproduksi tabel yang di-commit byte-for-byte.
- [ ] Tag `v0.11.0` (diserahkan ke maintainer sesuai alur git/rilis yang sudah mapan; anotasi direkomendasikan di percakapan rilis).

---

## 6. Usulan Delta Tata Letak File

```
qrgen/
├── kanji.go                 # baru — codec rune/Shift-JIS/13-bit + eligibility
├── kanji_table.go           # baru, di-generate — peta JIS X 0208 yang di-commit
├── gen_kanji_table.go       # baru — generator //go:build ignore (pakai x/text)
├── kanji_test.go            # baru — batas codec + round-trip tabel penuh
├── mode.go                  # +ModeKanji (1000) + lebar count 8/10/12
├── segment.go               # +ModeKanji di DP dengan eligibility + cost 13-bit
├── encode.go                # writeKanji; Kanji di version selection
├── decode_matrix.go         # decodeText: case 0b1000
├── roundtrip_test.go        # +kasus Kanji gozxing
docs/
├── plan-kanji-mode.md          # file ini
├── plan-kanji-mode.id.md       # padanan Indonesia
└── theory/
    ├── 22-kanji-mode.md         # baru
    └── 22-kanji-mode.id.md      # baru
examples/encode/kanji/
└── main.go                  # baru — encode + decode teks Jepang
```

## 7. Risiko & Catatan Teknis

- **Ukuran dan asal tabel.** JIS X 0208 punya kira-kira 6.900 karakter, jadi peta yang di-generate adalah Go source yang lumayan besar. Mereka di-generate deterministik dari codec Shift-JIS `x/text` dan di-check in; `go generate` harus mereproduksi file yang di-commit byte-for-byte, yang dicek di K6. Regenerasi adalah langkah jarang yang terdokumentasi.
- **Jalur default tak boleh bergeser.** Menambah Kanji ke DP tak boleh mengubah segmentasi input non-Kanji — sebuah rune Kanji-eligible hanya jika tabel memetakannya, dan ASCII tak pernah. Guard test membandingkan output non-Kanji terhadap encoding yang sudah diketahui benar, dan round-trip gozxing yang ada tetap hijau.
- **Bentuk tabel character-count.** `charCountBitsTable` sekarang `[3][3]` diindeks tiga mode data; lebar `[8, 10, 12]` Kanji dan nilai mode-nya butuh entah baris keempat dengan indeks di-remap atau lookup kecil khusus. Apa pun yang dipilih menjaga `CharCountBits` mengembalikan 0 untuk mode non-data (ECI, structured append).
- **Celah range Shift-JIS.** Tak tiap nilai di kedua range Shift-JIS ter-assign; codec harus round-trip hanya lewat tabel, tak pernah lewat aritmetika buta, jadi kode tak ter-assign tak pernah diproduksi atau diterima.
- **Kejujuran densitas.** Dokumen dan README menyatakan 13 bit/char versus 24 untuk byte mode UTF-8 (~1,8x), mengganti "4x" README yang keliru, konsisten dengan kebiasaan proyek mengukur alih-alih menebak.

## 8. Referensi

- ISO/IEC 18004:2015 — mode indicator Kanji `1000`, lebar character-count (8/10/12), dan derivasi dua-byte-Shift-JIS-ke-13-bit dengan dua range offset-nya.
- JIS X 0208 — character set double-byte yang di-encode QR Kanji mode; Shift-JIS adalah serialisasi byte-nya.
- `golang.org/x/text/encoding/japanese` — codec Shift-JIS yang dipakai hanya oleh generator tabel yang diabaikan.
- `docs/theory/02-data-encoding.md`, `09-data-tables.md`, dan `17-optimal-segmentation.md` — catatan mode, tabel, dan DP yang Kanji perluas.

## 9. Pertanyaan Terbuka

Dijawab sebelum milestone terkait dimulai:

- **Sumber generasi tabel.** Generate dari codec Shift-JIS `x/text` (diusulkan, karena x/text sudah modul indirect) versus mem-parse file data pemetaan resmi yang di-check in ke repo. Generator `//go:build ignore` menjaga keduanya di luar runtime graph. Konfirmasi di K3.
- **Bentuk tabel character-count.** Baris `charCountBitsTable` keempat dengan remap indeks, atau lookup lebar Kanji khusus. Konfirmasi di K4.
- **Cost DP dari Kanji eligibility.** Membangun prefix Kanji-eligibility berarti satu lookup tabel per rune saat segmentation; konfirmasi ongkos tambahannya dapat diterima (O(n) dengan lookup map, seorde dengan prefix yang ada) atau apakah perlu men-gate segmentasi Kanji di balik kehadiran rune eligible mana pun.
- **Isi contoh.** Teks sampel Jepang mana yang dikirim di contoh dan test (dijaga pendek, dicek keanggotaan JIS X 0208-nya). Konfirmasi di K5.
