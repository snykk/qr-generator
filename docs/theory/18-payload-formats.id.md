# Format Payload untuk Convenience Helpers

Simbol QR sekadar membawa string; yang membuat HP menawarkan "join jaringan Wi-Fi ini" atau "tambah kontak ini" adalah **konvensi pada string itu** yang dikenali app scanner. Konvensi ini bukan bagian dari ISO/IEC 18004 — mereka duduk di atasnya. Dokumen ini merekam template string persis dan aturan escaping yang dihasilkan payload builder v0.7 (`qrgen/payload.go`), dengan sitasi dan contoh terkerjakan untuk tiap-tiapnya. Builder hanya memformat dan meng-escape; mereka tidak memvalidasi input.

> Versi Inggris: [18-payload-formats.md](18-payload-formats.md).

## 1. Wi-Fi network join

Template:

```text
WIFI:T:<auth>;S:<ssid>;P:<password>;H:<hidden>;;
```

- `T` — tipe autentikasi: `WPA` (mencakup WPA/WPA2/WPA3), `WEP`, atau `nopass` untuk jaringan terbuka. Untuk `nopass` field `P` kosong atau dihilangkan.
- `S` — SSID (nama jaringan).
- `P` — password.
- `H` — `true` kalau SSID hidden; dihilangkan selain itu.
- Payload diakhiri dengan double semicolon `;;`.

**Escaping.** Di dalam `S` dan `P`, karakter `\ ; , : "` bersifat spesial dan harus di-backslash-escape (`\;`, `\,`, `\:`, `\\`, `\"`). Konvensi SSID leading-hex juga ada tapi jarang dibutuhkan dan di luar cakupan. Tidak ada RFC formal; format de-facto adalah yang dibaca `WifiResultParser` ZXing dan diimplementasi app kamera Android/iOS.

Contoh terkerjakan — SSID `Cafe; Wifi`, WPA, password `p:w,d\1`:

```text
WIFI:T:WPA;S:Cafe\; Wifi;P:p\:w\,d\\1;;
```

## 2. vCard contact

Builder mengemit vCard 3.0:

```text
BEGIN:VCARD
VERSION:3.0
FN:<formatted name>
N:<family>;<given>;<additional>;<prefix>;<suffix>
ORG:<org>
TITLE:<title>
TEL;TYPE=CELL:<number>
EMAIL:<address>
URL:<url>
ADR:;;<street>;<city>;<region>;<postcode>;<country>
NOTE:<note>
END:VCARD
```

Baris dipisahkan oleh CRLF (`\r\n`). Hanya field yang caller isi yang diemit; opsional kosong dilewati. `N` adalah field terstruktur dengan lima komponen dipisah-semicolon; `ADR` punya tujuh (dua pertama — post-office-box dan extended address — konvensional dibiarkan kosong).

**Escaping** (RFC 6350 §3.4, yang vCard 3.0 ikuti dalam praktik): di dalam text value, escape `\` jadi `\\`, `;` jadi `\;`, `,` jadi `\,`, dan newline jadi `\n`. Semicolon struktural yang memisahkan komponen `N`/`ADR` *tidak* di-escape — hanya semicolon literal di dalam value komponen.

vCard 3.0 juga mendefinisikan line folding pada 75 oktet; builder mengemit baris tak-terlipat karena setiap scanner umum mentoleransinya dan folding menambah kerapuhan parsing. Ini penyederhanaan sengaja.

Contoh terkerjakan — nama "Ada Lovelace", org "Analytical Engine, Ltd":

```text
BEGIN:VCARD
VERSION:3.0
FN:Ada Lovelace
N:Lovelace;Ada;;;
ORG:Analytical Engine\, Ltd
END:VCARD
```

## 3. Email (`mailto:`)

Template (RFC 6068):

```text
mailto:<addr>?subject=<subject>&body=<body>
```

- `<addr>` adalah penerima; query `?subject=…&body=…` dihilangkan seluruhnya ketika keduanya kosong.
- `subject` dan `body` di-percent-encode sebagai komponen query. Spasi menjadi `%20` (bukan `+`, yang merupakan konvensi form-encoding yang tidak dipakai `mailto:`), dan `&`, `=`, serta byte non-ASCII di-percent-encode.

Contoh terkerjakan — ke `ada@example.com`, subject `Hello there`, body `Hi & bye`:

```text
mailto:ada@example.com?subject=Hello%20there&body=Hi%20%26%20bye
```

## 4. Telepon (`tel:`)

Template (RFC 3966):

```text
tel:<number>
```

Nomor diemit apa adanya (biasanya dalam bentuk internasional `+CC...`). Tidak ada escaping yang dibutuhkan untuk karakter digit/`+`/`-` yang dipakai nomor telepon; builder tidak memformat ulang atau memvalidasi.

Contoh terkerjakan:

```text
tel:+15551234567
```

## 5. SMS

Template (skema `SMSTO:` yang didukung luas yang dibaca `SMSMMSResultParser` ZXing):

```text
SMSTO:<number>:<message>
```

Nomor dan pesan dipisahkan oleh colon; pesan berlanjut sampai akhir payload. Bentuk `sms:<number>?body=<message>` RFC 5724 adalah alternatif yang sebagian app lebih suka, dicatat di sini tapi bukan default.

Contoh terkerjakan — nomor `+15551234567`, pesan `on my way`:

```text
SMSTO:+15551234567:on my way
```

## 6. Lokasi geo (`geo:`)

Template (RFC 5870):

```text
geo:<lat>,<lon>
```

Latitude dan longitude dalam derajat desimal. Builder memformat tiap-tiapnya dengan representasi desimal eksak terpendek (`strconv.FormatFloat(v, 'f', -1, 64)`), yang menghindari notasi saintifik (parser `geo:` akan menolak `1e2`) dan trailing zero.

Contoh terkerjakan — 37.422°, -122.084°:

```text
geo:37.422,-122.084
```

## 7. Kenapa ini compose, bukan encode

Setiap builder mengembalikan `string`. String itu adalah payload QR biasa, jadi ia mengalir lewat pipeline yang ada tanpa perubahan: `Encode(WiFiPayload(cfg))` untuk PNG, `EncodeSVG(WiFiPayload(cfg))` untuk SVG, `Matrix(...)` untuk grid mentah. Yang digit-heavy (`tel`, `SMSTO`, `geo`) juga otomatis diuntungkan oleh optimal segmentation v0.6, yang mengemas run digit panjangnya ke numeric segment. Tidak ada jalur encoding payload-spesifik; helper adalah konstruksi string murni yang di-tes baik sebagai golden string maupun lewat round trip build-encode-decode.

## Pointer Implementasi

- `qrgen/payload.go` menampung config struct `WiFi` dan `VCard`, enam builder, dan helper `escapeWiFi` / `escapeVCard`; percent-encoding `mailto:`/query memakai `net/url`.
- Tiap escaper di-unit-test terhadap setiap karakter spesial yang konvensinya daftar; tiap builder punya tes golden-string dan tes round-trip lewat `Encode`/`DecodeBytes`.
- Field opsional kosong dihilangkan dari output alih-alih diemit kosong, jadi tidak ada separator menggantung.

## Referensi

- Skema Wi-Fi `WIFI:` — `WifiResultParser` ZXing (de-facto; tanpa RFC). <https://github.com/zxing/zxing>
- vCard 3.0 — RFC 2426; escaping text-value per RFC 6350 §3.4.
- `mailto:` — RFC 6068.
- `tel:` — RFC 3966.
- SMS `SMSTO:` — `SMSMMSResultParser` ZXing; `sms:` per RFC 5724.
- `geo:` — RFC 5870.
