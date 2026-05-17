# Matrix Construction

This stage takes the interleaved codeword bit stream and arranges it into the QR module grid, alongside the fixed functional patterns.

## Coordinate system

We use `(row, column)` with `(0, 0)` at the top-left, matching the orientation we use when rendering to images. The matrix is square of side `n = 21 + 4·(v − 1)` modules.

## Functional patterns

### Finder patterns

A finder is a 7×7 pattern:

```
1 1 1 1 1 1 1
1 0 0 0 0 0 1
1 0 1 1 1 0 1
1 0 1 1 1 0 1
1 0 1 1 1 0 1
1 0 0 0 0 0 1
1 1 1 1 1 1 1
```

It is placed at:

- top-left at `(0, 0)`,
- top-right at `(0, n − 7)`,
- bottom-left at `(n − 7, 0)`.

### Separators

A 1-module-wide light strip surrounds each finder on the sides facing the data area. Together a finder plus its separator occupies an 8×8 region.

### Timing patterns

Row 6 and column 6 are filled with alternating dark/light modules, starting with dark, in the region between the finders. They are written *after* the finders so that the corners remain as the finder pattern dictates.

### Alignment patterns

A 5×5 concentric pattern:

```
1 1 1 1 1
1 0 0 0 1
1 0 1 0 1
1 0 0 0 1
1 1 1 1 1
```

Centred at the coordinates listed in ISO/IEC 18004:2015 Annex E. Versions 2 through 40 have between 1 and 46 alignment patterns. The rule of thumb is that they sit where both the row centre and the column centre come from a per-version coordinate list, *excluding* positions that would clash with a finder pattern.

### Dark module

A single dark module at `(4·v + 9, 8)` for every version. It belongs to the format-information area but is always set to dark by the spec.

### Reserved areas

Two 15-bit strips around the top-left finder, and (for v ≥ 7) two 6×3 blocks adjacent to the top-right and bottom-left finders, are reserved for format and version information. They must be marked off during data placement so that the data-bit stream skips them.

## Placing data bits

After the functional patterns are in place, the data stream is woven into the remaining modules using this rule (ISO/IEC 18004:2015 §8.7.3):

1. Walk **upward** in a two-column-wide band starting from the right edge.
2. Inside each band, write right-column then left-column for each row.
3. When the top is reached, shift left by two columns and walk **downward**.
4. Repeat until every non-functional module has been visited.
5. **Column 6** (the vertical timing pattern) is skipped entirely — the band that would cover it is moved one column to the left.
6. Skip any module belonging to a functional or reserved area.

The most-significant bit of the codeword stream is written first, into the first available module visited by the walk.

For a version-1 matrix the walk begins at the column pair (20, 19), visiting rows 20 down to 0; then (18, 17); then (16, 15); then (14, 13); then (12, 11); then (10, 9); then (8, 7); then (5, 4) (because column 6 is skipped); then (3, 2); then (1, 0).

## Why this scheme

- Finders are large and asymmetric so a decoder can detect orientation regardless of how the symbol is held.
- Alignment patterns cap perspective distortion in larger symbols, at the cost of a fixed set of off-limits modules.
- Timing patterns are predictable, giving a calibration reference for module pitch even when the surface is curved.
- The zig-zag walk visits every non-functional module exactly once, which lets the decoder reverse it deterministically.

## Implementation pointers

- `qrgen/matrix.go`: a `Matrix` struct wrapping `[][]bool` plus a parallel `[][]bool` "reserved" mask that marks functional and reserved modules.
- Place functional patterns first; then run the data walk; later apply masking and write format/version info into the reserved area.

## References

- ISO/IEC 18004:2015, §6.3 (Symbol structure), §8.7 (Codeword placement in matrix), Annex E (Alignment pattern positions).
- Thonky, "Module Placement" — <https://www.thonky.com/qr-code-tutorial/module-placement-matrix>
