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

## Regenerating

The file was produced with segno 1.6.6 and with zint through zxing-cpp 2.3.0 (`zxingcpp.create_barcode`). Neither is a dependency of this module; install them in a throwaway virtual environment:

```sh
python3 -m venv /tmp/qrfix && /tmp/qrfix/bin/pip install segno==1.6.6 zxing-cpp==2.3.0
```

Then build each block from `segno.make("SWEEP 42", version=V, error=EC, mask=M, boost_error=False).matrix` for the segno cases, and from `zxingcpp.create_barcode(text, zxingcpp.BarcodeFormat.QRCode).to_image(size_hint=0, with_quiet_zones=False)` (one pixel per module, dark below 128) for the zint cases, writing a header line `@ source=.. version=.. ec=.. mask=.. text=..` followed by one row of `0`/`1` per module row. For the zint cases the EC level and mask are read back from the first format-information copy. Pass `boost_error=False` to segno, otherwise it silently raises the EC level.

For the Micro symbols, call `zxingcpp.create_barcode(text, zxingcpp.BarcodeFormat.MicroQRCode, ec_level=E)` with `E` set to `""` (unset), `"1"`, `"2"`, or `"3"` for L, M, or Q. The letters `"L"`, `"M"`, `"Q"` are accepted but do not select the level, and `"4"` is rejected because Micro QR has no H. Render it the same way at one pixel per module. Read the symbol type and mask from the single format-information copy (column 8 rows 1 to 8 hold bits 0 to 7, row 8 columns 1 to 7 hold bits 14 to 8, XOR `0x4445`), then unmask and read the data modules in placement order to produce the `stream=` line.
