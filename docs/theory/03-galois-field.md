# Galois Field GF(256)

Reed–Solomon error correction for QR is performed over the finite field **GF(2⁸)**, written GF(256). This document gives just enough algebra to implement the operations correctly.

## What a finite field is

A *field* is a set with addition, subtraction, multiplication, and division (by non-zero elements) that obeys the usual algebraic axioms. A *finite field* has finitely many elements; for any prime power `pⁿ` there is, up to isomorphism, exactly one field of order `pⁿ`. For QR we use `p = 2`, `n = 8`, so the field has 256 elements, which fit neatly in a byte.

## Representation

Elements of GF(2⁸) are polynomials in `x` of degree ≤ 7 with coefficients in GF(2) (i.e. 0 or 1). The byte `b₇b₆b₅b₄b₃b₂b₁b₀` represents:

    b₇·x⁷ + b₆·x⁶ + … + b₁·x + b₀

So `0x57` (`01010111`) is `x⁶ + x⁴ + x² + x + 1`.

## Addition and subtraction

Addition in GF(2) is XOR (1 + 1 = 0). Subtraction is the same as addition, because every element is its own additive inverse.

```text
add(a, b) = a XOR b
sub(a, b) = a XOR b
```

## Multiplication

Multiplication is polynomial multiplication modulo a fixed *primitive polynomial*. QR uses:

    p(x) = x⁸ + x⁴ + x³ + x² + 1   (binary 0x11D)

A correct but slow implementation does the schoolbook multiply and reduces the high bits on every shift:

```text
function mul(a, b):
    r = 0
    while b != 0:
        if b AND 1:
            r = r XOR a
        a = a SHL 1
        if a AND 0x100:
            a = a XOR 0x11D
        b = b SHR 1
    return r
```

In practice we precompute two tables so multiplication and division become O(1) lookups.

## Exp/log tables and the generator α

The element `α = 0x02` (the polynomial `x`) is a *generator* of the multiplicative group: its successive powers `α⁰, α¹, …, α²⁵⁴` enumerate all 255 non-zero field elements.

Define:

```text
exp[i] = αⁱ        for i in 0..254
log[αⁱ] = i        for i in 0..254
```

Then for non-zero `a`, `b`:

```text
a · b = exp[(log[a] + log[b]) mod 255]
a / b = exp[(log[a] − log[b] + 255) mod 255]
```

`log[0]` is undefined; the implementation must guard against it.

## Worked example: α³ · α⁵

- `log[α³] = 3`, `log[α⁵] = 5`.
- Sum = 8.
- Result = `α⁸ = exp[8]`.

Compute `exp[8]` step by step from `α¹ = 0x02`:

```text
α¹ = 0x02
α² = 0x04
α³ = 0x08
α⁴ = 0x10
α⁵ = 0x20
α⁶ = 0x40
α⁷ = 0x80
α⁸ = (0x80 SHL 1) = 0x100 → reduce by 0x11D → 0x100 XOR 0x11D = 0x1D
```

So `α³ · α⁵ = 0x1D`. Cross-check by direct polynomial multiplication: `x³ · x⁵ = x⁸`, reduced modulo `p(x)` gives `x⁴ + x³ + x² + 1 = 0x1D`. ✓

## Polynomials over GF(256)

Reed–Solomon needs polynomials whose coefficients are themselves GF(256) elements. They are represented as `[]byte`, with coefficient `i` paired with `x^(deg − i)` (highest degree first). Polynomial multiplication is the usual convolution but with `+` replaced by `XOR` and scalar `*` replaced by the GF(256) `mul` above.

## Implementation pointers

- `qrgen/gf256.go` will host `exp` and `log` tables initialised in `init()`, scalar `mul`/`div`/`pow` helpers, and polynomial helpers `polyMul`, `polyMod` used by `qrgen/reedsolomon.go`.
- Build the tables once and treat them as immutable.

## References

- ISO/IEC 18004:2015, §8.5 (Error correction coding).
- Reed, I. S. and Solomon, G., "Polynomial Codes over Certain Finite Fields," *J. SIAM*, 8(2), 1960, pp. 300–304.
- Plank, J. S., "A Tutorial on Reed–Solomon Coding for Fault-Tolerance in RAID-like Systems," *Software – Practice and Experience*, 27(9), 1997 — accessible introduction with worked examples.
