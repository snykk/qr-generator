# QR Encoder — Plan Convenience Payload Helpers

Dokumen ini menjelaskan rencana implementasi **convenience payload helpers** yang menargetkan rilis minor `v0.7.0`. Ini melanjutkan fase encoder/output-breadth (SVG di v0.5, optimal segmentation di v0.6) dengan menjadikan payload QR paling umum di dunia nyata — Wi-Fi join, contact card, email, telepon, SMS, geo — sebagai langkah formatting satu-panggilan alih-alih string yang dirakit manual.

> Status: **draft / dokumen hidup.** Milestone P1..P5 dikerjakan bertahap di branch `convenience-helpers`; tiap milestone berupa commit fokus (atau seri commit kecil) yang sudah lengkap dengan tes, mengikuti ritme milestone M, D, T, R, S, dan MM.

> Versi Inggris: [docs/plan-convenience-helpers.md](plan-convenience-helpers.md).

---

## 1. Visi & Tujuan

- Menyediakan **fungsi payload builder** yang mengubah input terstruktur menjadi string yang persis dan ter-escape dengan benar yang diharapkan kamera HP untuk konvensi QR yang dikenal luas: Wi-Fi network join, vCard contact, `mailto:` email, `tel:` telepon, SMS, dan `geo:` lokasi.
- Membuatnya **komposabel dengan setiap format output.** Tiap builder mengembalikan `string`, jadi caller pipe ke `Encode` (PNG), `EncodeSVG` (SVG), atau `Matrix` (`[][]bool`) yang sudah ada. Secara sengaja tidak ada wrapper `EncodeWiFi`-return-PNG: itu akan mengunci helper ke satu format output tepat setelah v0.5 menambah yang kedua, dan `Encode(WiFiPayload(cfg), opts...)` terbaca sama bersihnya.
- Menjaga perubahan **purely additive dan engine-free.** Builder adalah string formatting; mereka tidak menyentuh pipeline encoder maupun decoder. Tidak ada API eksisting yang berubah.
- **Escaping yang benar.** Satu-satunya kompleksitas nyata adalah escaping karakter: Wi-Fi meng-escape `\ ; , : "`, vCard meng-escape `\ ; ,` dan newline, dan bagian query `mailto:`/`geo:` di-percent-encode. Ini persis untuk apa tes berbasis tabel ada.
- Filosofi sama dengan tiap milestone sebelumnya: pure Go, zero runtime dependency (percent-encoding pakai `net/url` dari standard library), spec/reference-first dengan doc bilingual, dan tes berbasis tabel + round-trip.

## 2. Prinsip Desain

1. **Builder mengembalikan string, bukan bytes.** `WiFiPayload(cfg) string`, `VCardPayload(cfg) string`, `MailtoPayload(...) string`, `TelPayload(...) string`, `SMSPayload(...) string`, `GeoPayload(...) string`. Komposabel, minimal, dan dapat di-unit-test tanpa meng-encode apa pun.
2. **Config struct untuk format kaya, plain param untuk yang sederhana.** Wi-Fi dan vCard punya banyak field opsional, jadi mereka mengambil struct (`WiFi`, `VCard`) demi call site yang mudah dibaca dan forward compatibility. `tel`, `mailto`, `sms`, `geo` masing-masing hanya beberapa argumen dan mengambil plain parameter.
3. **Escaping tinggal di satu tempat ter-tes per format.** Sebuah `escapeWiFi` dan `escapeVCard` kecil plus `net/url` untuk percent-encoding. Tiap escaper di-unit-test terhadap karakter yang spec-nya sebutkan.
4. **Output spec-faithful.** Mengikuti konvensi de-facto yang diimplementasi setiap scanner besar: scheme `WIFI:`, vCard 3.0 (`BEGIN:VCARD`…`END:VCARD`), `mailto:` RFC 6068, `tel:` RFC 3966, scheme `SMSTO:`, dan `geo:` RFC 5870. Reference doc merekam tiap-tiap dengan sitasi.
5. **Tidak ada validation theatre.** Builder memformat apa yang diberikan; mereka tidak menolak nomor telepon atau email "invalid" (format bervariasi di seluruh dunia dan library ini bukan validator). Mereka hanya menjamin output ter-escape dengan benar sehingga strukturnya parse.
6. **Tes lebih dulu**, termasuk round-trip: build → `Encode` → `DecodeBytes` → assert text ter-decode sama dengan payload yang dibangun, plus gozxing cross-check supaya decoder independen setuju.

## 3. Cakupan

### Termasuk di v0.7.0

- `WiFiPayload(cfg WiFi) string` — `WIFI:T:<auth>;S:<ssid>;P:<password>;H:<hidden>;;` dengan escaping; mendukung WPA/WEP/nopass dan flag hidden.
- `VCardPayload(cfg VCard) string` — vCard 3.0 dengan field umum (nama terformat + terstruktur, org, title, satu atau lebih tel dan email, url, alamat, note), escaping RFC 6350, line break CRLF.
- `MailtoPayload(addr, subject, body string) string` — `mailto:` RFC 6068 dengan `subject`/`body` ter-percent-encode.
- `TelPayload(number string) string` — `tel:` RFC 3966.
- `SMSPayload(number, message string) string` — scheme `SMSTO:<number>:<message>` yang didukung luas.
- `GeoPayload(lat, lon float64) string` — `geo:lat,lon` RFC 5870 dengan formatting koordinat yang masuk akal.
- Reference doc `docs/theory/18-payload-formats.md` (EN + ID) yang mendokumentasikan tiap scheme dan escaping-nya, dengan sitasi.
- Sebuah example yang bisa dijalankan dan section usage README.

### Belum termasuk

- **Wrapper PNG gaya `EncodeWiFi`.** Ditolak sesuai prinsip 1 (komposabilitas dengan SVG/Matrix).
- **Format contact MeCard.** vCard adalah format contact yang dipilih; MeCard bisa jadi tambahan masa depan kalau diminta.
- **Validasi payload** (kebenaran telepon/email). Builder meng-escape, mereka tidak memvalidasi.
- **Skema calendar (VEVENT), crypto-address, atau EPC payment.** Kemungkinan helper masa depan; tidak di v0.7.
- **Helper `URLPayload`.** URL sudah merupakan payload-nya sendiri — `Encode(url)` cukup — jadi builder khusus hanya menambah permukaan tanpa nilai.

---

## 4. Milestone

Milestone dikerjakan berurutan. **Checkpoint A** (setelah P3) memberi builder dengan coverage escaping penuh. **Checkpoint B** (P5) adalah rilis `v0.7.0`.

### P1 — Plan Doc `(S)`

- [ ] `docs/plan-convenience-helpers.md` dan padanan Indonesia-nya yang mencakup visi, prinsip, cakupan, milestone P1..P5, delta layout file, risiko, referensi, pertanyaan terbuka.

### P2 — Reference Doc Format Payload `(S)`

Goal: mendokumentasikan tiap skema payload dan aturan escaping-nya sebelum kode apa pun mendarat.

- [x] `docs/theory/18-payload-formats.md` — untuk tiap Wi-Fi, vCard, mailto, tel, SMS, geo: template string persis, karakter mana yang harus di-escape dan bagaimana, spec/konvensi yang relevan dengan sitasi, dan contoh terkerjakan. Dibuka dengan menyatakan ini konvensi scanner yang dilapiskan di atas simbol QR, bukan bagian ISO/IEC 18004, dan ditutup dengan rasional compose-bukan-encode plus pointer implementasi. Memaku pertanyaan terbuka dari P1: vCard 3.0 dengan set field FN/N/ORG/TITLE/TEL/EMAIL/URL/ADR/NOTE dan baris tak-terlipat, skema `SMSTO:`, formatting float shortest-exact `geo:`.
- [x] Padanan Indonesia `docs/theory/18-payload-formats.id.md`.
- [x] Mengupdate `docs/theory/README.md` dan `.id.md`: entry 18 di subsection baru "Payload conventions (v0.7.0)" plus baris code-mapping yang merujuk ke `qrgen/payload.go`.

### P3 — Payload Builders + Escaping `(M)`

Goal: builder-nya sendiri, dengan escaping ter-cover penuh.

- [x] `qrgen/payload.go` dengan config struct `WiFi` dan `VCard`, type `WiFiSecurity` (`WiFiWPA`/`WiFiWEP`/`WiFiNoPass`), dan enam builder, plus `escapeWiFi`/`escapeVCard` tak-di-export (via `strings.Replacer`) dan `mailtoEscape` (via `net/url`, mengonversi `+` ke `%20`). Set vCard dilangsingkan ke `Name`, `FamilyName`, `GivenName`, `Org`, `Title`, `Phones`, `Emails`, `URL`, `Address`, `Note` (`TEL`/`EMAIL` tanpa tipe, `ADR` free-form di komponen street, `N` mengisi family/given) — template vCard doc 18 direkonsiliasi agar cocok.
- [x] Perilaku zero-value: opsional kosong dihilangkan (tidak ada separator menggantung); Wi-Fi default ke `WiFiWPA` dan menghilangkan password untuk `nopass`/kosong.
- [x] Tes di `qrgen/payload_test.go`: golden string berbasis tabel untuk tiap builder; escaping Wi-Fi untuk `; , : \ "`; escaping koma/newline vCard, banyak phone/email, dan opsional yang dihilangkan; mailto `%20`-bukan-`+` plus `&`→`%26`; formatting shortest-exact geo dengan kasus tanpa-notasi-saintifik; tes langsung `escapeWiFi`/`escapeVCard`. gofmt-clean, race-clean.

### Checkpoint A — builder menghasilkan payload yang ter-escape benar dan spec-faithful.

### P4 — Validasi Round-Trip + Example `(S)`

Goal: membuktikan payload yang dibangun encode dan decode utuh, termasuk di decoder independen.

- [ ] Tes round-trip: untuk tiap builder, `DecodeBytes(Encode(builder(...)))` harus mengembalikan string persis yang dibangun (ini juga menjalankan segmentation v0.6, karena payload tel/SMS/geo digit-heavy dan akan ter-segmentasi).
- [ ] gozxing cross-check pada satu payload representatif per builder supaya decoder independen mengkonfirmasi byte-nya.
- [ ] Example yang bisa dijalankan `examples/encode/payloads/main.go` yang membangun QR Wi-Fi join dan QR vCard ke PNG dan SVG.

### P5 — Polish & Rilis `(S)`

Goal: memotong `v0.7.0`.

- [ ] README: section usage "Payload helpers" dengan contoh Wi-Fi dan vCard yang menunjukkan komposisi dengan `Encode` dan `EncodeSVG`; baris API-summary untuk enam builder dan struct `WiFi`/`VCard`; update Scope; drop "convenience helpers" dari Roadmap.
- [ ] Entry `CHANGELOG.md` `v0.7.0` plus anchor compare/tag.
- [ ] `go test -race ./...` bersih.
- [ ] Tag `v0.7.0` (diserahkan ke maintainer sesuai alur git/rilis yang disepakati; annotation direkomendasikan di percakapan rilis).

---

## 5. Usulan Delta Layout File

```
qrgen/
├── payload.go            # baru — struct WiFi/VCard + enam builder + escaper
├── payload_test.go       # baru — tes golden + escaping + round-trip
docs/
├── plan-convenience-helpers.md     # versi Inggris
├── plan-convenience-helpers.id.md  # file ini
└── theory/
    ├── 18-payload-formats.md        # baru
    └── 18-payload-formats.id.md     # baru
examples/encode/payloads/
└── main.go               # baru — demo Wi-Fi + vCard ke PNG dan SVG
```

## 6. Risiko & Catatan Teknis

- **Escaping adalah inti segalanya.** Escaping Wi-Fi (`\ ; , : "`) dan vCard (`\ ; ,` + newline) harus persis, atau scanner salah-parse strukturnya (misal `;` di password mengakhiri field lebih awal). Tiap escaper di-unit-test terhadap setiap karakter spesial yang spec-nya daftar.
- **Cakupan percent-encoding.** Komponen query `mailto:` memakai query escaping milik `net/url`, tapi local-part/alamat tidak boleh over-encoded. Reference doc merekam persis bagian mana yang di-encode.
- **Line folding vCard.** vCard 3.0 secara teknis melipat baris panjang pada 75 oktet. Mayoritas scanner mentoleransi baris tak-terlipat, dan folding menambah kerapuhan parsing; v0.7 mengemit baris tak-terlipat dan doc mencatat ini sebagai penyederhanaan sengaja.
- **Tanpa validasi palsu.** Sengaja tidak memvalidasi rentang telepon/email/koordinat; builder meng-escape dan memformat saja. Didokumentasikan supaya caller tidak berharap penolakan input malformed.
- **Komposisi, bukan jalur output baru.** Karena builder mengembalikan string, tidak ada interaksi dengan encoder, segmenter, renderer, atau decoder. Satu-satunya permukaan tes adalah output string plus round-trip keyakinan lewat pipeline yang ada.

## 7. Referensi

- Konvensi Wi-Fi QR — skema URI `WIFI:` seperti diimplementasi app kamera Android/iOS (tanpa RFC formal; format de-facto didokumentasikan oleh `WifiResultParser` proyek ZXing).
- vCard 3.0 — RFC 2426; escaping per RFC 6350 §3.4 (vCard 4.0, aturan escaping yang sama dalam praktik).
- `mailto:` — RFC 6068.
- `tel:` — RFC 3966.
- SMS — skema `SMSTO:` (`SMSMMSResultParser` ZXing); `sms:` per RFC 5724 dicatat sebagai alternatif.
- `geo:` — RFC 5870.
- ISO/IEC 18004:2015 — simbologi QR-nya sendiri; konvensi payload duduk di atasnya.

## 8. Pertanyaan Terbuka

Untuk dijawab sebelum milestone yang bersangkutan dimulai:

- **vCard vs MeCard.** Default ke vCard 3.0 (cocok dengan "EncodeVCard" roadmap); MeCard lebih sederhana dan QR-native tapi kurang ekspresif. Ditinjau lagi hanya kalau MeCard secara eksplisit diinginkan.
- **Set field vCard.** Field mana yang masuk potongan pertama? Diusulkan: FN, N (family/given/additional/prefix/suffix), ORG, TITLE, TEL (banyak, dengan type), EMAIL (banyak), URL, ADR, NOTE. Dikonfirmasi di P3.
- **Skema SMS.** `SMSTO:number:message` (dipilih, dukungan terluas) vs `sms:number?body=message` (RFC 5724). Default `SMSTO:`; dokumentasikan alternatifnya.
- **Presisi geo.** Berapa tempat desimal untuk lat/lon? Default ke `strconv.FormatFloat(..., 'f', -1, 64)` (terpendek eksak), yang menghindari notasi saintifik dan trailing zero.
