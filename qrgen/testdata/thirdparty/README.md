# Third-party QR Code fixtures

`qr_symbols.txt` holds QR Code symbols produced by two encoders that are independent of this library, stored as text module matrices so the repository carries no binary fixtures. `thirdparty_test.go` decodes every symbol with `DecodeMatrix` and, for the segno symbols, checks that our encoder pinned to the same version, EC level, and mask reproduces them module for module.

They exist because a decoder tested only on its own encoder's output can share that encoder's mistakes. Until v0.14.1 both sides wrote and read the first format-information copy bit-reversed; every internal round trip passed while the decoder could not read 14 of the 32 format words from any other encoder.

## Contents

- 32 segno symbols at version 1, one per EC level and mask: all 32 format words in both copies.
- 8 segno symbols at version 7 (each EC level, masks 1 and 6) and 1 at version 14: version information and multi-block interleaving.
- 5 zint symbols chosen by zint itself (versions 1 and 2, several EC levels and masks): a second, unrelated encoder. These are decode-only, since zint's segmentation of mixed-mode text may legitimately differ from ours.

## Micro QR symbols

`micro_symbols.txt` holds 34 Micro QR Code symbols from zint, covering all eight symbol types (M1, M2-L, M2-M, M3-L, M3-M, M4-L, M4-M, M4-Q) and all four modes, with capacity-filling inputs, short inputs with pad codewords, the exact-codeword-boundary case (`1234` in M4-Q), and M1/M3's 4-bit final codeword. Each block adds a `stream=` line between the header and the rows: the final message bits (data codewords, the 4-bit half codeword where present, then the EC codewords) in placement order, read back from zint's symbol. The tests force our encoder to each symbol's type and compare that stream bit for bit, then check that our automatic choice of type matches zint's.

zint, not segno, is the reference here. segno's tables are right, but its padding departs from ISO/IEC 18004 in two places: it zero-fills all padding in M1 and M3 instead of alternating the pad codewords, and it adds eight zero bits when the terminator already ends on a codeword boundary. Decoders ignore pad bits, so segno's symbols scan, but byte-exact goldens taken from it would make a correct encoder look wrong.

## rMQR symbols

`rmqr_symbols.txt` holds 70 rMQR Code symbols from zint: all 32 versions at both levels, M and H, with single-mode inputs rotating through numeric, alphanumeric, byte, and Kanji, plus six special cases (a terminator cut to two bits, two terminators ending exactly on a codeword boundary, UTF-8 and Latin-1 ECI headers, and the shortest symbol). Each block adds a `codewords=` line between the header and the rows: the final codeword stream in hex, data and error-correction codewords interleaved, read back from zint's symbol in placement order. The tests force our encoder to each symbol's version and level and compare that stream byte for byte.

It also holds 545 header-only `auto` records: zint's own choice of version and level for numeric inputs across the whole capacity range, with the level unset, M, or H, and with the height free or fixed to each of the six heights. Their text is the first `digits=N` characters of the first 100 digits of pi, `3141592653…170679`, repeated four times. The tests check that our automatic choice matches zint's for every one.

zint, not rmqrcode, is the reference here. rmqrcode agrees with zint and zxing-cpp on every table except two block definitions, R13x27-M and R17x43-M, where it is wrong (docs/theory/28-rmqr.md section 12).

## Regenerating

The file was produced with segno 1.6.6 and with zint through zxing-cpp 2.3.0 (`zxingcpp.create_barcode`). Neither is a dependency of this module; install them in a throwaway virtual environment:

```sh
python3 -m venv /tmp/qrfix && /tmp/qrfix/bin/pip install segno==1.6.6 zxing-cpp==2.3.0
```

Then build each block from `segno.make("SWEEP 42", version=V, error=EC, mask=M, boost_error=False).matrix` for the segno cases, and from `zxingcpp.create_barcode(text, zxingcpp.BarcodeFormat.QRCode).to_image(size_hint=0, with_quiet_zones=False)` (one pixel per module, dark below 128) for the zint cases, writing a header line `@ source=.. version=.. ec=.. mask=.. text=..` followed by one row of `0`/`1` per module row. For the zint cases the EC level and mask are read back from the first format-information copy. Pass `boost_error=False` to segno, otherwise it silently raises the EC level.

For the Micro symbols, call `zxingcpp.create_barcode(text, zxingcpp.BarcodeFormat.MicroQRCode, ec_level=E)` with `E` set to `""` (unset), `"1"`, `"2"`, or `"3"` for L, M, or Q. The letters `"L"`, `"M"`, `"Q"` are accepted but do not select the level, and `"4"` is rejected because Micro QR has no H. Render it the same way at one pixel per module. Read the symbol type and mask from the single format-information copy (column 8 rows 1 to 8 hold bits 0 to 7, row 8 columns 1 to 7 hold bits 14 to 8, XOR `0x4445`), then unmask and read the data modules in placement order to produce the `stream=` line.

For the rMQR symbols, zxing-cpp's `create_barcode` cannot choose a symbol size, so they come from zint through zint-bindings 1.2.2 (`pip install zint-bindings==1.2.2`): set `symbology` to `RMQR`, `input_mode` to `UNICODE` (so Kanji text uses Kanji mode and ECI text is transcoded), `option_1` to 2 for M or 4 for H (leave it unset for the auto records), `option_2` to 1..32 to force a version or 33..38 to fix the height at 7, 9, 11, 13, 15, or 17, and `eci` for the ECI cases. Each row of `encoded_data` packs module `c` into bit `c & 7` of byte `c >> 3`. Read the version and level from the copy beside the finder (bits 0 to 14 at `(1 + i mod 5, 8 + i / 5)`, bits 15 to 17 down column 11 from row 1, XOR `0x1FAB2`), then unmask with QR's mask 4 and read the data modules in placement order, column pairs from the second column on the right, to produce the `codewords=` line. zxing-cpp 2.3.0 read all 70 symbols back.
