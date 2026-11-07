# Micro QR Detection

Doc 26 described the Micro QR symbol and how v0.15 decodes a clean Micro grid. This document is about the step before that: finding the grid in an image. The QR pipeline locates a symbol through three finder patterns, and a Micro symbol has one. The sections below show what one finder determines on its own, what it leaves open, how the timing patterns and the format information close the gap, and how the search fits into the existing pipeline without disturbing QR decoding.

> Indonesian version: [27-micro-qr-detection.id.md](27-micro-qr-detection.id.md).

## 1. Why the QR pipeline cannot start

Both QR detectors are built on a triangle. The affine path (docs 12 and 15) scans for 1:1:3:1:1 runs, keeps the three strongest clusters, and orders them: the right-angle vertex is the top-left finder, a cross product tells top-right from bottom-left, the spacing gives the version, and the three centres plus a completed fourth corner give the homography. The projective path (doc 24) finds the finders as nested connected components, extracts their corners, and fits a homography to the twelve corners of the triple. Both reject an image with fewer than three finders, and one finder forms no triangle to read anything from.

Micro QR keeps a single finder because a small symbol cannot spare the area for three (doc 26, section 1). What makes one finder enough is everything else the standard places next to it.

## 2. What four corners determine, and what they leave open

The connected-component detector already returns each finder as a quad: its centre and the four outer corners of its dark ring (doc 24, sections 3 and 4). In module coordinates, with module centres at integers, those corners sit at `(-0.5, -0.5)`, `(6.5, -0.5)`, `(6.5, 6.5)`, and `(-0.5, 6.5)`. A projective homography has eight degrees of freedom, and four correspondences in general position fix it exactly, so a single finder pins down the whole mapping between the symbol's plane and the image, perspective included.

What the corners do not say is which is which. The finder is three concentric squares: it looks the same after a quarter turn and in a mirror, and its corners come out of the detector unlabelled. The homography is therefore known only up to those symmetries. The winding removes the mirror half. For a symbol seen from the front, the corners in the order top-left, top-right, bottom-right, bottom-left enclose a positive signed area in image coordinates (y growing downward), as the module corners do, because a homography that does not reflect preserves orientation. Putting the four extracted corners in that winding leaves four candidate assignments, the four cyclic shifts, one per quarter turn. A mirror image, which a view through the back of a transparent sheet would produce, has the opposite winding and is not read, as for QR (doc 15).

## 3. Orientation from the timing patterns

Around the finder of a Micro symbol, the two sides away from the data, top and left, border the quiet zone. The two sides facing the data are followed by the separator, and the timing patterns start right after it, on row 0 and column 0, dark at even indices (doc 26, section 6). The modules (row 0, column 8) and (row 8, column 0) are therefore dark in every Micro version.

Reading those two modules through each candidate homography resolves the quarter turn. Under the correct assignment both are timing modules. Under each wrong one, at least one of them lands two modules out into the quiet zone, which the standard requires to be at least two modules wide:

| Finder corner taken as the symbol's outer corner | (row 0, column 8) lands on | (row 8, column 0) lands on |
|---|---|---|
| Top-left (correct) | timing, dark | timing, dark |
| Top-right | format module (row 8, column 6) | quiet zone (row 0, column -2) |
| Bottom-right | quiet zone (row 6, column -2) | quiet zone (row -2, column 6) |
| Bottom-left | quiet zone (row -2, column 0) | format module (row 6, column 8) |

On a clean image exactly one assignment passes. The separator modules (row 0, column 7) and (row 7, column 0) would add nothing to this test: under the top-right assignment, for instance, they land on the separator row and in the quiet zone, both light, exactly as under the correct one. zxing-cpp's detector uses the same two-module test (section 9).

## 4. Reading the format information before the size is known

The fifteen format modules, column 8 rows 1 to 8 and row 8 columns 1 to 7, all lie in the 9 by 9 block at the finder's corner, in the same place in every version (doc 26, section 7). They can therefore be read through the finder homography while the symbol's size is still unknown, and decoded against all 32 Micro format words, rather than the four to twelve words of a single version that `readMicroFormatInfo` considers once the grid size is known. The nearest word within the BCH(15,5) budget of three errors gives the symbol number, and with it the version, the error-correction level, and the mask. Where more than one assignment passes the timing test, as can happen in a cluttered image, the one whose format word is nearest wins.

A valid format word is weaker evidence than it looks. The code has 32 words at minimum distance 7, so the balls of radius 3 around them are disjoint, each holding 1 + 15 + 105 + 455 = 576 words: 18,432 of the 32,768 possible 15-bit words, 56 percent, decode to some Micro symbol within the budget. A random arrangement of modules passes the format check more often than not. The format word identifies the symbol once we know we are looking at one; it cannot be what decides that we are. The all-light strip that doc 26 warned about is one such word: with three one-bits, M4-L with mask 1 (`0x1202`) is within budget of it.

## 5. Cross-checking the size with the timing pattern

The size the format word claims can be checked against the symbol itself. Along row 0, a Micro symbol of size n reads: the light separator at index 7, timing modules from index 8 to n - 1 alternating from dark and ending dark because n - 1 is even, then the quiet zone at n and n + 1. Column 0 reads the same. That is 2(n - 5) modules, from 12 in M1 to 24 in M4, whose expected values follow from n alone.

The cross-check counts the disagreements over those modules and accepts a small number of them. Its main job is to reject finders that are not Micro finders at all. The finder at a corner of a QR symbol has exactly the right shape, and its separator is where a Micro separator would be; but beyond the separator, row 0 of a QR symbol holds format information and data rather than timing, so about half the checked modules disagree. Both quiet-zone modules are needed, not just the first: a symbol's odd timing indices are light, so a claimed size two smaller than the real one would find light at index n and pass, while index n + 1 lands on a dark timing module and catches it.

A claimed size that is one version off produces a single disagreement per edge, which a tolerance may accept, so the cross-check cannot always separate neighbouring sizes on its own. It does not need to. A wrong size requires at least four errors in the BCH-protected format word, and the Reed-Solomon check on a grid sampled at the wrong size fails.

## 6. Extrapolation, and finding the far corners again

For a perfect image the finder homography is exact: the symbol is planar, and four exact correspondences determine the homography of the whole plane. Real corners are not exact. They come from the extreme pixels of a binarised ring, so they carry pixel noise, and a pixel's centre sits half a pixel inside the ring's true outer corner. Those errors grow with distance from the points that fix the transform. A scale error at the finder's corners, 3.5 modules from its centre, grows in proportion to distance: a corner misplaced by a tenth of a module becomes nearly four tenths at the bottom-right module of an M4 symbol, 13 modules from the finder centre on each axis. Under perspective it is worse, because the foreshortening terms of the homography are fixed by a small square and extrapolated across a large one. Sampling reads each module at its predicted centre, so the whole budget is half a module.

The half-pixel bias alone breaks small symbols, and was the first thing the tests found. At 3 pixels per module, corners half a pixel inside the ring shrink the finder by one pixel in 21, and 13 modules out the timing modules of every M4 symbol are read one module too close to the finder. The correction is cheap: move each corner outward from the finder centre to the boundary of its own pixel, which restores the true corner exactly for an upright ring and for one turned 45 degrees, and nearly in between. With it, all 34 zint symbols decode at every quarter turn from 8 pixels per module down to 1.

The rest of the remedy is to locate points far from the finder and refit, and the standard provides them. The last timing module on each edge, at index n - 1, is always dark and bounded by the quiet zone on two sides, so the top-right and bottom-left corners of the symbol can be found as the dark pixels furthest along the outward diagonal near their predicted positions. That anchors two of the three far corners and leaves the bottom-right one extrapolated, and at small module sizes that is where the remaining errors gathered.

The bottom-right corner has no fixed module, but the edges that meet there have plenty. The Micro masking rule chooses the mask that maximises the dark modules on the last row and the last column (doc 26, section 9), and every dark module on a border meets the quiet zone at the symbol's outer edge. Marching outward from a dark border module to the first light pixel therefore gives a point on that edge. The finder's outer row and column and the even timing modules supply points on the top and left edges, the dark modules of the last column and row supply the right and bottom ones, a line is fitted through each edge's points (dropping any point more than half a module from a first fit), and the four lines meet at the four corners of the symbol. With the finder corners, they give a least-squares homography that no longer extrapolates anywhere.

Each step needs the one before it close enough that its searches start in the right modules, so they run in order: the finder homography, the far corners, then two rounds of the edge fit. The cross-check and Reed-Solomon choose among the results, so a refinement that goes wrong costs a decode attempt rather than a symbol. Over all 34 zint symbols rendered at 2 and 3 pixels per module and rotated in 10-degree steps, the finder homography alone reads 192 of 612 scenes; the far corners raise that to 380, the edge fit to 498, and its second round to 502. zxing-cpp re-locates the three far corners with a local search instead.

## 7. False positives and the gates in front of them

A finder is accepted as a Micro symbol only after passing, in order: the timing test of section 3, the format check of section 4, the cross-check of section 5, and a full decode of the sampled grid by `decodeMicroMatrix`, Reed-Solomon included. Each gate catches what the others let through. For a finder whose surroundings are not a Micro symbol, the timing test passes about one time in four and the format check about one in two. The cross-check compares 12 to 24 modules, so with a tolerance of two disagreements a random neighbourhood passes it about one time in fifty for M1 and far less often for the larger versions. Measured on every finder of 224 QR symbols (versions 1 to 7, every EC level and mask), the assignments that got as far as the cross-check disagreed in 1 to 14 modules, 8 at the median; three fell within the tolerance, and Reed-Solomon rejected all three.

Reed-Solomon is the last gate. A uniformly random grid passes it about one time in 25,000 for M3-L, whose six check codewords correct up to three errors, and one time in 65,536 for M1, whose two check codewords only detect errors; every other type passes fewer than one in 140,000, down to about one in 200 billion for M4-Q. The segment parser then rejects many of the survivors. Combining the gates, M1 is the weakest case, because its cross-check is the shortest, and it is the one the negative tests target.

Order matters as much as the gates. The Micro search runs only after QR detection has failed on the same binarised image, so the corner of a readable QR symbol is never offered to it.

## 8. Where the search plugs in

`Decode` runs the affine QR pipeline first, unchanged (doc 24, section 8). When it fails, the connected-component attempt labels the binarised image once and uses its finder quads twice: first the best QR triple, then, if that does not decode, each quad as a Micro finder, largest first, the first to decode winning. It does this on the Otsu binarisation and, when needed, again on the Sauvola one (doc 14). Because the labelling is shared, a clean Micro image is found on the first binarisation without an extra pass.

On total failure `Decode` returns the affine attempt's error, as before, so the errors for QR images do not change. The one exception is an image in which the affine attempt found no finder triple but a finder convincingly belongs to a Micro symbol: its timing pattern matches a Micro size exactly while its format information is unreadable, or it passed every gate and only its data failed. There the Micro attempt's error is returned, because "format information unreadable" or "too many errors" describes a damaged Micro symbol better than "finder patterns not found". Passing the two-module timing test is not enough for this. Measured on every finder of 224 QR symbols, about a third pass it and over half of those name a valid format word, while fewer than one in a hundred look like a Micro symbol in either stronger sense; a QR symbol with one finder wiped out therefore keeps reporting "finder patterns not found".

`DecodeAll` does the same within its claim loop (doc 25): after the QR triples have claimed their finders, every unclaimed quad is tried as a Micro finder and claimed if it decodes. A Micro result reports its version in a field of its own, since `Version` is the QR version type.

## 9. Prior art

zxing-cpp locates Micro symbols the same way in outline (core/src/qrcode/QRDetector.cpp, `SampleMQR`, read on its master branch in November 2026). It fits the four rotations of a finder's corner quad, requires the same two innermost timing modules to be dark, decodes the format bits against all 32 words in both bit orders (the second covering mirrored symbols), keeps the rotation with the lowest Hamming distance, and takes the version from the symbol number. Instead of a timing cross-check it samples row n and column n just outside the symbol and rejects the candidate when more than a third of those samples are dark, its guard against the corners of QR symbols. Before sampling it re-locates the top-right, bottom-left, and bottom-right corners with a local search. Its reader tries every finder pattern not used by a decoded QR symbol, after the QR pass. Its corner quad lies midway through the finder's outer ring, at the centres of the corner modules, rather than on the ring's outer edge.

## 10. Validation and limits

As with arbitrary-angle decoding (doc 24, section 9), there is no table to check a detector against, so it is validated empirically and against independent implementations. The zint symbols stored as text in `qrgen/testdata/thirdparty/micro_symbols.txt` are rendered into images at several module sizes, quiet zones, and rotations, and each must decode to its text; they come from an encoder that shares no code with this library. zxing-cpp's reader is run once on the same synthetic scenes, to confirm that an independent detector reads them and to expose any scene it reads and this library does not. Negative cases cover blank images, lone finder patterns, and the corners of QR symbols. The measured band, over all 34 zint symbols: any in-plane rotation decodes from 3 pixels per module up (every 5 degrees at 3, 4, and 5 pixels per module, with no failure), and most rotations still decode at 2. Perspective tilt decodes up to 60 degrees at 10 pixels per module, 55 at 5, and 45 at 4, under every in-plane rotation tried. Quiet zones from 0 to 4 modules all work, including a symbol cropped flush to the image edge, because samples outside the image read light. Past about 65 degrees of tilt the finder itself stops being found, the same limit the QR detector meets (doc 24, section 9).

On the same 1,836 synthetic scenes, zxing-cpp 2.3.0 with its default reader options returns the same text wherever both read and never a different one, and it reads fewer scenes overall. At 3 and 5 pixels per module both read every rotation. At 2 pixels per module this library reads 92 rotations zxing-cpp does not, and zxing-cpp reads 5 this library does not. Under tilt this library reads 306 scenes zxing-cpp does not, and zxing-cpp reads 14 this library does not, all at 65 degrees of tilt with no in-plane rotation at 5 pixels per module, where our finder detector gives out first. Real photographs add blur, glare, and uneven light that synthetic scenes lack, so the band is an upper bound on what a camera image will give.

## 11. Implementation pointers

Planned in MD3 to MD6 (see `docs/plan-micro-qr-detection.md`):

- `qrgen/decode_micro.go`: the half-pixel corner correction and the corner winding, the homography for each assignment, the timing test, image-side format reading over all 32 words, the timing cross-check, the far-corner refinement and the edge fit (`refineMicroHomography`, `edgeFitMicroHomography`, `microHomographyVariants`), and `decodeMicroQuad`.
- `qrgen/decode_finder_cc.go`: `allFinderQuads`, every finder quad with no minimum count.
- `qrgen/decode_image.go`: the Micro pass after the QR triple on each binarisation, and the error contract.
- `qrgen/decode_multi.go`: the Micro pass over unclaimed quads in `DecodeAll`, and `Result.MicroVersion`.

## References

- ISO/IEC 18004:2015 — the Micro QR layout, timing patterns, format information, quiet zone, and masking rule.
- zxing-cpp, core/src/qrcode/QRDetector.cpp (`SampleMQR`), QRReader.cpp, and QRFormatInformation.cpp (`DecodeMQR`) — prior art for single-finder detection, and the independent reader used for validation.
- `docs/theory/24-arbitrary-angle-detection.md` — the connected-component detector, corner extraction, and least-squares homography reused here.
- `docs/theory/25-multi-symbol-decoding.md` — the claim loop the Micro pass joins.
- `docs/theory/26-micro-qr.md` — the Micro symbol itself: layout, format information, masking, and the matrix decoder this detector feeds.
