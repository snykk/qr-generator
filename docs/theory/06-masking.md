# Masking

After data placement the matrix can contain long runs of same-colour modules or accidental finder-like patterns inside the data area, both of which can confuse decoders. *Masking* permutes the data bits to break those patterns while remaining perfectly reversible.

## The 8 mask patterns

A *mask* is a function of `(row, column)`. For each data module the mask function returns a boolean; if true, the module is inverted (toggled). ISO/IEC 18004:2015 §8.8.1 defines 8 masks:

| Mask | Condition (`i = row`, `j = column`)           |
|:----:|:----------------------------------------------|
|  0   | `(i + j) mod 2 == 0`                          |
|  1   | `i mod 2 == 0`                                |
|  2   | `j mod 3 == 0`                                |
|  3   | `(i + j) mod 3 == 0`                          |
|  4   | `(floor(i/2) + floor(j/3)) mod 2 == 0`        |
|  5   | `((i·j) mod 2) + ((i·j) mod 3) == 0`          |
|  6   | `(((i·j) mod 2) + ((i·j) mod 3)) mod 2 == 0`  |
|  7   | `(((i+j) mod 2) + ((i·j) mod 3)) mod 2 == 0`  |

The mask is XOR'd into **only** the data-area modules — never into finders, alignment, timing, format, or version areas.

## Why XOR is safe

XOR is its own inverse. The decoder reads the format-information bits that name the mask, applies the same mask function, and recovers the original data bits. No information is lost because masking is a symmetric, bijective operation.

## Penalty evaluation

We apply each of the eight masks in turn and pick whichever produces the lowest *penalty score*. The score is the sum of four contributions (ISO/IEC 18004:2015 §8.8.2).

### Rule 1 — runs of same colour

For each row and for each column, find runs of 5 or more same-colour modules. A run of length `L ≥ 5` contributes `L − 2` to the score. So a length-5 run costs 3, a length-6 run costs 4, and so on.

### Rule 2 — 2×2 blocks

Each 2×2 block of modules that are all the same colour costs **3** points. Overlap counts: a 3×3 fully-dark region contains four 2×2 sub-blocks, for 12 points.

### Rule 3 — finder-like patterns

Each occurrence of the pattern `1 0 1 1 1 0 1 0 0 0 0` — or its reverse `0 0 0 0 1 0 1 1 1 0 1` — in any row or column costs **40** points. These 11-module patterns imitate a finder plus separator and would confuse a decoder if left in the data area.

### Rule 4 — dark module ratio

Let `r` be the percentage of dark modules across the whole symbol. Compute:

    k = floor( |r − 50| / 5 )

The penalty is `10 · k`. A perfectly balanced matrix scores 0 from this rule; every 5% deviation from 50/50 costs 10 points.

## Picking the best mask

For each mask `k` in `0..7`:

1. Clone the matrix.
2. Apply mask `k` to data modules only.
3. Write format-info bits computed for `(EC level, k)` (see [07-format-version-info.md](07-format-version-info.md)).
4. Sum the four penalty contributions.

The mask with the minimum total score wins. Ties are broken in favour of the lowest mask index.

## Practical tips

- For v0.1 a straightforward O(n²) pass per rule is fine; the matrix is small enough that incremental evaluation is not worth the complexity.
- Build the masked matrix in place but keep a copy of the unmasked matrix if you plan to evaluate multiple masks side by side, or remember to undo the XOR before trying the next mask.

## Implementation pointers

- `qrgen/mask.go`: `applyMask(m *Matrix, k int)`, `penalty(m *Matrix) int`, `selectMask(m *Matrix) (k int, masked *Matrix)`.

## References

- ISO/IEC 18004:2015, §8.8 (Masking).
- Thonky, "Data Masking" — <https://www.thonky.com/qr-code-tutorial/data-masking>
