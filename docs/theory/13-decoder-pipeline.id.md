# Pipeline Decoder

Dokumen ini adalah pandangan-elang tentang bagaimana `qrgen.Decode(image.Image) (string, error)` bekerja end-to-end. Algoritma individual didetailkan di [11-rs-decoding.id.md](11-rs-decoding.id.md) dan [12-image-processing.id.md](12-image-processing.id.md); di sini kita menjelaskan bagaimana mereka berkomposisi, apa yang bisa gagal di tiap tahap, dan jenis error apa yang caller lihat di tiap kasus.

> Versi English: [13-decoder-pipeline.md](13-decoder-pipeline.md).

## Alur end-to-end

```text
image.Image
   │
   ▼ D8: grayscale + binarisasi Otsu
bitmap (lebar × tinggi bool)
   │
   ▼ D9: scan finder-pattern + validasi geometri
3 centre finder + estimasi pitch modul
   │
   ▼ D10: estimasi versi + homography dari 4 korespondensi sudut
matriks homography 3×3
   │
   ▼ D11: refinement alignment-pattern opsional (V2+)
homography yang sudah direfine
   │
   ▼ D12: sampling modul di tiap centre grid
matrix [][]bool (n × n)
   │
   ▼ D4: baca format-info, BCH error-correct → (EC level, mask)
(ECLevel, mask)
   │
   ▼ D5: balikkan zig-zag walk, XOR mask, buang remainder bits
byte stream ter-interleave [data ‖ EC]
   │
   ▼ D6: deinterleave per block, RS decode tiap block
data codeword stream yang sudah dikoreksi
   │
   ▼ D7: parse indikator mode + count, decode payload per mode
teks decoded final
```

Pemisahan horizontal di diagram berkorespondensi rapi dengan pemisahan public API kita: tahap D8..D12 mengubah image jadi matrix, dan tahap D4..D7 mengubah matrix itu jadi teks. `DecodeMatrix` adalah paruh kedua yang diekspose sendiri; `Decode` menjalankan kedua paruh.

## Tanggung jawab tahap & mode kegagalan

| Tahap | Input | Output | Error saat gagal |
|------:|------|--------|------------------|
| D8    | `image.Image` | bitmap | tidak ada (binarisasi tidak pernah gagal) |
| D9    | bitmap | 3 centre finder | `ErrFinderNotFound` |
| D10   | finder | homography | `ErrInvalidVersion` bila versi estimasi di luar range |
| D11   | homography | homography yang sudah direfine | diam-diam fallback ke H tanpa refine |
| D12   | H yang sudah direfine | matrix `[][]bool` | tidak ada (sampling tidak bisa gagal begitu H dihitung) |
| D4    | matrix | (EC, mask) | `ErrFormatUnreadable` bila kedua salinan terlalu korup |
| D5    | matrix + (EC, mask) | byte ter-interleave | tidak ada |
| D6    | ter-interleave + versi + EC | data byte yang sudah dikoreksi | `ErrTooManyErrors` bila ada block yang melebihi kapasitas RS |
| D7    | data byte | teks | `ErrCorruptedPayload` untuk indikator mode atau character count yang mustahil |

Setiap tipe error diekspor dari package agar caller bisa branch dengan `errors.Is` ketimbang parsing pesan error.

## Mengapa pemisahan ini penting

Memisahkan image processing dan matrix processing menjadi dua tahap terpisah membiarkan kita:

1. **Test mereka secara terpisah.** `DecodeMatrix` diberi makan oleh encoder kita sendiri lewat round-trip test — tidak ada image, tidak ada noise numerik homography. Bug di RS decoding atau mask reversal langsung kelihatan.
2. **Ekspose `DecodeMatrix` sebagai entry point publik.** Caller yang sudah punya matrix bersih (mungkin dari pipeline image lain atau rasteriser kustom) bisa skip tahap image yang berat.
3. **Iterasi robustness image tanpa menyentuh yang lain.** Menambahkan local thresholding, multi-finder candidate, atau recovery geometri lebih agresif hanya mengubah tahap D8–D11.

## Filosofi penanganan error

Decoder harus **gagal terang-terangan atau sukses dengan benar** — tidak ada mode "kembalikan teks yang kelihatan plausible". Konkretnya:

- Bila syndrome non-nol dan Berlekamp–Massey menghasilkan polynomial locator berdegree lebih besar dari `n/2`, block tidak bisa dikoreksi dan kita abort dengan `ErrTooManyErrors`.
- Bila Chien search mengembalikan posisi error lebih sedikit dari degree BM, data syndrome inkonsisten secara internal (lebih dari `n` error), dan kita juga abort.
- Bila decoder BCH format-info tidak bisa menemukan satu pun dari 32 codeword valid dalam 3 bit-flip dari salah satu salinan, `ErrFormatUnreadable`.
- Bila indikator mode yang dibaca tidak ada di `{0001, 0010, 0100}` (atau terminator `0000`), `ErrCorruptedPayload`.

Path fallback diam-diam mengundang jenis bug terburuk: kode yang "men-decode" teks yang salah karena matrix sebenarnya tidak terbaca.

## Yang decoder *tidak* lakukan

- Tidak mencari simbol QR *banyak* di image. Triple finder valid pertama yang menang.
- Tidak auto-rotate image sebagai preprocessing. Detektor finder-pattern cukup rotation-aware untuk menemukan finder terlepas dari orientasi, jadi pass rotasi terpisah tidak perlu.
- Tidak menangani simbol QR terbalik (light-on-dark) di v0.2. Sebagian besar scanner melakukannya; kita bisa menambahkan nanti dengan mencoba kedua polaritas warna di tahap binarisasi.
- Tidak mem-parse segmen ECI. Payload byte-mode diinterpretasikan sebagai UTF-8 mencerminkan konvensi encoder.

## Penunjuk implementasi

- `qrgen/decode.go` mengekspos `Decode`, `DecodeMatrix`, `DecodeBytes`. Ini façade tipis di atas helper bertahap di bawahnya.
- `qrgen/decode_image.go` mencakup D8..D12.
- `qrgen/decode_matrix.go` mencakup D5 dan D7.
- `qrgen/rs_decode.go` mencakup internal D3 dan D6.
- `qrgen/format_decode.go` mencakup D4.
- Error tinggal bersebelahan dengan tahap penghasilnya tapi didokumentasikan di `qrgen/decode.go` untuk kemudahan ditemukan.

## Referensi

Lihat [11-rs-decoding.id.md](11-rs-decoding.id.md) dan [12-image-processing.id.md](12-image-processing.id.md) untuk referensi spesifik per tahap. Struktur pipeline keseluruhan mengikuti ISO/IEC 18004:2015 §11 dan diinformasikan oleh implementasi referensi ZXing.
