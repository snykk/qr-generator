# Rendering

Begitu matrix `[][]bool` selesai, rendering ke bitmap menjadi straightforward. Dokumen ini mencatat keputusan untuk renderer PNG.

> Versi English: [08-rendering.md](08-rendering.md).

## Pemetaan modul → piksel

Tiap modul QR menjadi blok persegi `s × s` piksel, di mana `s` adalah *module size* yang dapat dikonfigurasi (default 8 px pada CLI kita). Panjang sisi gambar output dalam piksel:

    pixels = s · (n + 2·q)

dengan `n` adalah panjang sisi matrix dalam modul dan `q` adalah lebar quiet zone dalam modul (default 4, sesuai spec).

## Quiet zone

ISO/IEC 18004:2015 §6.3.8 mewajibkan minimal 4 modul background di seluruh sisi simbol. Kita selalu me-render quiet zone secara eksplisit ke output; kita **tidak** mengandalkan caller meninggalkan whitespace di sekeliling gambar. Dengan begitu output aman di-embed di latar belakang apa pun.

## Model warna

Renderer default menghasilkan gambar grayscale 8-bit (`image.Gray`), dengan modul gelap pada `0x00` dan modul terang pada `0xFF`. Bila caller mengoper warna kustom via `WithColors`, kita beralih ke `image.RGBA` dan menulis nilai RGBA yang diberikan langsung. Tidak ada anti-aliasing — modul di-render sebagai persegi tajam karena decoder QR mengandalkan tepi yang tegas.

## Encoding

Kita encode melalui `image/png` dari standard library Go. Default encoder sudah memadai; kita tidak butuh transparansi atau interlacing.

## Peringatan kontras

Scanner QR butuh kontras cukup antara modul gelap dan terang. Renderer secara default akan menolak pasangan foreground/background yang luminance contrast ratio-nya di bawah 4:1 (ambang yang WCAG 2.1 pakai untuk teks normal). Ini soft check; caller dapat opt-out lewat `WithSkipContrastCheck(true)` bila mereka yakin tahu konsekuensinya.

## Kenapa PNG dulu

- PNG bersifat lossless sehingga tepi modul tetap tajam — penting untuk decodability.
- Encoder standard library sudah berada dalam scope; tidak butuh dependency baru.
- Format lain dapat ditambahkan kemudian sebagai sibling render function yang berbagi struct `renderOptions` yang sama dan signature `func(m *matrix, opts renderOptions) ([]byte, error)`, tanpa membongkar public API. SVG tiba lewat cara ini di v0.5 — lihat [16-svg-rendering.id.md](16-svg-rendering.id.md), bagian 7, untuk alasan kenapa interface bersama akan prematur untuk dua renderer non-polimorfik.

## Penunjuk implementasi

- `qrgen/render_png.go`: `renderPNG(m *Matrix, opts renderOpts) ([]byte, error)`.
- Renderer sebaiknya menerima *matrix yang belum dirender* dan struct opsi, bukan image yang setengah jadi, agar tetap trivially testable.

## Referensi

- ISO/IEC 18004:2015, §6.3.8 (Quiet zone), §11 (Reference decode algorithm).
- W3C, *Web Content Accessibility Guidelines (WCAG) 2.1*, §1.4.3 — formula contrast ratio untuk soft check.
- Standard library Go, dokumentasi `image` dan `image/png`.
