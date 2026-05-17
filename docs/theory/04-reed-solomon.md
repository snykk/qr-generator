# Reed–Solomon Error Correction

Reed–Solomon (RS) codes are the redundancy mechanism that lets a QR code be read despite missing or corrupted modules. This document describes how QR applies RS — encoder-only, since this library does not implement a decoder.

## RS in one paragraph

An RS code over GF(q) treats `k` source symbols as the coefficients of a message polynomial `M(x)`, multiplies it by `xⁿ` where `n` is the number of EC symbols, divides the product by a *generator polynomial* `g(x)`, and appends the remainder. The resulting polynomial has degree less than `k + n` and is the codeword. Because the generator's roots are consecutive powers of `α`, the code can correct up to `n / 2` symbol errors per block. For QR `q = 256`, so each symbol is a single byte (called a *codeword*).

## Generator polynomial

For `n` EC codewords the generator is:

    g(x) = (x − α⁰)(x − α¹)(x − α²) … (x − α^(n−1))

multiplied out over GF(256). Examples (coefficients written as powers of α from the leading term down):

- `n = 7`:  `x⁷ + α⁸⁷x⁶ + α²²⁹x⁵ + α¹⁴⁶x⁴ + α¹⁴⁹x³ + α²³⁸x² + α¹⁰²x + α²¹`
- `n = 10`: `x¹⁰ + α²¹⁵x⁹ + α¹⁹⁴x⁸ + …`

In code we build `g` iteratively: start with `[1]`, then for each `i` in `0..n−1` multiply the current polynomial by `(x − αⁱ)`. Because subtraction equals addition in GF(2⁸), this is `(x + αⁱ)`.

## Encoding a block

Given a block of `k` data codewords:

1. Pad with `n` zero codewords on the right → polynomial of degree `k + n − 1`.
2. Divide by `g(x)` using polynomial long division over GF(256).
3. The remainder, of degree `< n`, is the block's EC codewords.

Pseudo-code:

```text
function encodeBlock(data[0..k-1], g[0..n]):
    result = data ++ zeros(n)
    for i in 0..k-1:
        coef = result[i]
        if coef != 0:
            for j in 0..n:
                result[i + j] = result[i + j] XOR mul(g[j], coef)
    return result[k..k+n-1]   # the remainder
```

## QR-specific block structure

For most (version, EC level) combinations the codewords are split into two groups of blocks with two different data sizes. ISO/IEC 18004:2015 Table 9 specifies, for each (version, EC level):

- the number of error-correction codewords per block (the same for every block in the (version, EC level) pair),
- group 1: number of blocks and data codewords per block,
- group 2: number of blocks and data codewords per block (zero if not used).

Example — version 5 / EC level Q:

| Group | Blocks | Data codewords / block | EC codewords / block |
|:-----:|:------:|:----------------------:|:--------------------:|
|   1   |   2    |          15            |          18          |
|   2   |   2    |          16            |          18          |

Total data = `2·15 + 2·16 = 62` codewords. EC budget = `4 · 18 = 72` codewords. The encoder must split the encoded byte stream accordingly and run the per-block encoder on each piece.

## Interleaving

After per-block encoding we have, e.g., four data blocks D₁..D₄ and four EC blocks E₁..E₄. The final transmission order is *column-major* over blocks:

```text
D₁[0], D₂[0], D₃[0], D₄[0],
D₁[1], D₂[1], D₃[1], D₄[1],
...
D₁[max], D₂[max], D₃[max], D₄[max],
E₁[0], E₂[0], E₃[0], E₄[0],
...
```

When blocks have different data lengths, shorter blocks contribute nothing in positions beyond their length — those slots are simply skipped. The EC blocks always share the same length for a given (version, EC level), so EC interleaving is straightforward.

The interleaved byte sequence is the codeword stream placed into the matrix.

## Remainder bits

For certain versions an extra 0, 3, 4, or 7 zero bits are appended to the codeword stream so that its length matches the data-area cell count exactly. The per-version count is in ISO/IEC 18004:2015 Table 1.

## Implementation pointers

- `qrgen/reedsolomon.go`: `genPoly(n) []byte`, `encodeBlock(data, n) []byte`.
- Block splitting and interleaving live next to the encoder driver, sharing the codeword tables with `qrgen/version.go`.

## References

- ISO/IEC 18004:2015, §8.5–§8.6 and Tables 9 and 13–22.
- Wicker, S. B., *Error Control Systems for Digital Communication and Storage*, Prentice Hall, 1995 — chapter on RS codes.
- Project Nayuki, "Creating a QR Code step by step" — worked numerical example through encoding and interleaving.
