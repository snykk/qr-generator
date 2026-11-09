# rMQR Detection

Doc 28 described the rMQR symbol and how v0.17 decodes a clean rMQR grid. This document is about finding that grid in an image. rMQR shares Micro QR's problem, a single finder pattern where QR has three, and adds one of its own: the symbol can be 20 times longer than the finder is wide. The sections below show what the finder settles, what it cannot, how the finder sub-pattern at the far corner anchors the other end, and how the search joins the pipeline after QR and Micro QR.

> Indonesian version: [30-rmqr-detection.id.md](30-rmqr-detection.id.md).

## 1. What one finder leaves open

The connected-component detector (doc 24) finds an rMQR finder as it finds any finder: a dark ring around a light ring around a dark centre, in the 7:5:3 proportion. Its separator keeps the ring apart from the rest of the symbol, as in Micro QR, so the four outer corners of the ring are clean, and doc 27's corrections apply unchanged: each corner is moved half a pixel outward to the ring's true boundary, and the corners are wound in a fixed order so that matching them to the module corners (−0.5, −0.5), (6.5, −0.5), (6.5, 6.5), and (−0.5, 6.5) leaves only four choices, the quarter turns.

The finder fixes a homography, but only over the 7 modules it spans. For Micro QR that is enough to reach the far corner of a 17-module symbol with a refinement (doc 27, section 6). An rMQR symbol is up to 139 modules wide, so the far end lies up to 20 finder widths away, and every error in the finder's corners grows with that distance: a corner misplaced by a tenth of a module tilts the predicted top edge enough to miss the far end by a module or more. The finder alone settles the orientation and where to read the format information, but not where the symbol ends.

## 2. Orientation from the top timing pattern

In the right quarter turn, row 0 to the right of the finder reads a fixed sequence: column 7 is the separator, light, and from column 8 the top timing pattern runs dark at even columns and light at odd ones. The search reads row 0 at columns 7 to 11 through each of the four assignments and keeps those that read light, dark, light, dark, light with at most one module wrong. Alignment patterns, which interrupt the top timing further along, are centred on column 19 or later and so begin at column 18 at the earliest; they never touch these five modules.

The one wrong module is measured, not assumed. Row 0 borders the quiet zone, so a small error in the finder's corners pushes a sample of it off the symbol. Under 45 to 55 degrees of tilt at 4 and 5 pixels per module, the right quarter turn of a finder that was found missed one of the five modules about twice as often as it missed two, and requiring all five lost those symbols at this first gate. With one allowed, random modules pass six times in 32 rather than once, which the gates after it more than absorb.

The five modules lie up to 11 modules from the finder, almost twice its width, and even that small extrapolation matters: through the homography fixed by the finder's four corners, the samples of a small or foreshortened finder can already drift off their modules. The test is therefore read through both that homography and the affine map of section 4, and either may pass it.

zxing-cpp tests columns 8 to 11 the same way, but exactly. Micro QR's test (doc 27, section 2) reads the innermost modules of both timing patterns, row 0 and column 0, but an R7 symbol has no column timing below its finder, since the finder fills its whole height, so the rMQR test uses row 0 alone. Micro QR symbols pass this test too: their row 0 is a timing pattern with the same parity, and in an M1 symbol column 11 is quiet zone, which is light. What follows tells the two apart.

## 3. The format information, before the size is known

The format copy beside the finder sits at rows 1 to 5 of columns 8 to 10 and rows 1 to 3 of column 11 (doc 28, section 7), all within reach of the finder. Its 18 bits, unmasked with 0x1FAB2, are compared with the 64 valid words, one per version and level, and the nearer of the readings through the two maps of section 2 is kept. BCH(18,6) has minimum distance 8, so a word within 3 bits of a valid one is decoded uniquely, and its version names both dimensions of the symbol.

That match is weak evidence on its own. Each valid word has 988 words within 3 bits of it, so the 64 of them cover 24 percent of all 2¹⁸ possible words: one random word in four decodes to some version. Together with the orientation test, about one random finder assignment in 22 gets this far. The evidence that counts comes next, from the far end of the symbol.

## 4. The finder sub-pattern, and where to look for it

The finder sub-pattern is a 5 by 5 pattern in the bottom-right corner: a dark ring, a light ring, and a single dark centre at row h − 3, column w − 3. Its shape is exactly that of a QR alignment pattern.

**Which part to find.** Its dark ring is the wrong part to look for. Unlike the finder, the sub-pattern has no separator: the format copy beside it occupies the column to its left and the row above it, and in R7 symbols the top-right corner pattern touches its top edge. Wherever one of those modules is dark, the dark ring joins it into one larger component of no fixed shape. The light ring is different. The solid dark ring encloses it completely, so it is always a component of its own, a hollow square 3 modules across, with the one-module dark centre as a component inside it. That nesting, a light component about three times the side of a concentric dark component, is the sub-pattern's signature. A QR finder does not qualify, since its light ring is 5 modules across around a 3-module centre.

`labelComponents` already labels the light components of the whole image for the finder search, so the candidates cost one pass over its output, and a linear one: each small dark component is a possible centre, and the light component enclosing it is the one holding the pixel just above it, so no pair of components is ever compared. A light component that reaches the edge of the image is not a ring and is rejected; the quiet zone around a lone pattern would otherwise pass as one.

**Where to look.** The signature is common. Every QR alignment pattern has it, and so does any light ring the data happens to draw around a single dark module, so a candidate is accepted only near where the version puts the sub-pattern, and the prediction of that point turned out to be the hard part.

- **Not the finder homography.** Fixed by four corners 7 modules apart, its perspective terms swing widely with a fraction of a pixel of corner error, and extrapolated to the far end of a long symbol its prediction was measured up to 27 percent of the distance off under in-plane rotation alone. An affine fit of the same four corners, which has no perspective terms, kept that error under 7 percent.
- **Corrected for perspective.** Perspective changes the module size along the symbol and the affine map keeps it constant, so under tilt the sub-pattern lay up to 79 percent of the distance from the affine prediction, always along the symbol's long axis. Along a line, a projective map sends a point D modules away to D times the geometric mean of the module sizes at the two ends; with r the candidate's module size over the finder's, the sub-pattern therefore lies √r times further out than the affine map predicts, and each candidate is judged against its own corrected position. That brought the error along the axis under 40 percent, but no lower, because binarisation thickens dark modules and so measures the light ring about a fifth small.
- **A window longer than it is wide.** The axis itself stays a straight line under perspective, so across it the error comes only from the finder's angle: at most 5 percent of the distance under rotation and 15 under tilt. The window is 2 modules plus 60 percent of the distance along the axis and 18 percent across it, and a candidate's module size may be up to 2.6 times the finder's either way, since tilt made the far modules up to 2.8 times larger along the axis.

Inside such a window several candidates can qualify, so the twelve nearest are tried in turn, and the evidence of section 7 decides among them.

The light ring also gives four more corners, at (w − 4.5, h − 4.5), (w − 1.5, h − 4.5), (w − 1.5, h − 1.5), and (w − 4.5, h − 1.5) in module coordinates. They are extracted as the finder's corners are, corrected half a pixel outward, and matched to their module corners by the cyclic shift nearest the finder homography's prediction of them.

## 5. Geometry: eight points, then the long edges

With four corners at each end of the symbol, a least-squares homography no longer extrapolates: the far end is measured, not predicted. Over a symbol 20 finder widths long, though, eight points close to two corners still leave the long edges to interpolation, and a slight error in either quad bends the predicted rows in the middle.

The long edges are the best measurement the symbol offers. Every border module that the layout fixes as dark, taken from the layout itself so that finder, timing, alignment, corner, and sub-pattern modules are all included, is a starting point: the search marches outward from each to the symbol's edge, as doc 27's edge fit does, and fits a line to each edge through dozens of points on the long ones. The four lines meet in the symbol's four corners, which with the eight corners of the two patterns give the final homography; a second round starts from the first.

The refinement earns its place. Over the widest symbol of each height at 2 and 3 pixels per module, rotated in 10 degree steps, the eight-point homography alone read 31 of 108 scenes and the edge fit 73. As in v0.16, each refinement is offered as a variant rather than trusted, the edge fits first and the eight-point homography last, and Reed-Solomon decides among them.

## 6. The second format copy

The format copy beside the sub-pattern, at rows h − 6 to h − 2 of columns w − 8 to w − 6 and row h − 6 of columns w − 5 to w − 3, unmasked with 0x20A7B, can only be read once a sub-pattern candidate gives the far end. When the finder-side copy is unreadable, the search tries each of the 32 versions, pairs the finder with each candidate near where that version puts the sub-pattern, and reads the sub-side copy through the resulting homography; a pairing whose copy names that version goes on to the check of section 7. So either copy alone is enough to locate the symbol, as either is enough for `decodeRMQRMatrix` (doc 28, section 7).

The copy is not, on its own, enough to confirm a symbol. A random word lies within 3 bits of a given version's two words with probability 2 × 988 / 2¹⁸, under 1 percent; but the fallback tries up to twelve candidates for each of 32 versions, hundreds of readings, and in testing it duly matched by chance on a symbol whose copies had both been destroyed. Confirmation needs more modules than 18.

## 7. Evidence, stages, and errors

The confirmation is the timing pattern along both long edges. Every function module of row 0 and row h − 1, dark or light, is fixed by the layout, at least 54 of them in the narrowest version. A located symbol is a pairing through one of whose homographies at most a tenth of them read wrong: a real symbol read through a good homography gets almost all of them right, while modules read at random get half of them wrong, and the chance of passing by accident is negligible.

Each finder records how far it got, so that when nothing decodes the most convincing failure is reported:

- **none**: no quarter turn passed the orientation test;
- **oriented**: the orientation test passed, but no format copy named a version;
- **format**: a format copy named a version, but no pairing at that version read its timing rows;
- **located**: a finder and a sub-pattern whose timing rows read correctly; the outcome is the matrix decoder's.

A QR or Micro finder reaches the format stage by chance often enough, but not a located symbol: across every finder of 224 QR symbols (versions 1 to 7, every level and mask) and of the zint Micro symbols, none was located. So, as for Micro QR (doc 27, section 8), `Decode` reports an rMQR error in place of "finder patterns not found" only for a located symbol, and a located rMQR symbol outranks a Micro symbol that merely passed Micro's timing check, since an rMQR finder of height 9 can pass that check. A symbol whose two format copies are both unreadable names no version and is reported as not found.

## 8. Where the search joins the pipeline

The rMQR search comes last, on the same finder quads. On each binarisation, the projective QR attempt takes the best finder triple, the Micro search tries each finder as a Micro finder, and only then does the rMQR search try each finder as an rMQR finder. In `DecodeAll`, QR triples claim their finders first, Micro symbols claim theirs next, and the finders still unclaimed are offered to the rMQR search. A symbol the earlier searches decode is therefore never re-read as rMQR, and QR and Micro QR results cannot change.

The order also keeps the symbologies apart. A Micro finder passes the rMQR orientation test and may name an rMQR version by chance, but no pairing reads rMQR timing rows around it, and it has already decoded as Micro QR. An R9 rMQR finder can pass Micro's timing check, but Micro's Reed-Solomon rejects what it reads, and no zint rMQR symbol, at any quarter turn, read as Micro QR.

A decoded rMQR symbol reports its outline through its final homography, the module corners (−0.5, −0.5), (w − 0.5, −0.5), (w − 0.5, h − 0.5), and (−0.5, h − 0.5), and its finder centre is the one finder `Result` reports.

The search costs little where it finds nothing. On realistic frames without a symbol it made no measurable difference to `Decode`, and on the worst case, a frame of fine random noise in which every speck is a candidate, it is about a tenth of the time, the rest being the finder search itself (doc 24).

## 9. zxing-cpp as prior art

zxing-cpp 2.3.0 reads rMQR symbols in images with `SampleRMQR`, tried on every finder no QR symbol used. It runs the orientation test of section 2 on columns 8 to 11, reads the finder-side format copy in each surviving quarter turn, and keeps the turn whose word is nearest a valid one. It looks for the sub-pattern with its alignment-pattern locator around the point the finder homography predicts, takes the corners of both patterns, and builds a four-point transform, from the two patterns' outer corners for symbols up to 9 modules tall and from their centres and two intersected corners for taller ones. It samples the grid from that transform. It does not read the sub-side format copy, does no refinement along the symbol, and marks the code "TODO: this is a WIP".

On the same 1680 scenes built from the zint symbols (rotations every 15 degrees at 3 and 4 pixels per module, and tilts of 30, 45, and 55 degrees at 10 and 5 under two in-plane rotations), zxing-cpp read 409 and this detector 1674. zxing-cpp read every upright scene, but few of the turned or tilted ones, which is what a four-point transform without refinement along a long symbol predicts; it read no scene this detector missed, and never returned a different text.

## 10. How it was checked

The detector is measured against material that does not come from our own encoder: the 70 zint symbols of v0.17 rendered into images, zxing-cpp's reader on the same scenes, composites of QR, Micro QR, and rMQR symbols, and the finders of QR and Micro QR symbols, which must never be read as rMQR.

- **Quarter turns.** Every symbol, painted at 3 and 5 pixels per module, reads at every quarter turn.
- **Rotation.** In 5 degree steps over all 70 symbols, 1255 of 1260 scenes read at 4 pixels per module, 1228 at 3, and 984 at 2; the misses at 4 are the longest symbols near 45 degrees.
- **Tilt.** Under four in-plane rotations, every scene reads to 55 degrees of tilt at 10 pixels per module (278 of 280 at 60), every scene at 30 degrees at 5, and 262 of 280 at 45.
- **Quiet zones.** Every scene reads with a quiet zone of none to four modules, since every gate reads the symbol's own modules.
- **Damage.** With the finder-side format copy destroyed, every symbol reads through the other; with its data destroyed, it is still located and reports its Reed-Solomon error.
- **Confusion.** No QR or Micro finder was located as rMQR, no rMQR symbol read as Micro QR, and `DecodeAll` reported a scene mixing all three kinds once each, as its own kind.

## 11. Implementation pointers

Built in RD3 to RD6 (see `docs/plan-rmqr-detection.md`):

- `qrgen/decode_rmqr.go`: the orientation test (`rmqrOriented`), both format reads (`nearestRMQRFormat`, `rmqrSubFormat`), the affine prediction and the window (`finderAffine`, `subPatternsNear`), the eight-point homography (`rmqrHomography`), the edge fit (`edgeFitRMQRHomography`), the timing check (`rmqrTimingAgrees`), the stages, and the decoder for one finder (`locateRMQR`, `decodeRMQRQuad`).
- `qrgen/decode_finder_cc.go`: one labelling shared by every search (`scanComponents`), and the sub-pattern candidates and their ring corners (`subPatterns`, `ringCorners`).
- `qrgen/decode_image.go`: rectangular sampling (`sampleGrid`), and the rMQR pass after the Micro pass with the error contract.
- `qrgen/decode_multi.go`: the rMQR pass in `DecodeAll`, and `Result.RMQRVersion`.

## References

- ISO/IEC 23941:2022 — the rMQR layout, through zint's and zxing-cpp's source (doc 28).
- zxing-cpp 2.3.0, core/src/qrcode/QRDetector.cpp (`SampleRMQR`) and QRReader.cpp — prior art, and the independent reader used for validation.
- `docs/theory/24-arbitrary-angle-detection.md` — the connected-component detector, corner extraction, and least-squares homography.
- `docs/theory/25-multi-symbol-decoding.md` — the claim loop the rMQR pass joins.
- `docs/theory/27-micro-qr-detection.md` — the single-finder search, the half-pixel correction, and the edge fit this detector extends.
- `docs/theory/28-rmqr.md` — the rMQR symbol: layout, format information, and the matrix decoder this detector feeds.
