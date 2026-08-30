package qrgen

import (
	"math"
	"sort"
)

// This file implements an arbitrary-angle finder detector based on
// connected-component analysis, used as a fallback when the 1:1:3:1:1 line
// scanner in decode_image.go fails under strong perspective. The line scan
// measures a one-dimensional run ratio, which foreshortening past ~30 degrees
// of tilt distorts beyond tolerance; this detector instead matches the
// finder's topological nesting -- a dark ring around a light ring around a
// dark centre, concentric, with 7:5:3 bounding-box sides -- which a projective
// transform preserves. See docs/theory/24-arbitrary-angle-detection.md.

const (
	// minFinderOuterPx ignores nested triples whose outer ring is smaller than
	// one pixel per module: below this the inner structure is unresolvable and
	// any match is noise.
	minFinderOuterPx = 7.0
	// maxFinderAspect caps the outer-ring bounding-box aspect ratio. A tilted
	// square's axis-aligned box grows non-square as it foreshortens, so the
	// ceiling is generous; downstream triple consistency filters false matches.
	maxFinderAspect = 3.0
	// maxHollowFill requires the outer ring to be hollow: a 7-module ring fills
	// only ~49% of its box, a solid dark blob fills ~100%, so this rejects
	// solid squares that are not finder rings.
	maxHollowFill = 0.90
	// nestSideTol is the relative tolerance on a child/parent bounding-box side
	// ratio (5/7 for the light ring in the dark ring, 3/5 for the centre in the
	// light ring). Perspective scales parent and child together, so the ratio
	// survives and the tolerance need only absorb discretisation.
	nestSideTol = 0.45
	// concentricFrac bounds a nested region's centroid offset from its parent's
	// centroid, as a fraction of the parent bounding-box side. Affine maps send
	// a symmetric region's centroid to the shape centre, so all three coincide.
	concentricFrac = 0.30
	// maxCCCandidates caps how many candidates feed the O(n^3) triple search,
	// bounding cost on a noisy image; the largest-module candidates are kept
	// since real finders are the biggest nested squares.
	maxCCCandidates = 64
)

// ccStats accumulates the bounding box, pixel area, centroid sums, and colour
// of one connected component during labelling.
type ccStats struct {
	minX, minY, maxX, maxY int
	area                   int
	sumX, sumY             int
	dark                   bool
}

func (s ccStats) cx() float64 { return float64(s.sumX) / float64(s.area) }
func (s ccStats) cy() float64 { return float64(s.sumY) / float64(s.area) }
func (s ccStats) bw() int     { return s.maxX - s.minX + 1 }
func (s ccStats) bh() int     { return s.maxY - s.minY + 1 }

// side is the mean bounding-box side length, a tilt-tolerant size estimate.
func (s ccStats) side() float64 { return (float64(s.bw()) + float64(s.bh())) / 2 }

// aspect is the bounding box's longer side over its shorter side (>= 1).
func (s ccStats) aspect() float64 {
	w, h := float64(s.bw()), float64(s.bh())
	if w < h {
		w, h = h, w
	}
	return w / h
}

// unionFind is a disjoint-set structure over provisional component labels,
// with path halving in find and union by smaller root.
type unionFind struct{ parent []int }

func (u *unionFind) find(x int) int {
	for u.parent[x] != x {
		u.parent[x] = u.parent[u.parent[x]]
		x = u.parent[x]
	}
	return x
}

func (u *unionFind) union(a, b int) {
	ra, rb := u.find(a), u.find(b)
	if ra == rb {
		return
	}
	if ra < rb {
		u.parent[rb] = ra
	} else {
		u.parent[ra] = rb
	}
}

// labelComponents performs two-pass connected-component labelling over the
// binarised bitmap with 4-connectivity, grouping maximal same-colour regions
// and returning one ccStats per component. See theory doc 24 section 3.
func labelComponents(bm *bitmap) []ccStats {
	w, h := bm.width, bm.height
	labels := make([]int, w*h)
	uf := unionFind{parent: []int{0}} // index 0 stands for "no label"
	next := 1

	// First pass: provisional labels from the up and left same-colour
	// neighbours, unioning when both exist and differ.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := bm.get(x, y)
			up, left := 0, 0
			if y > 0 && bm.get(x, y-1) == c {
				up = labels[(y-1)*w+x]
			}
			if x > 0 && bm.get(x-1, y) == c {
				left = labels[y*w+x-1]
			}
			switch {
			case up != 0 && left != 0:
				m := up
				if left < m {
					m = left
				}
				labels[y*w+x] = m
				uf.union(up, left)
			case up != 0:
				labels[y*w+x] = up
			case left != 0:
				labels[y*w+x] = left
			default:
				labels[y*w+x] = next
				uf.parent = append(uf.parent, next)
				next++
			}
		}
	}

	// Second pass: resolve each provisional label to its set root, compact the
	// roots into consecutive indices, and accumulate per-component stats.
	compact := make([]int, next) // root label -> stats index + 1 (0 = unseen)
	var stats []ccStats
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			root := uf.find(labels[y*w+x])
			si := compact[root] - 1
			if si < 0 {
				si = len(stats)
				compact[root] = si + 1
				stats = append(stats, ccStats{
					minX: x, maxX: x, minY: y, maxY: y,
					dark: bm.get(x, y),
				})
			}
			s := &stats[si]
			s.area++
			s.sumX += x
			s.sumY += y
			if x < s.minX {
				s.minX = x
			}
			if x > s.maxX {
				s.maxX = x
			}
			if y < s.minY {
				s.minY = y
			}
			if y > s.maxY {
				s.maxY = y
			}
		}
	}
	return stats
}

// nestedIn finds, among the components in pool, the one whose bounding box sits
// inside parent's box, whose centroid is concentric with parent's, and whose
// side is closest to wantRatio times parent's side. Returns its index in stats,
// or -1 if none qualifies.
func nestedIn(stats []ccStats, pool []int, parent ccStats, wantRatio float64) int {
	ps := parent.side()
	pcx, pcy := parent.cx(), parent.cy()
	tol := concentricFrac * ps
	best, bestErr := -1, math.Inf(1)
	for _, idx := range pool {
		c := stats[idx]
		// Bounding box must lie inside the parent's, allowing one pixel of
		// slack for discretisation at the borders.
		if c.minX < parent.minX-1 || c.minY < parent.minY-1 ||
			c.maxX > parent.maxX+1 || c.maxY > parent.maxY+1 {
			continue
		}
		if math.Hypot(c.cx()-pcx, c.cy()-pcy) > tol {
			continue
		}
		e := math.Abs(c.side()/ps-wantRatio) / wantRatio
		if e > nestSideTol {
			continue
		}
		if e < bestErr {
			best, bestErr = idx, e
		}
	}
	return best
}

// finderCandidatesCC returns the centres of finder-like nested triples found by
// connected-component analysis. Each accepted candidate is a hollow, roughly
// square dark ring enclosing a concentric light ring enclosing a concentric
// dark centre, with bounding-box sides in the finder's 7:5:3 proportion.
func finderCandidatesCC(bm *bitmap) []finderCandidate {
	stats := labelComponents(bm)
	var darks, lights []int
	for i := range stats {
		if stats[i].dark {
			darks = append(darks, i)
		} else {
			lights = append(lights, i)
		}
	}

	var out []finderCandidate
	for _, di := range darks {
		outer := stats[di]
		side := outer.side()
		if side < minFinderOuterPx || outer.aspect() > maxFinderAspect {
			continue
		}
		if float64(outer.area) > maxHollowFill*float64(outer.bw()*outer.bh()) {
			continue // solid blob, not a ring
		}
		li := nestedIn(stats, lights, outer, 5.0/7.0)
		if li < 0 {
			continue
		}
		light := stats[li]
		ci := nestedIn(stats, darks, light, 3.0/5.0)
		if ci < 0 {
			continue
		}
		centre := stats[ci]
		out = append(out, finderCandidate{
			x:          (outer.cx() + light.cx() + centre.cx()) / 3,
			y:          (outer.cy() + light.cy() + centre.cy()) / 3,
			moduleSize: side / 7.0,
		})
	}
	return dedupeCCCandidates(out)
}

// dedupeCCCandidates drops candidates whose centres coincide within half a
// module of an already-kept candidate.
func dedupeCCCandidates(cands []finderCandidate) []finderCandidate {
	var out []finderCandidate
	for _, c := range cands {
		dup := false
		for i := range out {
			if math.Hypot(out[i].x-c.x, out[i].y-c.y) < 0.5*(out[i].moduleSize+c.moduleSize) {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, c)
		}
	}
	return out
}

// findFindersCC locates the three finder patterns via connected-component
// analysis and orders them (top-left, top-right, bottom-left). It is the
// perspective-robust counterpart of findFinders, used only on the fallback
// path. Returns ErrFinderNotFound if fewer than three candidates are found.
func findFindersCC(bm *bitmap) (finderTriple, error) {
	cands := finderCandidatesCC(bm)
	if len(cands) < 3 {
		return finderTriple{}, ErrFinderNotFound
	}
	if len(cands) > maxCCCandidates {
		sort.Slice(cands, func(i, j int) bool {
			return cands[i].moduleSize > cands[j].moduleSize
		})
		cands = cands[:maxCCCandidates]
	}
	best, ok := bestFinderTriple(cands)
	if !ok {
		return finderTriple{}, ErrFinderNotFound
	}
	return orderFinderTriple(best[0], best[1], best[2])
}

// bestFinderTriple picks the triple of candidates that best fits a QR symbol's
// geometry: three finders sharing a module size whose centres form a right
// isosceles triangle. It minimises finderTripleCost over all triples.
func bestFinderTriple(c []finderCandidate) ([3]finderCandidate, bool) {
	n := len(c)
	best := [3]finderCandidate{}
	bestCost, ok := math.Inf(1), false
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			for k := j + 1; k < n; k++ {
				if cost := finderTripleCost(c[i], c[j], c[k]); cost < bestCost {
					bestCost, best, ok = cost, [3]finderCandidate{c[i], c[j], c[k]}, true
				}
			}
		}
	}
	return best, ok
}

// finderTripleCost scores how well three candidates fit the finder geometry;
// lower is better. It sums the module-size spread and two shape residuals: the
// centres should form a right angle (leg^2 + leg^2 = hypotenuse^2) with equal
// legs (a square symbol seen through any similarity transform).
func finderTripleCost(a, b, c finderCandidate) float64 {
	mMin := math.Min(a.moduleSize, math.Min(b.moduleSize, c.moduleSize))
	mMax := math.Max(a.moduleSize, math.Max(b.moduleSize, c.moduleSize))
	mMean := (a.moduleSize + b.moduleSize + c.moduleSize) / 3
	modSpread := (mMax - mMin) / mMean

	dab, dbc, dca := distSq(a, b), distSq(b, c), distSq(c, a)
	hyp := math.Max(dab, math.Max(dbc, dca))
	if hyp == 0 {
		return math.Inf(1)
	}
	legSum := dab + dbc + dca - hyp // the two smaller squared distances
	right := math.Abs(legSum-hyp) / hyp
	leg1, leg2 := twoSmallest(dab, dbc, dca)
	iso := math.Abs(leg1-leg2) / (leg1 + leg2)
	return modSpread + right + iso
}

func distSq(a, b finderCandidate) float64 {
	dx, dy := a.x-b.x, a.y-b.y
	return dx*dx + dy*dy
}

// twoSmallest returns the two smallest of three values.
func twoSmallest(x, y, z float64) (float64, float64) {
	s := []float64{x, y, z}
	sort.Float64s(s)
	return s[0], s[1]
}
