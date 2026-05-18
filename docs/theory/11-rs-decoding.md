# Reed–Solomon Decoding

Reed–Solomon decoding inverts the EC stage from [04-reed-solomon.md](04-reed-solomon.md), but the algorithmic toolkit is completely different. Encoding multiplied by a generator polynomial; decoding has to identify *where* errors are and *what* the original symbols were, from nothing more than the corrupted received codewords. This document covers the four-step decoder that `qrgen/rs_decode.go` (built in milestone D3) implements.

> Indonesian version: [11-rs-decoding.id.md](11-rs-decoding.id.md).

## Setup

A received codeword `R = (r_0, r_1, …, r_{n+k-1})` is the sum of the transmitted codeword `C` and an unknown error vector `E`:

```text
R(x) = C(x) + E(x)
```

`C(x)` is divisible by the generator polynomial `g(x)` because the encoder enforced that. `E(x)` has at most `t = floor(n/2)` non-zero coefficients (the spec's promise that we can correct up to `t` errors per block when there are `n` EC codewords). The decoder's job is to recover `E(x)` so we can compute `C(x) = R(x) − E(x)` over GF(256), then strip the EC tail and read the data codewords.

The decoder has no idea where the errors are. The recovery procedure has four well-defined stages.

## Stage 1 — Syndromes

A *syndrome* is `R(α^i)` for `i = 0, 1, …, n-1`. Since `C(α^i) = 0` for those `i` (they are the roots of `g(x)`), each syndrome `S_i` equals `E(α^i)`:

```text
S_i = R(α^i) = C(α^i) + E(α^i) = 0 + E(α^i) = E(α^i)
```

If all `n` syndromes are zero, `E(x)` has no contribution at any of those roots — the decoder concludes the codeword is error-free and returns the data part directly. Otherwise we have `n` non-trivial equations in the unknown error positions and magnitudes.

Implementation: compute each `S_i` via Horner's evaluation. This is what `polyEval` from D2 is for.

## Stage 2 — The error-locator polynomial

Let there be `v` actual errors at positions `j_1, …, j_v` with magnitudes `Y_1, …, Y_v`. Define `X_k = α^{j_k}` — the "error locator" for position `j_k`. The decoder builds a polynomial whose roots are `X_k^{-1}`:

```text
Λ(x) = ∏_{k=1}^{v} (1 − X_k · x)
```

If we know `Λ(x)`, we know the error positions (apply Chien search; see stage 3). The challenge is that `v`, the `X_k`, and the `Y_k` are all unknown — we only have the `n` syndromes.

The link between syndromes and `Λ(x)` is the **key equation**:

```text
S_i · Λ_0 + S_{i-1} · Λ_1 + … + S_{i-v} · Λ_v = 0   for i ≥ v
```

That is, `Λ` is the shortest linear recurrence that produces the syndrome sequence. Finding the shortest such recurrence is exactly what Berlekamp–Massey does.

## Stage 3 — Berlekamp–Massey

Berlekamp–Massey (BM) is an iterative algorithm that walks through the syndromes, growing `Λ(x)` minimally as new information demands it. Pseudo-code:

```text
function berlekampMassey(S [n]GF):
    Λ      = [1]              # current locator estimate (length L+1)
    B      = [1]              # last "good" locator
    L      = 0                # current length
    m      = 1                # steps since last L update
    b      = 1                # discrepancy used last time L grew

    for i = 0..n-1:
        δ = S[i] + Σ_{j=1..L} Λ[j] · S[i-j]   # discrepancy
        if δ == 0:
            m += 1
        else if 2*L <= i:
            T = Λ
            Λ = Λ − (δ/b) · x^m · B
            L = i + 1 − L
            B = T
            b = δ
            m = 1
        else:
            Λ = Λ − (δ/b) · x^m · B
            m += 1
    return Λ, L
```

Two notes for QR specifically:

- All arithmetic is over GF(256). `δ/b` uses `gf256Inverse` from D2.
- The "shift by `x^m`" means prepend `m` zeros to `B` before subtracting.

After BM terminates we know `L` (the number of errors) and `Λ(x)`. If `L > t = n/2`, the codeword has more errors than the code can correct and the decoder must give up with `ErrTooManyErrors`.

## Stage 4 — Chien search

Chien search finds the roots of `Λ(x)` by evaluating it at every non-zero element of GF(256):

```text
for i = 0..254:
    if polyEval(Λ, α^{-i}) == 0:
        record error position i
```

For QR we only care about positions `0..n+k-1`, so we can break early. The root `α^{-i}` corresponds to error position `i` (so `X_k = α^{i_k} = (α^{-i_k})^{-1}`). Implementation lives in `qrgen/rs_decode.go` alongside BM.

## Stage 5 — Forney's algorithm

Once we know *where* the errors are, Forney gives us the magnitudes. Define the error evaluator polynomial:

```text
Ω(x) = S(x) · Λ(x) mod x^n
```

where `S(x) = Σ_{i=0..n-1} S_i · x^i`. Then for each error position `j_k` with locator `X_k`:

```text
Y_k = − Ω(X_k^{-1}) / Λ'(X_k^{-1})
```

`Λ'` is the formal derivative of `Λ` — implemented in `polyDeriv` from D2. Over GF(256) the "minus" disappears (every element is its own additive inverse), so:

```text
Y_k = Ω(X_k^{-1}) / Λ'(X_k^{-1})
```

The final correction is `c_{j_k} = r_{j_k} XOR Y_k`.

## Putting it together

```text
function rsDecode(received []byte, n int) []byte:
    S = syndromes(received, n)
    if S all zero: return received[:k]              # no errors
    Λ, L = berlekampMassey(S)
    if L > n/2: error "too many errors"
    positions = chienSearch(Λ, len(received))
    if len(positions) != L: error "Λ inconsistent"  # algorithmic check
    Ω = polyMul(S, Λ) truncated to degree < n
    Λ' = polyDeriv(Λ)
    for k in 0..L-1:
        X_k = α^{positions[k]}
        Y_k = polyEval(Ω, X_k^{-1}) / polyEval(Λ', X_k^{-1})
        received[positions[k]] ^= Y_k
    return received[:k]
```

`k = len(received) − n` is the data-codeword count for the block.

## Error capacity & failure modes

| Error count | Outcome |
|:-----------:|:--------|
| 0           | All syndromes zero, fast return. |
| 1..t        | Recovered exactly. |
| t+1..n−1    | BM may or may not converge to a consistent solution; the decoder reports `ErrTooManyErrors` after consistency checks. |
| ≥ n         | Behaviour undefined — the codeword has so many errors there is no unique correction. We rely on the consistency check in Chien search to catch this. |

The decoder must always perform the consistency check that `len(chienSearch(Λ)) == L`; otherwise it can silently return wrong data, which is worse than failing loudly.

## Implementation pointers

- `qrgen/rs_decode.go` will host `syndromes`, `berlekampMassey`, `chienSearch`, `forneyMagnitudes`, and `rsDecode`.
- `polyEval`, `polyDeriv`, `polyDivQR`, and `gf256Inverse` come from D2 (`qrgen/gf256.go`).
- Tests use the HELLO WORLD encoded block from `docs/theory/10-worked-example.md` as a fixture: corrupt 1..5 bytes (the capacity for n=10) and assert exact recovery.

## References

- ISO/IEC 18004:2015, §9 — reference decode algorithm.
- Berlekamp, E. R. — *Algebraic Coding Theory* (1968).
- Massey, J. L. — "Shift-Register Synthesis and BCH Decoding," IEEE Trans. Information Theory, IT-15(1), 1969.
- Forney, G. D. — "On Decoding BCH Codes," IEEE Trans. Information Theory, IT-11, 1965.
- Plank, J. S. — "A Tutorial on Reed–Solomon Coding for Fault-Tolerance in RAID-like Systems," 1997.
