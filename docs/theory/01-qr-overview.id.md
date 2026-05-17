# Ikhtisar QR Code

> Versi English: [01-qr-overview.md](01-qr-overview.md).

## Apa itu QR code

QR (Quick Response) code adalah barcode matriks dua dimensi yang ditemukan pada tahun 1994 oleh Masahiro Hara di Denso Wave untuk pelacakan komponen otomotif. QR menyimpan data dalam grid persegi berisi modul gelap dan terang serta menyertakan redundansi melalui error correction Reed–Solomon, sehingga tetap dapat dibaca meski simbol mengalami kerusakan, kotor, atau terhalang sebagian.

QR distandarisasi oleh ISO/IEC 18004 (edisi terbaru: 2015).

## Struktur simbol

Sebuah simbol QR terdiri dari:

- **Finder pattern** — tiga kotak konsentris besar di pojok kiri-atas, kanan-atas, dan kiri-bawah. Berfungsi membuat scanner menemukan simbol tanpa peduli rotasinya.
- **Separator** — strip terang selebar satu modul di sekeliling tiap finder menghadap area data.
- **Alignment pattern** — kotak konsentris yang lebih kecil (mulai versi 2) untuk mengoreksi distorsi perspektif.
- **Timing pattern** — modul gelap-terang berselang-seling di baris 6 dan kolom 6; memungkinkan scanner mengukur pitch modul.
- **Format information** — 15 bit yang menyimpan level error correction dan mask pattern yang dipakai, ditulis di dua lokasi redundan.
- **Version information** — 18 bit yang menyimpan nomor versi, hadir hanya pada versi 7 ke atas, juga direplikasi di dua lokasi.
- **Modul data dan error correction** — mengisi seluruh sel sisa.
- **Quiet zone** — minimal 4 modul background di sekeliling simbol.

Sketsa ASCII simbol versi 1 (21×21):

```
F F F F F F F . . . . . . . F F F F F F F
F . . . . . F . . . . . . . F . . . . . F
F . X X X . F . . . . . . . F . X X X . F
F . X X X . F . . . . . . . F . X X X . F
F . X X X . F . . . . . . . F . X X X . F
F . . . . . F . . . . . . . F . . . . . F
F F F F F F F . T . . . . . F F F F F F F
. . . . . . . . . . . . . . . . . . . . .
. . . . . . T . . . . . . . . . . . . . .
. . . . . . . . D D D D D D D D D D D D D
. . . . . . . . D D D D D D D D D D D D D
...
```

Keterangan: `F` finder, `T` timing, `D` area data.

## Versi dan ukuran

QR memiliki 40 versi. Versi `v` memiliki panjang sisi:

    side(v) = 21 + 4 · (v − 1)   modul

Jadi versi 1 berukuran 21×21 dan versi 40 berukuran 177×177.

## Level error correction

Empat level redundansi didukung:

| Level | Recovery aproksimasi |
|:-----:|:---------------------|
|   L   | hingga ~7%           |
|   M   | hingga ~15%          |
|   Q   | hingga ~25%          |
|   H   | hingga ~30%          |

Level yang lebih tinggi berarti lebih sedikit data codeword untuk versi yang sama, karena anggaran codeword tetap untuk satu versi dan dibagi antara data dan error correction.

## Kapasitas

Kapasitas bergantung pada tripel (versi, EC level, mode encoding). Dua contoh ujung:

- Versi 1 / EC level M dapat menampung 14 karakter alphanumeric.
- Versi 40 / EC level L dapat menampung 7.089 digit numeric.

Tabel detailnya ada di ISO/IEC 18004:2015 Table 7 dan akan dikodekan di `qrgen/version.go`.

## Alur encode

1. **Pemilihan mode** — pilih mode encoding yang meminimalkan jumlah bit.
2. **Pemilihan versi** — ambil versi terkecil yang kapasitasnya mampu menampung payload pada EC level yang diminta.
3. **Konstruksi bit stream** — mode indicator + character count indicator + payload bits + terminator + pad bytes.
4. **Reed–Solomon** — hitung EC codewords per block lalu interleave.
5. **Konstruksi matrix** — tempatkan functional pattern, kemudian sisipkan bit data dalam pola zig-zag melalui sel-sel sisa.
6. **Masking** — XOR-kan modul area data dengan salah satu dari 8 mask pattern, pilih yang menghasilkan penalty paling kecil.
7. **Format & version info** — tulis metadata yang sudah di-BCH-encode ke modul reserved.
8. **Render** — terjemahkan bit matrix menjadi bitmap PNG beserta quiet zone.

Dokumen-dokumen selanjutnya menjelaskan tiap langkah secara detail.

## Referensi

- ISO/IEC 18004:2015, bagian 6 (struktur simbol) dan 7 (encoding).
- Thonky, "Module Placement in Matrix" — <https://www.thonky.com/qr-code-tutorial/module-placement-matrix>
- Denso Wave, "History of QR Code" — <https://www.qrcode.com/en/history/>
