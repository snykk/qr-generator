# Symbol Inspection

Docs 5 to 7, 26, and 28 describe how a symbol is built: function patterns first, then the codewords placed around them, then a mask, then the format information. The finished grid hides those steps; a dark module could be part of a finder, a format bit, or a data bit that the mask inverted. This document is about taking a finished symbol apart again: what `qrgen.Inspect` reports, how the encoder is made to report it without changing what it draws, and how the web demo runs the library in a browser to show it.

> Indonesian version: [29-symbol-inspection.id.md](29-symbol-inspection.id.md).

## 1. The layers of a symbol

Every module belongs to one of two kinds. **Function modules** are fixed by the layout: finder patterns, separators, timing patterns, alignment patterns, the dark module, and, in rMQR, the finder sub-pattern and corner patterns. The format and version information also sit in fixed places, though their values depend on the symbol. **Message modules** carry the codeword stream, data codewords and error-correction codewords interleaved, plus a few remainder bits where the stream runs out before the modules do.

The mask touches only message modules. So a symbol is three layers on top of each other: the function modules, the message as placed, and the mask pattern, which inverts the message modules it selects. `Matrix` returns their combination. `Inspect` returns each layer, and for every message module, the codeword bit it carries.

## 2. Module roles

`ModuleRole` names eleven roles: data, remainder, finder, separator, timing, alignment, format, version, dark module, finder sub-pattern, and corner pattern. Data and remainder are message modules; the rest are function modules (`IsFunction`). Counting the roles in a symbol is a good check on the layout, since every count follows from the standard:

| Symbol | Finder | Separator | Timing | Alignment | Format | Version | Other | Data | Remainder |
|---|---|---|---|---|---|---|---|---|---|
| QR V1 (21 × 21) | 147 | 45 | 10 | 0 | 30 | 0 | dark module 1 | 208 | 0 |
| QR V7 (45 × 45) | 147 | 45 | 48 | 150 | 30 | 36 | dark module 1 | 1568 | 0 |
| QR V40 (177 × 177) | 147 | 45 | 272 | 1150 | 30 | 36 | dark module 1 | 29648 | 0 |
| Micro M1 (11 × 11) | 49 | 15 | 6 | 0 | 15 | 0 | | 36 | 0 |
| Micro M4 (17 × 17) | 49 | 15 | 18 | 0 | 15 | 0 | | 192 | 0 |
| rMQR R7x43 | 49 | 7 | 58 | 18 | 36 | 0 | sub-pattern 25, corners 4 | 104 | 0 |
| rMQR R11x27 | 49 | 15 | 41 | 0 | 36 | 0 | sub-pattern 25, corners 9 | 120 | 2 |
| rMQR R17x139 | 49 | 15 | 297 | 72 | 36 | 0 | sub-pattern 25, corners 9 | 1856 | 4 |

Three overlaps decide some of these numbers, and each is a place where a naive count goes wrong.

- **Alignment over timing.** From version 7 on, QR places alignment patterns centred on row 6 and column 6, on top of the timing patterns, and each takes over five timing modules. With n the side and k the number of alignment centres, QR's timing count is 2(n − 16) − 10(k − 2) for versions 7 and up: 48 in V7 (k = 3) and 272 in V40 (k = 7). The alignment count is 25(k² − 3).
- **QR's format area and the timing modules.** The area QR reserves for format information, row 8 and column 8 near the top-left finder, includes (6, 8) and (8, 6), which belong to the timing patterns. They stay timing modules; the format information has 2 × 15 = 30 modules.
- **R7 symbols.** At height 7 the finder fills the whole height, so its separator is column 7 alone (7 modules, against 15 from height 9), and the bottom-left corner pattern, the three dark modules (h − 1, 0) to (h − 1, 2), falls on the finder's bottom edge. Those modules are counted as finder modules, which leaves 4 corner-pattern modules at the top-right. From height 11 the bottom-left corner pattern has 5 modules of its own, hence 9.

Data modules number 8 times the codeword count (26 codewords in V1-M, 13 in R7x43), except in Micro QR M1 and M3, whose last data codeword is 4 bits long (section 4).

## 3. Recording roles while building

`Inspect` does not describe the layout a second time. A second description could drift from the code that draws the symbol, and a drawing that is right with a description that is wrong would teach the wrong thing. Instead, `Inspect` runs the encoder itself (`buildMatrix`) with a trace attached to the matrix. Each placement function tags the modules it draws with their role: `placeSingleFinder` tags finder and separator modules, the timing and alignment functions tag theirs, and so on. A pattern drawn over an earlier one takes its modules, as alignment does over timing. A step that only reserves modules, such as QR's format area, tags just the modules no earlier pattern holds, which is what keeps (6, 8) and (8, 6) timing modules.

The zigzag placement (`placeBits`) records, in each message module, the index of the stream bit it writes. After the mask and the format information, `Inspect` reads the trace and copies the finished modules. Because the trace rides on the very code that encodes, `Modules` equals what `Matrix` returns by construction, and a test checks it across all three symbologies.

The trace is a pointer that only `Inspect` sets. Every other encoder builds an untraced matrix, so tracing costs them a nil check per module.

## 4. From stream bits to codewords

For QR and rMQR, the stream is a whole number of codewords, most significant bit first. Stream bit b is bit 7 − (b mod 8) of codeword ⌊b / 8⌋, counting bit 7 as the most significant. Bits past the last codeword are remainder bits: 0, 3, 4, or 7 in QR depending on the version, and 0 to 7 in rMQR. They are zero before masking and carry no codeword.

Micro QR M1 and M3 break the pattern. Their data capacity is not a multiple of 8 (20 and 84 bits), so the last data codeword is 4 bits long, and the error-correction codewords follow it immediately in the stream (doc 26, section 4). With f whole data codewords before the half one:

- bits 0 to 8f − 1 belong to codewords 0 to f − 1 as usual;
- bits 8f to 8f + 3 are bits 7 to 4 of codeword f, the half codeword;
- bit 8f + 4 + j belongs to codeword f + 1 + ⌊j / 8⌋, bit 7 − (j mod 8).

M1, for example, has two whole data codewords, one half codeword, and two error-correction codewords: 16 + 4 + 16 = 36 stream bits, one per message module. `Inspection.Codewords` stores the half codeword in the high nibble of its byte, with zeros below, which is also how Reed-Solomon sees it.

A test checks the mapping from both sides: every message module, before masking, holds the bit of `Codewords` that `ModuleCodeword` and `ModuleBit` name, and no codeword bit appears twice.

## 5. Before the mask, and the mask scores

`MaskPattern` marks every module the mask formula selects, over the whole grid. The formula is QR's mask k for QR, QR's mask 1, 4, 6, or 7 for Micro QR masks 0 to 3, and QR's mask 4 for rMQR (docs 6, 26, and 28). `Unmasked` is the symbol with the mask taken off the message modules again: a message module of `Unmasked` is its value in `Modules` XOR the pattern, and a function module is unchanged. In the unmasked view, the data's long runs and solid blocks become visible, which is exactly what masking exists to break up.

`MaskScores` scores every candidate mask on the unmasked symbol, as the encoder does when it chooses:

- **QR.** Each mask is applied, the format information for that level and mask is written, and the four penalty rules are summed (doc 6): N1 for runs of five or more, N2 for 2 × 2 blocks, N3 for finder-like sequences, N4 for an unbalanced dark ratio. `MaskPenalties` keeps the four parts. The lowest total wins, the lower mask number on a tie.
- **Micro QR.** Each mask is applied and the dark modules are counted along the right and bottom edges, skipping the timing modules; with SUM1 the smaller count and SUM2 the larger, the score is 16 × SUM1 + SUM2 (doc 26, section 9). The highest score wins.
- **rMQR.** There is one fixed mask, so there is nothing to score, and `MaskScores` is nil.

The scores are computed even when `WithMask` forces a mask: the encoder skips the comparison then, but the scores show what it would have chosen and why.

## 6. The message's bit budget

`Segments` lists the parts of the message in order, each with the widths of its three fields: the mode indicator, the character count, and the payload. The widths depend on the symbology: QR's indicator is 4 bits and its count width depends on the version group (doc 2); Micro QR's indicator is 0 to 3 bits and its count widths are its own (doc 26); rMQR's indicator is 3 bits and its count widths depend on the version (doc 28). An ECI header appears as a segment of mode ECI, its count field empty and its payload the designator: 8 bits for ECI 3 (ISO-8859-1) and ECI 26 (UTF-8). Under an ECI, a byte segment is counted in transcoded bytes, so `é` is one byte under ISO-8859-1 and two under UTF-8. Kanji segments count characters, 13 bits each.

After the segments, `DataBits` long in total, comes the terminator: 4 zero bits in QR, 3, 5, 7, or 9 in Micro QR M1 to M4, and 3 in rMQR, shortened when the capacity runs out first. `TerminatorBits` is its actual length. The rest of `CapacityBits` is padding: zero bits up to the next codeword boundary, then the pad codewords 0xEC and 0x11 in turn.

`HELLO WORLD` at V1-M (doc 10) uses the budget like this: an alphanumeric segment of 4 + 9 + 61 = 74 bits, a 4-bit terminator, 2 zero bits to reach 80 bits, and six pad codewords to fill the 16 data codewords, giving `20 5B 0B 78 D1 72 DC 4D 43 40 EC 11 EC 11 EC 11`. The R11x27-M symbol for 14 digits (doc 28, section 11) shows the other edge: its segment takes 54 of the 56 bits, so the terminator shrinks to 2 bits and there is no padding at all.

## 7. Where each codeword lives

`Blocks` lists each Reed-Solomon block's data codewords and the error-correction codewords computed over them; `Codewords` is the stream in placement order. The two are related by interleaving (doc 4): the first data codeword of every block, then the second of every block, and so on, skipping a shorter block once it runs out, then the error-correction codewords the same way.

The demo replays that order to label each codeword of the stream with its block, its position in the block, and whether it is data or error correction. V5-Q has two blocks of 15 data codewords and two of 16, with 18 error-correction codewords each. Its first 60 stream codewords take positions 0 to 14 from the four blocks in turn; codewords 60 and 61 are position 15 of the two longer blocks; and the 72 error-correction codewords follow, again block by block. Hovering a codeword in the demo highlights its eight modules, which shows how interleaving scatters each block across the symbol, so that local damage costs every block a little rather than one block a lot.

## 8. Running the library in a browser

The demo runs the library itself. Go's `js/wasm` target compiles it, with a small bridge, into one WebAssembly module of about 4.7 MB (1.3 MB compressed). The module exposes a single JavaScript function, `qrgenCall(method, request, ...blobs)`, which takes a JSON request and binary inputs such as an image file, and returns a JSON response. The operations behind it (encode, inspect, masks, payload, structuredAppend, decode, decodePixels, and decodeStructuredAppend) are ordinary Go in `web/internal/bridge`, so they are tested with the normal toolchain, and the same tests also run compiled to WebAssembly in Node.

Go's WebAssembly runtime runs on one thread, and some calls take visible time: a version 40 symbol, or a camera frame. The module therefore lives in a Web Worker, which handles calls one at a time while the page stays responsive. The `wasm_exec.js` glue that starts the runtime must come from the same Go release as the compiler, so the build script copies it from the toolchain that builds the module.

Images go through the library's own path: a file's bytes are decoded by Go's image packages and read by `DecodeAll`, with `Decode` as a fallback for blurry symbols the multi-symbol detector misses. Camera frames arrive as RGBA pixels from a canvas. The page sends the centre square of each frame, at most 480 pixels wide, and the bridge converts it to grey with the weights of `color.GrayModel`, Y = (19595 R + 38470 G + 7471 B + 2¹⁵) >> 24 on 16-bit channels, so the decoder sees the same grey levels it would from the RGBA image.

The cost per frame depends on what the frame shows. A 640 × 480 frame with a symbol decodes in about 6 ms natively and 20 ms in WebAssembly; a realistic frame without one takes about 30 ms natively. A frame of fine random noise is the worst case, about 0.7 s natively and 2.6 s in WebAssembly: every speck becomes a connected component, and the finder search compares nested components pairwise (doc 24). The smaller square bounds that cost, and the worker keeps the page responsive while it is paid.

## 9. How it was checked

`Inspect` reports on the encoder, so the risk is a report that agrees with itself and not with the symbol. The checks therefore lean on facts from outside `Inspect`:

- `Modules` equals `Matrix`'s output for every symbology and the options that change the encoding: versions, levels, masks, ECI, Kanji, and a logo.
- The layers agree with the codewords: every unmasked message module holds its claimed bit, every codeword bit appears once, remainder modules are light before masking, and function modules are never masked.
- The role census of section 2 was worked out by hand from the layouts in docs 5, 26, and 28. It caught one labelling error while this document was written: R7's bottom-left corner modules were first tagged as corner pattern over the finder; the symbol was right, the labels were not.
- The codewords of doc 10 and two streams of doc 28, which come from zint, match exactly.
- The automatic mask is the lowest QR penalty or the highest Micro score, and forcing a mask leaves the scores unchanged.

## 10. Implementation pointers

- `qrgen/inspect.go`: `ModuleRole`, `Inspection`, `InspectSegment`, `InspectBlock`, `Inspect`, the trace, the stream-bit mapping (`byteLocator`, `microLocator`), and the mask scores.
- `qrgen/matrix.go`, `qrgen/micro_matrix.go`, `qrgen/rmqr_matrix.go`: role tags in the placement code (`tag`, `reserveAs`), and stream bits in `placeBits`.
- `web/internal/bridge/bridge.go`: the JSON operations, including the codeword locations and the grey conversion of camera frames.
- `web/wasm/main.go`: the WebAssembly entry point.
- `web/site/`: the page, with `inspect.js` drawing the layers described here.

## References

- ISO/IEC 18004:2015 and ISO/IEC 23941:2022 — the layouts, masks, and mask evaluation the roles and scores follow.
- `docs/theory/04-reed-solomon.md`, `05-matrix-construction.md`, `06-masking.md`, `07-format-version-info.md`, `10-worked-example.md`, `24-arbitrary-angle-detection.md`, `26-micro-qr.md`, `28-rmqr.md` — the steps this document takes apart again.
- Go, `syscall/js` and the `js/wasm` port — the WebAssembly target and its `wasm_exec.js` glue.
