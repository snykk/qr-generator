# rMQR Detection

Doc 28 described the rMQR symbol and how v0.17 decodes a clean rMQR grid. This document is about finding that grid in an image. rMQR shares Micro QR's problem, a single finder pattern where QR has three, and adds one of its own: the symbol can be 20 times longer than the finder is wide. The sections below show what the finder settles, what it cannot, how the finder sub-pattern at the far corner anchors the other end, and how the search joins the pipeline after QR and Micro QR.

> Indonesian version: [30-rmqr-detection.id.md](30-rmqr-detection.id.md).

## 1. What one finder leaves open

The connected-component detector (doc 24) finds an rMQR finder as it finds any finder: a dark ring around a light ring around a dark centre, in the 7:5:3 proportion. Its separator keeps the ring apart from the rest of the symbol, as in Micro QR, so the four outer corners of the ring are clean, and doc 27's corrections apply unchanged: each corner is moved half a pixel outward to the ring's true boundary, and the corners are wound in a fixed order so that matching them to the module corners (−0.5, −0.5), (6.5, −0.5), (6.5, 6.5), and (−0.5, 6.5) leaves only four choices, the quarter turns.

The finder fixes a homography, but only over the 7 modules it spans. For Micro QR that is enough to reach the far corner of a 17-module symbol with a refinement (doc 27, section 6). An rMQR symbol is up to 139 modules wide, so the far end lies up to 20 finder widths away, and every error in the finder's corners grows with that distance: a corner misplaced by a tenth of a module tilts the predicted top edge enough to miss the far end by a module or more. The finder alone settles the orientation and where to read the format information, but not where the symbol ends.

## 2. Orientation from the top timing pattern

In the right quarter turn, row 0 to the right of the finder reads a fixed sequence: column 7 is the separator, light, and from column 8 the top timing pattern runs dark at even columns and light at odd ones. The search reads row 0 at columns 7 to 11 through each of the four assignments and keeps those that read light, dark, light, dark, light. Random modules pass this test one time in 32. Alignment patterns, which interrupt the top timing further along, are centred on column 19 or later and so begin at column 18 at the earliest; they never touch these five modules.

zxing-cpp tests columns 8 to 11 the same way. Micro QR's test (doc 27, section 2) reads the innermost modules of both timing patterns, row 0 and column 0, but an R7 symbol has no column timing below its finder, since the finder fills its whole height, so the rMQR test uses row 0 alone.

Micro QR symbols pass this test too: their row 0 is a timing pattern with the same parity, and in an M1 symbol column 11 is quiet zone, which is light. The format information and the sub-pattern, not the orientation test, tell the two apart.

## 3. The format information, before the size is known

The format copy beside the finder sits at rows 1 to 5 of columns 8 to 10 and rows 1 to 3 of column 11 (doc 28, section 7), all within reach of the finder homography. Its 18 bits, unmasked with 0x1FAB2, are compared with the 64 valid words, one per version and level. BCH(18,6) has minimum distance 8, so a word within 3 bits of a valid one is decoded uniquely, and its version names both dimensions of the symbol.

That match is weak evidence on its own. Each valid word has 988 words within 3 bits of it, so the 64 of them cover 24 percent of all 2¹⁸ possible words: one random word in four decodes to some version. Together with the orientation test, about one random finder assignment in 130 gets this far. The evidence that counts comes next, from the far end of the symbol.

## 4. The finder sub-pattern, and which part of it to find

The finder sub-pattern is a 5 by 5 pattern in the bottom-right corner: a dark ring, a light ring, and a single dark centre at row h − 3, column w − 3. Its shape is exactly that of a QR alignment pattern.

Its dark ring is the wrong part to look for. Unlike the finder, the sub-pattern has no separator: the format copy beside it occupies the column to its left and the row above it, and in R7 symbols the top-right corner pattern touches its top edge. Wherever one of those modules is dark, the dark ring joins it into one larger component of no fixed shape. The light ring is different. The solid dark ring encloses it completely, so it is always a component of its own, a hollow square 3 modules across, with the one-module dark centre as a component inside it. That nesting, a light component about three times the side of a concentric dark component, is the sub-pattern's signature. `labelComponents` already labels the light components of the whole image for the finder search, so the candidates cost one pass over its output. A QR finder does not qualify, since its light ring is 5 modules across around a 3-module centre.

The signature is common. Every QR alignment pattern has it, and so does any light ring the data happens to draw around a single dark module. A candidate is accepted only where the symbol needs it:

- its centre lies near the point the finder homography predicts for (w − 3, h − 3), within a tolerance that grows with the distance from the finder, since that is how the prediction's error grows;
- its module size, a third of the light ring's side, is close to the finder's.

The nearest qualifying candidate wins. zxing-cpp instead searches around the predicted point with its alignment-pattern locator; a candidate list built once per image makes the tolerance a parameter rather than a search radius, and reuses the labelling the finder search has already paid for.

The light ring also gives four more corners, at (w − 4.5, h − 4.5), (w − 1.5, h − 4.5), (w − 1.5, h − 1.5), and (w − 4.5, h − 1.5) in module coordinates. They are extracted as the finder's corners are, corrected half a pixel outward, and matched to their module corners by the finder homography's prediction of each.

## 5. Geometry: eight points, then the long edges

With four corners at each end of the symbol, a least-squares homography no longer extrapolates: the far end is measured, not predicted. Over a symbol 20 finder widths long, though, eight points close to two corners still leave the long edges to interpolation, and a slight error in either quad bends the predicted rows in the middle.

The long edges are the best measurement the symbol offers. Row 0 is dark across the finder, at every even column of the top timing pattern, across the top row of each alignment pattern, and at the top-right corner pattern; row h − 1 is dark at the bottom-left corner pattern, at every even column of the bottom timing pattern, across the bottom row of each alignment pattern, and across the bottom of the sub-pattern. The search marches outward from each of those modules to the symbol's edge, as doc 27's edge fit does, and fits a line to each long edge through dozens of points. The short edges take the finder's outer column, the corner modules, and the sub-pattern's outer column. The four lines meet in the symbol's four corners, which with the eight corners of the two patterns give the final homography.

As in v0.16, each refinement is offered as a variant rather than trusted: the edge fit first, then the eight-point homography it starts from, and Reed-Solomon decides among them. A finder with no sub-pattern where its version needs one is not decoded at all.

## 6. The second format copy

The format copy beside the sub-pattern, at rows h − 6 to h − 2 of columns w − 8 to w − 6 and row h − 6 of columns w − 5 to w − 3, unmasked with 0x20A7B, can only be read once the far end is located. It serves twice.

- **Confirmation.** A random word lies within 3 bits of a given version's two words with probability 2 × 988 / 2¹⁸, under 1 percent. When the finder-side copy named the version, the sub-side copy agreeing with it is strong evidence that the symbol is real, and its level must match too.
- **Fallback.** When the finder-side copy is unreadable, the search tries each of the 32 versions whose sub-pattern position holds a candidate, and reads the sub-side copy through the homography that candidate gives. A version whose own copy agrees is accepted. So either copy alone is enough to locate the symbol, as either is enough for `decodeRMQRMatrix` (doc 28, section 7).

## 7. Evidence, stages, and errors

Each finder records how far it got, so that when nothing decodes the most convincing failure is reported:

- **none**: no quarter turn passed the orientation test;
- **oriented**: the orientation test passed, but no format copy named a version;
- **format**: a version was named, but no sub-pattern sits where it puts one;
- **located**: a finder, a sub-pattern where the version needs it, and a format copy agreeing; the outcome is the matrix decoder's.

Only a located symbol is convincing. A QR or Micro finder can reach the format stage by chance (section 3), but a sub-pattern at the one place a particular version puts it, with the right module size, is not chance. So, as for Micro QR (doc 27, section 8), `Decode` reports an rMQR error in place of "finder patterns not found" only for a located symbol, and a located rMQR symbol outranks a Micro symbol that merely passed Micro's timing check, since an rMQR finder of height 9 can pass that check.

## 8. Where the search joins the pipeline

The rMQR search comes last, on the same finder quads. On each binarisation, the projective QR attempt takes the best finder triple, the Micro search tries each finder as a Micro finder, and only then does the rMQR search try each finder as an rMQR finder. In `DecodeAll`, QR triples claim their finders first, Micro symbols claim theirs next, and the finders still unclaimed are offered to the rMQR search. A symbol the earlier searches decode is therefore never re-read as rMQR, and QR and Micro QR results cannot change.

The order also keeps the symbologies apart. A Micro finder passes the rMQR orientation test and may name an rMQR version by chance, but there is no sub-pattern where that version would put one, and it has already decoded as Micro QR. An R9 rMQR finder can pass Micro's timing check, but Micro's Reed-Solomon rejects what it reads, and the rMQR search reads it next.

A decoded rMQR symbol reports its outline through the final homography: the module corners (−0.5, −0.5), (w − 0.5, −0.5), (w − 0.5, h − 0.5), and (−0.5, h − 0.5). Its finder centre is the one finder `Result` reports.

## 9. zxing-cpp as prior art

zxing-cpp 2.3.0 reads rMQR symbols in images with `SampleRMQR`, tried on every finder no QR symbol used. It runs the orientation test of section 2 on columns 8 to 11, reads the finder-side format copy in each surviving quarter turn, and keeps the turn whose word is nearest a valid one. It looks for the sub-pattern with its alignment-pattern locator around the point the finder homography predicts, takes the corners of both patterns, and builds a four-point transform, from the two patterns' outer corners for symbols up to 9 modules tall and from their centres and two intersected corners for taller ones. It samples the grid from that transform. It does not read the sub-side format copy, does no refinement along the symbol, and marks the code "TODO: this is a WIP". It is the reader this implementation is compared against.

## 10. How it is checked

The detector is measured against material that does not come from our own encoder: the 70 zint symbols of v0.17, rendered into images at several scales, at every quarter turn, under in-plane rotation and perspective tilt, and with quiet zones from none to four modules; zxing-cpp's reader on the same scenes; composites of QR, Micro QR, and rMQR symbols, each of which must be reported once as its own kind; and every finder of QR and Micro QR symbols, none of which may decode as rMQR. Each geometric step is kept only if it reads at least as many scenes as the one before it. The measured band is recorded here once it is measured.

## 11. Implementation pointers

Planned in RD3 to RD5 (see `docs/plan-rmqr-detection.md`):

- `qrgen/decode_rmqr.go`: the orientation test, both format reads, sub-pattern candidates and pairing, the eight-point homography, the edge fit, the stages, and the decoder for one finder.
- `qrgen/decode_finder_cc.go`: sub-pattern candidates from the light components of the labelling.
- `qrgen/decode_image.go`: rectangular sampling, and the rMQR pass after the Micro pass with the error contract.
- `qrgen/decode_multi.go`: the rMQR pass in `DecodeAll`, and `Result.RMQRVersion`.

## References

- ISO/IEC 23941:2022 — the rMQR layout, through zint's and zxing-cpp's source (doc 28).
- zxing-cpp 2.3.0, core/src/qrcode/QRDetector.cpp (`SampleRMQR`) and QRReader.cpp — prior art, and the independent reader used for validation.
- `docs/theory/24-arbitrary-angle-detection.md` — the connected-component detector, corner extraction, and least-squares homography.
- `docs/theory/25-multi-symbol-decoding.md` — the claim loop the rMQR pass joins.
- `docs/theory/27-micro-qr-detection.md` — the single-finder search, the half-pixel correction, and the edge fit this detector extends.
- `docs/theory/28-rmqr.md` — the rMQR symbol: layout, format information, and the matrix decoder this detector feeds.
