# QR Encoder — SVG Renderer Plan

This document describes the implementation plan for the **SVG renderer** enhancement that targets the `v0.5.0` minor release. It is the first output-format addition after the original PNG renderer (v0.1.0) and opens an encoder/output-breadth phase following two decoder-robustness releases (v0.3.0 adaptive thresholding, v0.4.0 rotation handling).

> Status: **draft / living document.** Milestones S1..S6 land incrementally on the `svg-renderer` branch; each is a focused commit (or small commit series) with tests, matching the cadence used for M1..M11, D1..D14, T1..T6, and R1..R6.

> Indonesian version: [docs/plan-svg-renderer.id.md](plan-svg-renderer.id.md).

---

## 1. Vision & Goals

- Add a **scalable vector output** to the library so callers can produce resolution-independent QR codes — crisp at any size and trivially embeddable in HTML, print pipelines, and design tools. (Note: raw SVG is larger than the equivalent PNG for a QR symbol; the win is scalability and embeddability, not file size — see the corrected size note in section 6 and doc 16 section 1.)
- Expose it as **additive public functions** `EncodeSVG` and `EncodeSVGToFile` that mirror the existing `Encode` / `EncodeToFile` shape and reuse every existing option (`WithECLevel`, `WithVersion`, `WithMask`, `WithModuleSize`, `WithQuietZone`, `WithColors`). No breaking change to `Encode`'s documented PNG-bytes contract.
- Keep the **same philosophy** as every prior milestone: pure Go, zero runtime dependencies beyond the standard library (SVG is plain text emitted with `strings`/`fmt`), spec-first with a bilingual theory doc, and golden/round-trip tests.
- **Correct the documentation debt** that the triage surfaced: `docs/theory/08-rendering.md` promises that "other formats can be added later behind the same `Render` interface", but no such interface exists. v0.5.0 deliberately does **not** introduce that interface (YAGNI — there are only two renderers and neither is selected polymorphically at runtime, matching the straight-code-over-strategy-interface decision already made for the Sauvola dispatch in v0.3). Instead it adds `renderSVG` as a sibling of `renderPNG` sharing the existing `renderOptions`, and rewrites the doc 08 sentence to describe sibling render functions rather than an interface.

## 2. Design Principles

1. **Sibling, not interface.** `renderSVG(m *matrix, opts renderOptions) ([]byte, error)` sits next to `renderPNG` with the identical signature. `EncodeSVG` calls `renderSVG` directly the same way `Encode` calls `renderPNG` directly. No `Renderer` interface, no `WithFormat` enum — the format is chosen by which function you call, which keeps each function's return contract unambiguous.
2. **Reuse the whole front half.** `EncodeSVG` runs the exact same `resolveOptions -> validate -> buildMatrix` pipeline as `Encode`; only the final render call differs. The matrix, masking, EC, and option plumbing are untouched, so the mature encoder path carries zero regression risk.
3. **Compact path-data, not one rect per module.** Emit a single `<path>` whose `d` attribute draws every dark module, rather than hundreds of individual `<rect>` elements. This is the standard approach (Nayuki, qrcode-svg) and keeps file size small. Run-length merging of horizontal dark runs is a possible future optimisation, noted but not required for v0.5.
4. **Decodability first.** Set `shape-rendering="crispEdges"` so renderers do not anti-alias module boundaries into blurry greys that would hurt downstream scanning. Use an integer-friendly coordinate system.
5. **Resolution independence with PNG-compatible nominal size.** The `viewBox` is expressed in module units (`0 0 (n+2qz) (n+2qz)`) so the symbol scales cleanly to any display size, while `width`/`height` default to `moduleSize * (n + 2*quietZone)` pixels so an SVG and a PNG produced with the same options describe the same nominal dimensions.
6. **Honour WithColors including alpha.** Foreground/background convert to `#RRGGBB` hex; when a colour carries alpha below full opacity, emit `fill-opacity` alongside the hex rather than relying on the not-universally-supported 8-digit hex form.
7. **Tests first.** Every milestone ships with table-driven Go tests. The renderer is validated structurally (well-formed XML, correct element/attribute presence, dark-module count matches the matrix) and end-to-end where feasible.

## 3. Scope

### In scope for v0.5.0

- `renderSVG(m *matrix, opts renderOptions) ([]byte, error)` in a new `qrgen/render_svg.go`.
- Public `EncodeSVG(text string, opts ...Option) ([]byte, error)` and `EncodeSVGToFile(text, path string, opts ...Option) error`.
- Full reuse of the existing option set; `WithModuleSize`, `WithQuietZone`, and `WithColors` all take effect on the SVG output.
- Single-`<path>` dark-module rendering, background rect covering the full canvas including the quiet zone, `crispEdges`, module-unit `viewBox`.
- Colour-to-hex conversion with alpha handled via `fill-opacity`.
- CLI support: `cmd/qrgen` learns to emit SVG, either via a `-format svg` flag or by detecting a `.svg` extension on `-out` (decided in S5).
- Theory doc `docs/theory/16-svg-rendering.md` (EN + ID) and the correction to `docs/theory/08-rendering.md`.
- A runnable example under `examples/encode/svg`.

### Out of scope (still)

- **Terminal / ASCII renderer.** Deferred to a later minor (different API shape — a string or `io.Writer`, not `[]byte`); noted in the roadmap.
- **JPEG / PDF renderers.** Not requested; JPEG is lossy and bad for QR, PDF needs a heavier writer.
- **A `Renderer` interface or `WithFormat` enum.** Explicitly rejected per principle 1.
- **Run-length / rectangle-merge path optimisation.** The per-module path is compact enough for v0.5; merging is a future size tweak.
- **Logo embedding in SVG.** Tracked separately on the roadmap.

---

## 4. Milestones

Milestones land sequentially. **Checkpoint A** (after S4) gives a working `EncodeSVG` validated end-to-end. **Checkpoint B** (S6) is the `v0.5.0` release.

### S1 — Plan Doc `(S)`

Goal: this document and its Indonesian counterpart, committed before any code or theory lands.

- [ ] `docs/plan-svg-renderer.md` and `docs/plan-svg-renderer.id.md` covering vision, principles, scope, milestones S1..S6, file-layout delta, risks, references, open questions.

### S2 — SVG Theory Doc + Doc 08 Correction `(S)`

Goal: cover the SVG document model and rendering choices in `docs/theory/` before any code lands, and pay down the phantom-interface documentation debt.

- [x] `docs/theory/16-svg-rendering.md` — eight sections: why SVG, the SVG document model, single-path drawing vs one rect per module, the module-unit coordinate system with pixel sizing, `crispEdges` and decodability, colour-to-hex with `fill-opacity` for alpha (including the 0x101 division and premultiplied-alpha note), the sibling-function-not-interface rationale, and implementation pointers.
- [x] Indonesian counterpart `docs/theory/16-svg-rendering.id.md`.
- [x] Corrected `docs/theory/08-rendering.md` and its `.id.md`: rewrote the "behind the same `Render` interface" sentence to describe sibling render functions sharing `renderOptions` and the `func(m *matrix, opts renderOptions) ([]byte, error)` signature, with a cross-link to doc 16 section 7.
- [x] Updated `docs/theory/README.md` and `docs/theory/README.id.md`: entry 16 under a new "Output formats (v0.5.0)" subsection plus a code-mapping row pointing at `qrgen/render_svg.go` (planned, S3).
- [ ] **Surfaced separately (not yet actioned):** doc 08 also documents a contrast check and a `WithSkipContrastCheck` option that are not implemented in the code (only an advisory comment on `WithColors`). Flagged to the maintainer to decide between implementing the check or removing it from the doc; out of scope for S2's interface correction.

### S3 — `renderSVG` Implementation `(M)`

Goal: the renderer itself, sharing `renderOptions` with `renderPNG`.

- [x] `qrgen/render_svg.go` with `renderSVG(m *matrix, opts renderOptions) ([]byte, error)`: applies `withDefaults`, guards nil matrix and invalid dimension/side, emits the XML declaration, the `<svg>` root with module-unit `viewBox` and pixel `width`/`height` plus `shape-rendering="crispEdges"`, a full-canvas background `<rect>`, and a single foreground `<path>` whose `d` draws each dark module as `M(c+qz) (r+qz)h1v1h-1z` offset by the quiet zone. The `<path>` is omitted entirely for an all-light matrix.
- [x] `colorToHex(c color.Color) (string, float64)` un-premultiplies via `channel*0xff/a` (which collapses the un-premultiply and 16-to-8-bit steps into one exact division), returns `#000000, 0` for fully transparent input, and fractional opacity `a/0xFFFF` otherwise.
- [x] `writeOpacity` emits a `fill-opacity` attribute only when opacity `< 1`, formatting with the shortest round-tripping decimal (`strconv.FormatFloat(op, 'g', 4, 64)`), so the common opaque black-on-white case stays minimal.
- [x] Tests in `qrgen/render_svg_test.go`: every output parses via `encoding/xml`; `viewBox`/`width`/`height` checked against the option math for default and custom module-size/quiet-zone; background and foreground fills checked for a custom opaque colour pair; the move-command count equals the dark-module total; an `image/color.NRGBA` half-alpha foreground asserts `fill-opacity ≈ 0.5`; an all-light matrix emits no `<path>`; nil matrix errors; and a `colorToHex` table covers black, white, opaque navy, and fully transparent. Manually confirmed a real V1 "HI" SVG rasterises to a valid three-finder QR via qlmanage.

### Checkpoint A — `renderSVG` produces well-formed, option-correct SVG validated structurally.

### S4 — Public API + Examples `(M)`

Goal: expose the renderer and prove the round trip.

- [x] `EncodeSVG(text string, opts ...Option) ([]byte, error)` and `EncodeSVGToFile(text, path string, opts ...Option) error` added to `qrgen/api.go`, each running the identical `resolveOptions -> validate -> buildMatrix` front-half as `Encode` and only swapping `renderPNG` for `renderSVG`; the file variant writes with mode 0644.
- [x] Doc comments mirror `Encode`/`EncodeToFile`, note the shared option set, and point at doc 16.
- [x] **Cross-validation:** `TestEncodeSVGRoundTripGrid` reconstructs the module grid from the emitted path — reading the canvas dimension from the `viewBox`, deriving the quiet zone as `(dim - n) / 2`, and walking every `M x y` subpath — then asserts it equals `Matrix(text, opts...)` cell for cell. Covers V1-M default, a URL at EC-Q, a small-quiet-zone numeric payload, a multi-block EC-H payload, and a custom-colour case. This closes an encode -> SVG -> grid loop analogous to the decoder round-trip tests, dependency-free.
- [x] Runnable example `examples/encode/svg/main.go` writes a styled navy-on-cream SVG; verified end-to-end (`go run ./examples/encode/svg`).
- [x] `qrgen/api_svg_test.go` also covers module-size propagation into `width`/`height`, file output well-formedness, and the invalid-option error path. Race-clean.

### S5 — CLI SVG Support `(S)`

Goal: make SVG reachable from the `qrgen` binary.

- [x] Surface settled as both: a `-format png|svg` flag wins when set, and a `.svg` extension on `-out` infers SVG when `-format` is unset, via the `resolveFormat` helper. PNG stays the default for everything else.
- [x] `runEncode` builds the shared option list once, then dispatches to `EncodeSVG` or `Encode` by format; the default output filename becomes `qr.svg` for SVG and stays `qr.png` otherwise. Existing PNG invocations are byte-for-byte unchanged.
- [x] CLI help banner and `cmd/qrgen` package doc gained SVG examples (`-out url.svg` inference and `-format svg -out -` to stdout).
- [x] Tests in `cmd/qrgen/main_test.go`: `-format svg` writes parseable SVG to a file (even with a non-`.svg` name) and to stdout; `.svg` extension inference works without `-format`; an invalid `-format gif` errors; and a no-format `.png` output still decodes as PNG so the default did not regress. Smoke-tested the built binary. Race-clean.

### S6 — Benchmarks, Docs Polish & Release `(S)`

Goal: cut `v0.5.0`.

- [x] Added `BenchmarkEncodeSVGSmall` and `BenchmarkEncodeSVGURL` in `qrgen/bench_test.go`. Measured (Apple M5): SVG encode is ~5x faster than PNG (87us vs 451us for the small payload; 175us vs 845us for the URL) with far fewer allocations (39 KB/op vs 926 KB/op), because it skips rasterisation and zlib.
- [x] README: new `## Rendering to SVG` section with a code sample and an honest size note, `EncodeSVG`/`EncodeSVGToFile` rows in the API summary table, CLI `-format svg` and `.svg`-inference examples, and Scope/Roadmap updates (SVG removed from "still out of scope" and from the renderers roadmap bullet, which now reads terminal/JPEG/PDF with a "SVG shipped in v0.5" note; Scope now lists SVG output).
- [x] `CHANGELOG.md` `v0.5.0` entry under Added / Changed / Validated plus a "Note on file size" recording the measured PNG-vs-SVG numbers, and the bottom-of-file compare/tag anchors.
- [x] **Corrected an inaccuracy found during S6:** the plan and theory doc had claimed SVG is "smaller than PNG". Measurement showed the opposite (raw SVG ~5x larger; gzipped roughly PNG-sized), so doc 16, doc 08-adjacent claims, and both plan files were corrected to state the real trade-off — scaling and embeddability, not size.
- [x] `go test -race ./...` clean. The shared encoder front-half is untouched, so the existing PNG encode benchmarks are unaffected; only the new SVG path was added.
- [ ] Tag `v0.5.0` on the merge commit after pushing the branch to GitHub so the tag lands on the commit the remote sees. Left for the user to run manually; annotation recommended in the release conversation.

---

## 5. Proposed File Layout Delta

```
qrgen/
├── render_png.go            # existing — unchanged
├── render_svg.go            # new — renderSVG + colorToHex helper
├── render_svg_test.go       # new — structural SVG unit tests
├── api.go                   # existing — gains EncodeSVG / EncodeSVGToFile (or a new api_svg.go)
├── api_svg_test.go          # new — byte/file output + grid round-trip tests
└── encode_bench_test.go     # existing or new — gains BenchmarkEncodeSVG*
cmd/qrgen/
├── main.go                  # existing — gains -format flag / .svg inference
└── main_test.go             # existing — gains SVG CLI tests
examples/encode/svg/
└── main.go                  # new — runnable SVG demo
docs/
├── plan-svg-renderer.md     # this file
├── plan-svg-renderer.id.md  # Indonesian counterpart
└── theory/
    ├── 08-rendering.md       # corrected: sibling render funcs, not a Render interface
    ├── 08-rendering.id.md    # same correction
    ├── 16-svg-rendering.md   # new
    └── 16-svg-rendering.id.md # new
```

## 6. Risks & Technical Notes

- **XML correctness and escaping.** SVG is XML; numeric attributes are fully controlled by us so injection is not a concern, but the emitter must produce well-formed output (proper namespace, closed tags, quoted attributes). Tests parse the output with `encoding/xml` to guarantee well-formedness rather than eyeballing strings.
- **Anti-aliasing at fractional scales.** If a viewer scales the module-unit `viewBox` to a non-integer pixel size, module edges can blur. `shape-rendering="crispEdges"` mitigates this; the theory doc records the trade-off and why we still prefer a module-unit coordinate system for scalability.
- **Colour-model conversion.** `color.Color.RGBA()` returns 16-bit premultiplied channels; converting to 8-bit hex must divide by 0x101 (not bit-shift alone) to round correctly, and alpha handling must avoid double-premultiplication. The `colorToHex` helper is unit-tested against known colours.
- **File-size expectations (corrected after S6 measurement).** Raw SVG is *larger* than the equivalent PNG, not smaller — PNG's zlib is very tight on a monochrome bitmap. Measured: V1 "HELLO WORLD" is 632 B PNG vs 3209 B raw SVG (719 B gzipped); the ~40-byte URL is 1155 B PNG vs 6047 B raw SVG (1205 B gzipped). Gzipped SVG is roughly PNG-sized, and SVG encoding is ~5x faster with far fewer allocations because it skips rasterisation and zlib. Run-length merging would shrink the raw path; deferred. The docs were corrected to stop claiming SVG is smaller than PNG.
- **No decoder impact.** The decoder never reads SVG. There is zero change to the decode path, so the entire decoder test and benchmark suite is unaffected — but `go test -race ./...` still runs it to be sure nothing in the shared package broke.
- **CLI surface creep.** Adding `-format` must not change the default behaviour of existing PNG invocations; the flag defaults to PNG and `.svg` inference only triggers when `-format` is unset.

## 7. References

- ISO/IEC 18004:2015 — §11 (symbol rendering is implementation-defined; the spec constrains the module grid, not the output medium).
- W3C — *Scalable Vector Graphics (SVG) 1.1 (Second Edition)*: <https://www.w3.org/TR/SVG11/>. Path data grammar (§8.3), `shape-rendering` (§11.2), basic shapes.
- Project Nayuki — *QR Code generator library*: its `toSvgString` method renders the whole symbol as a single path, the approach adopted here. <https://www.nayuki.io/page/qr-code-generator-library>
- `docs/theory/08-rendering.md` — the existing PNG rendering notes, corrected in S2.

## 8. Open Questions

To be answered before the corresponding milestone starts:

- **CLI format surface.** Explicit `-format png|svg`, extension inference from `-out`, or both? Leaning toward both: `-format` wins when set, `.svg` extension infers when it is not. Settled in S5.
- **API file placement.** Put `EncodeSVG`/`EncodeSVGToFile` in the existing `api.go` next to `Encode`, or a dedicated `api_svg.go`? Leaning toward `api.go` for discoverability since the surface is small; revisit if it crowds the file.
- **Path optimisation.** Ship the simple per-module path in v0.5 and leave run-length rectangle merging for a later size-focused pass? Default yes — correctness and clarity first.
- **Grid round-trip depth.** Is parsing the emitted path back into `[][]bool` sufficient cross-validation, or should a test also rasterise the SVG via a third-party renderer? Default to path-parse only to keep the test dependency-free, consistent with the stdlib-only policy.
