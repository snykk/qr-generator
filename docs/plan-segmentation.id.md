# QR Encoder — Plan Mixed-Mode Segmentation

Dokumen ini menjelaskan rencana implementasi **mixed-mode segmentation DP-optimal** di encoder, menargetkan rilis minor `v0.6.0`. Ini melanjutkan fase encoder-breadth yang dibuka SVG renderer (v0.5.0) dan menutup limitasi "greedy mode analyzer" yang sudah lama didokumentasikan di README.

> Status: **draft / dokumen hidup.** Milestone MM1..MM6 dikerjakan bertahap di branch `encoder-segmentation`; tiap milestone berupa commit fokus (atau seri commit kecil) yang sudah lengkap dengan tes, mengikuti ritme M1..M11, D1..D14, T1..T6, R1..R6, dan S1..S6.

> Versi Inggris: [docs/plan-segmentation.md](plan-segmentation.md).

---

## 1. Visi & Tujuan

- Mengganti **single-mode greedy analyzer** milik encoder dengan **dynamic-programming optimal segmentation** yang memecah input menjadi urutan segment numeric / alphanumeric / byte yang meminimalkan total panjang bit ter-encode. Payload seperti `"PHONE: 12345"` saat ini ter-encode sepenuhnya di byte mode (atau alphanumeric kalau memenuhi syarat); segmentation dapat meng-encode teks awal sebagai alphanumeric dan digit akhir sebagai numeric, sering mengecilkan simbol — kadang sampai satu versi penuh.
- Menjaga perubahan **encoder-only tanpa perubahan API publik.** `Encode`, `EncodeToFile`, `EncodeSVG`, dan `Matrix` mempertahankan signature persisnya; caller melihat simbol yang lebih kecil atau sama untuk input yang sama, tidak pernah lebih besar.
- **Tidak butuh kerja decoder.** Bit-stream parser decoder sudah membaca urutan mode segment sembarang (ia loop atas header segment sampai terminator), jadi output ter-segmentasi round-trip lewat decoder kita sendiri dan lewat decoder pihak ketiga tanpa perubahan. Test suite round-trip yang ada memvalidasi ini pada saat segmentation mendarat.
- Mempertahankan **filosofi yang sama**: pure Go, zero runtime dependency, spec-first dengan theory doc bilingual, dan tes golden / round-trip / property.
- **Menjamin tidak ada regresi.** Input pure-numeric, pure-alphanumeric, atau pure-byte harus tetap menghasilkan satu segment yang byte-nya identik dengan output hari ini, supaya golden fixture yang ada dan round-trip gozxing tetap hijau.

## 2. Prinsip Desain

1. **DP atas posisi, di-key oleh mode.** Algoritma optimal-segmentation klasik (Nayuki) berjalan dari kiri ke kanan, melacak biaya bit minimum untuk meng-encode prefix yang berakhir di tiap dari tiga mode, dengan transisi "perpanjang segment saat ini" dan "ganti mode (membayar header baru)". Biaya sebuah segment adalah `4 (mode indicator) + mode.CharCountBits(v) + payloadBitLength(mode, segmentText)`.
2. **Segmentation bergantung version-group.** `CharCountBits(v)` berubah lintas tiga version group (1–9, 10–26, 27–40), jadi split optimal dapat berbeda per group. Pemilihan versi karena itu menghitung segmentation optimal *untuk tiap versi kandidat* (atau per group) dan memilih versi terkecil yang panjang ter-segmentasinya muat. Empat puluh iterasi DP O(n) itu murah.
3. **Hormati batas rune UTF-8.** Karakter numeric dan alphanumeric adalah ASCII single-byte; rune multi-byte apa pun hanya dapat tinggal di byte segment dan menyumbang panjang byte UTF-8 penuhnya ke count byte-mode. DP tidak boleh memecah rune multi-byte lintas batas segment. Mengoperasikan DP atas rune (bukan byte mentah) sambil menghitung biaya byte-mode segment dalam byte menjaga ini tetap benar.
4. **Subsume, jangan special-case.** DP harus menghasilkan persis satu segment untuk input homogen, byte-identik dengan jalur single-mode saat ini, supaya greedy analyzer menjadi special case yang terbukti, bukan jalur paralel yang bisa melenceng.
5. **Tidak ada permukaan publik baru.** Tidak ada option baru, tidak ada exported function atau type baru. `segment` dan `segmentText` tidak di-export. Helper greedy `analyzeMode` boleh tetap ada sebagai helper internal (dan untuk tes) tapi `encodeText` lewat segmenter.
6. **Tes lebih dulu.** Tiap milestone disertai Go test berbasis tabel: kebenaran biaya DP, optimality-vs-greedy pada payload campuran, invariant identitas input-homogen, kasus batas UTF-8, dan round-trip end-to-end.

## 3. Cakupan

### Termasuk di v0.6.0

- Type `segment` (`{mode Mode, text string}`) dan `segmentText(text string, v Version) []segment`, segmenter DP-optimal, di file baru `qrgen/segment.go`.
- Helper bit-length untuk sebuah segmentation pada versi tertentu, dipakai ulang oleh DP dan pemilihan versi.
- `selectVersion` dikerjakan ulang untuk mensize segmentation optimal per versi kandidat.
- `encodeText` dikerjakan ulang untuk menulis urutan blok `[mode indicator][char count][payload]`, lalu terminator + bit padding + pad bytes tunggal yang dibagi, persis seperti hari ini.
- Theory doc `docs/theory/17-optimal-segmentation.md` (EN + ID) yang membahas DP, cost model, interplay version-group, aturan batas UTF-8, dan identitas input-homogen.
- Validasi: tes optimality dan identitas, round-trip decoder + gozxing, dan benchmark encoder (DP menambah kerja ke hot path encode, jadi ini harus diukur).

### Belum termasuk

- **ECI segment dan Kanji mode.** Segmentation bekerja dalam tiga mode yang ada; ECI/Kanji tetap roadmap item terpisah. (Kanji, setelah ditambahkan, akan menjadi mode keempat yang dapat ditarget DP — dicatat untuk masa depan.)
- **Option publik "matikan segmentation".** Segmentation strictly lebih-baik-atau-sama, jadi tidak ada alasan meng-expose toggle; ditinjau lagi hanya kalau caller nyata butuh parity byte-for-byte dengan encoder lain.
- **Structured append.** Tidak berubah; roadmap item terpisah.
- **Perubahan decoder.** Tidak ada — ia sudah mem-parse stream multi-segment.

---

## 4. Milestone

Milestone dikerjakan berurutan. **Checkpoint A** (setelah MM4) memberi encoder ter-segmentasi yang bekerja dan tervalidasi round-trip. **Checkpoint B** (MM6) adalah rilis `v0.6.0`.

### MM1 — Plan Doc `(S)`

Goal: dokumen ini dan padanan Indonesia-nya, di-commit sebelum kode atau theory apa pun mendarat.

- [ ] `docs/plan-segmentation.md` dan `docs/plan-segmentation.id.md` yang mencakup visi, prinsip, cakupan, milestone MM1..MM6, delta layout file, risiko, referensi, pertanyaan terbuka.

### MM2 — Theory Doc Optimal-Segmentation `(S)`

Goal: menutupi algoritma dan subtletinya di `docs/theory/` sebelum kode apa pun mendarat.

- [x] `docs/theory/17-optimal-segmentation.md` — delapan bagian: kenapa greedy meninggalkan bit terbuang (dengan tabel densitas bit/char), cost model per-segment, formulasi DP (state, transisi, base case, traceback), contoh terkerjakan yang *benar* `"Order #1234567890"` (greedy byte 148 bit vs byte+numeric 116 bit pada V1) plus counter-example `"PHONE: 12345"` yang menunjukkan run 5-digit terlalu pendek untuk dipecah dan menyatakan break-even ~7-digit (dari alpha) / ~4-digit (dari byte), interplay version-group dan recompute per-versi, aturan batas rune UTF-8, dan jaminan identitas input-homogen.
- [x] Padanan Indonesia `docs/theory/17-optimal-segmentation.id.md`.
- [x] Mengupdate `docs/theory/README.md` dan `.id.md`: entry 17 di subsection baru "Encoder completeness (v0.6.0)" plus baris code-mapping yang merujuk ke `qrgen/segment.go`; juga mengoreksi frasa "small files" di entry-16 supaya cocok dengan koreksi ukuran v0.5. Cross-link dari doc 02 (kedua bahasa): catatan greedy-analyzer-nya kini menunjuk maju ke doc 17 dan menyatakan segmentation rilis di v0.6.

### MM3 — Type `segment` + Segmenter DP `(M)`

Goal: segmenter-nya sendiri, belum ada integrasi encoder.

- [ ] `qrgen/segment.go` dengan struct `segment` dan `segmentText(text string, v Version) []segment` yang mengimplementasi DP, plus `segmentsBitLength(segs []segment, v Version) int`.
- [ ] Menangani string kosong dengan benar (satu segment kosong, atau hasil kosong eksplisit yang diperlakukan encoder sebagai segment numeric panjang-nol, cocok dengan perilaku input-kosong hari ini).
- [ ] UTF-8: iterasi rune; digit/alnum ASCII eligible-DP untuk numeric/alphanumeric; selainnya memaksa byte mode; biaya byte-mode menghitung byte UTF-8.
- [ ] Tes di `qrgen/segment_test.go`: input homogen mengembalikan satu segment dengan mode yang diharapkan (invariant identitas); input campuran gaya `"PHONE: 12345"` mengembalikan split alphanumeric+numeric yang diharapkan dengan jumlah bit strictly lebih kecil dari encoding single-mode; segmentation tidak pernah lebih buruk dari greedy untuk sweep payload; kasus batas version-group (teks sama, versi 9 vs 10 vs 27) menghitung ulang dengan benar; payload UTF-8 menjaga rune multi-byte utuh di byte segment.

### Checkpoint A — segmenter benar dan terbukti tidak pernah lebih buruk dari greedy.

### MM4 — Integrasi Encoder `(M)`

Goal: mengarahkan encoder lewat segmenter.

- [ ] Kerjakan ulang `selectVersion` untuk memilih versi terkecil yang segmentation optimal-nya muat (header `4`-bit-per-segment termasuk), memakai `segmentsBitLength`.
- [ ] Kerjakan ulang `encodeText` untuk menghitung segmentation bagi versi terpilih, menulis tiap `[mode indicator][char count][payload]` segment via `writeNumeric/Alphanumeric/Byte` yang ada, lalu terminator + bit padding + pad bytes tunggal yang dibagi. Sesuaikan cek kapasitas `forceVersion` untuk mensize segmentation alih-alih single mode. Rekonsiliasi return value `m Mode` (internal, saat ini dibuang `buildMatrix`) — entah drop dari signature atau kembalikan mode representatif; catat pilihannya di doc.
- [ ] Pertahankan `analyzeMode` sebagai helper internal (masih dirujuk tes) tapi berhenti memakainya untuk keputusan kapasitas.
- [ ] Tes: round-trip `Encode`/`Matrix` end-to-end lewat `DecodeMatrix`/`DecodeBytes` kita sendiri untuk payload campuran; invariant output byte-identik input-homogen terhadap beberapa golden fixture; `WithVersion`/`WithMask` tetap dihormati; `ErrCapacityExceeded` tetap menyala ketika bahkan V40 tidak dapat menampung segmentation (yang kini lebih kecil).

### MM5 — Validasi & Benchmark `(M)`

Goal: membuktikan kemenangannya dan menjaga hot path.

- [ ] Assertion optimality: tabel payload campuran di mana versi/jumlah-bit ter-segmentasi strictly lebih baik dari baseline greedy single-mode, dengan angka yang diharapkan persis dicatat.
- [ ] Cross-validation: perluas tes round-trip gozxing (dan round-trip decoder kita sendiri) dengan payload ter-segmentasi supaya decoder independen mengkonfirmasi stream ter-segmentasi valid-spec.
- [ ] No-regression: konfirmasi input pure-numeric / pure-alphanumeric / pure-byte tetap cocok dengan output golden-nya byte-for-byte.
- [ ] Benchmark: DP jalan per versi kandidat, jadi ukur `BenchmarkEncodeSmall` / `URL` / `MultiBlock` / `Large` terhadap baseline v0.5 dan catat delta-nya. Kalau DP per-versi terukur hot, cache segmentation lintas versi dalam satu group (CharCountBits sama) sebagai optimisasi.
- [ ] `go test -race ./...` bersih.

### MM6 — Polish & Rilis `(S)`

Goal: memotong `v0.6.0`.

- [ ] README: hapus bullet **"Greedy mode analyzer"** dari `## Limitations`; sebut optimal segmentation di Library usage atau catatan singkat; update `## Roadmap` (drop "mixed-mode segmentation" dari bullet encoding-completeness, menyisakan ECI + Kanji).
- [ ] Update komentar doc `analyzeMode` dan `docs/theory/02-data-encoding.md` supaya tidak lagi mendeskripsikan segmentation sebagai ditunda.
- [ ] Entry `CHANGELOG.md` `v0.6.0` di bawah Added / Changed / Validated plus anchor compare/tag.
- [ ] Tag `v0.6.0` (diserahkan ke maintainer sesuai alur rilis yang sudah disepakati; annotation direkomendasikan di percakapan rilis).

---

## 5. Usulan Delta Layout File

```
qrgen/
├── encode.go            # eksisting — selectVersion + encodeText dikerjakan ulang untuk segment; analyzeMode dipertahankan sebagai helper
├── segment.go           # baru — type segment, DP segmentText, segmentsBitLength
├── segment_test.go      # baru — tes kebenaran DP, optimality, identitas, UTF-8
├── encode_test.go       # eksisting — mendapat tes round-trip payload campuran + invariant identitas
├── bench_test.go        # eksisting — diukur ulang; kemungkinan cache segmentation per-group
└── roundtrip_test.go    # tes gozxing eksisting — mendapat payload ter-segmentasi
docs/
├── plan-segmentation.md          # versi Inggris
├── plan-segmentation.id.md       # file ini
└── theory/
    ├── 02-data-encoding.md        # catatan greedy-analyzer menunjuk maju ke doc 17
    ├── 17-optimal-segmentation.md     # baru
    └── 17-optimal-segmentation.id.md  # baru
```

## 6. Risiko & Catatan Teknis

- **Sirkularitas version/segmentation.** Segmentation optimal bergantung pada versi (via `CharCountBits`), tapi pemilihan versi bergantung pada panjang ter-encode, yang bergantung pada segmentation. Diselesaikan dengan menghitung segmentation per versi kandidat di dalam loop pemilihan; kebenaran-nya unconditional, dan cache per-group menghilangkan biaya nyata apa pun.
- **Kebenaran UTF-8.** Trap paling tajam: rune multi-byte tidak boleh dipecah, dan panjang byte-mode dihitung dalam byte, bukan rune. DP iterasi rune dan menghitung biaya byte segment dengan `len(string(runes))`; tes mencakup payload emoji / CJK.
- **Invariant identitas.** Risiko regresi terbesar adalah mengubah byte input homogen. Tes khusus meng-assert kesamaan byte-for-byte dengan output pra-segmentation untuk string pure-numeric / pure-alpha / pure-byte; ini juga menjaga golden fixture v0.1 dan round-trip gozxing tetap valid.
- **Biaya hot-path.** Menjalankan DP O(n) hingga 40 kali per encode lebih banyak kerja dari scan single-mode O(n) lama. Untuk payload khas ini negligible, tapi benchmark harus mengkonfirmasinya; cache per-version-group (tiga komputasi alih-alih empat puluh) adalah fallback kalau dibutuhkan.
- **Return value `m Mode`.** `encodeText` saat ini mengembalikan mode tunggal yang dibuang `buildMatrix`. Segmentation tidak punya mode tunggal; signature-nya mesti drop return atau kembalikan nilai representatif. Internal-only, tapi layak dikerjakan bersih supaya tidak ada API yang menyesatkan.
- **Mask/penalty tidak terpengaruh.** Segmentation mengubah data bit stream, bukan konstruksi matrix, masking, atau rendering. Tahap-tahap itu dan tesnya tidak tersentuh.

## 7. Referensi

- ISO/IEC 18004:2015 — klausa 7.4 (data encoding, mode segment dan mode indicator), klausa 7.4.1 (mencampur mode dalam satu simbol).
- Project Nayuki — *Optimal text segmentation for QR Codes*: <https://www.nayuki.io/page/optimal-text-segmentation-for-qr-codes>. Formulasi dynamic-programming yang diadopsi di sini.
- `docs/theory/02-data-encoding.md` — catatan mode/character-count yang ada, diperluas oleh doc 17.
- `docs/theory/09-data-tables.md` — `CharCountBits` per version group dan tabel nilai alphanumeric.

## 8. Pertanyaan Terbuka

Untuk dijawab sebelum milestone yang bersangkutan dimulai:

- **Drop atau pertahankan return `m Mode` dari `encodeText`?** Condong ke drop karena internal dan dibuang; mode representatif akan menyesatkan untuk encode multi-segment. Diselesaikan di MM4.
- **Cache segmentation per-version-group dari awal, atau hanya kalau benchmark menuntut?** Default: implementasi komputasi per-versi sederhana dulu, ukur di MM5, dan tambah cache group tiga-entry hanya kalau delta-nya material. Kebenaran dulu.
- **Penanganan string kosong.** Hari ini payload kosong melaporkan Numeric dan meng-encode segment numeric panjang-nol. Pertahankan perilaku persis itu lewat segmenter supaya output tidak berubah untuk edge case ini.
- **Apakah `analyzeMode` harus dihapus seluruhnya?** Ia disubsume oleh DP untuk kasus single-segment, tapi ia kecil, self-documenting, dan dipakai di tes; default ke mempertahankannya sebagai helper internal kecuali ia menjadi dead code.
