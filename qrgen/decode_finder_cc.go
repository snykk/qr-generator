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
// binarised bitmap with 4-connectivity, grouping maximal same-colour regions.
// It returns one ccStats per component and labelOf, a per-pixel slice giving
// each pixel's component index into that stats slice (used to collect a
// finder ring's pixels for corner extraction). See theory doc 24 section 3.
func labelComponents(bm *bitmap) (stats []ccStats, labelOf []int) {
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
	// roots into consecutive indices, record each pixel's component index, and
	// accumulate per-component stats.
	labelOf = make([]int, w*h)
	compact := make([]int, next) // root label -> stats index + 1 (0 = unseen)
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
			labelOf[y*w+x] = si
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
	return stats, labelOf
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

// ccCandidate is a matched finder nesting: its centre estimate plus the stats
// index of the outer dark ring, kept so corner extraction can revisit the
// ring's pixels.
type ccCandidate struct {
	center finderCandidate
	outer  int
}

// finderNestings scans the labelled components for finder-like nested triples
// (a hollow, roughly square dark ring enclosing a concentric light ring
// enclosing a concentric dark centre, with bounding-box sides in the 7:5:3
// proportion) and returns one ccCandidate per match, deduplicated by centre.
func finderNestings(stats []ccStats) []ccCandidate {
	var darks, lights []int
	for i := range stats {
		if stats[i].dark {
			darks = append(darks, i)
		} else {
			lights = append(lights, i)
		}
	}

	var out []ccCandidate
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
		out = append(out, ccCandidate{
			center: finderCandidate{
				x:          (outer.cx() + light.cx() + centre.cx()) / 3,
				y:          (outer.cy() + light.cy() + centre.cy()) / 3,
				moduleSize: side / 7.0,
			},
			outer: di,
		})
	}
	return dedupeCCNestings(out)
}

// finderCandidatesCC returns the centres of finder-like nested triples found by
// connected-component analysis, discarding the outer-ring provenance.
func finderCandidatesCC(bm *bitmap) []finderCandidate {
	stats, _ := labelComponents(bm)
	nests := finderNestings(stats)
	out := make([]finderCandidate, len(nests))
	for i, c := range nests {
		out[i] = c.center
	}
	return out
}

// dedupeCCNestings drops matches whose centres coincide within half a module of
// an already-kept match.
func dedupeCCNestings(cands []ccCandidate) []ccCandidate {
	var out []ccCandidate
	for _, c := range cands {
		dup := false
		for i := range out {
			if math.Hypot(out[i].center.x-c.center.x, out[i].center.y-c.center.y) <
				0.5*(out[i].center.moduleSize+c.center.moduleSize) {
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
	idx, ok := bestFinderTripleIdx(c)
	if !ok {
		return [3]finderCandidate{}, false
	}
	return [3]finderCandidate{c[idx[0]], c[idx[1]], c[idx[2]]}, true
}

// bestFinderTripleIdx returns the indices of the lowest-cost triple, so callers
// that carry extra per-candidate data (finder quads) can recover it.
func bestFinderTripleIdx(c []finderCandidate) ([3]int, bool) {
	n := len(c)
	var best [3]int
	bestCost, ok := math.Inf(1), false
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			for k := j + 1; k < n; k++ {
				if cost := finderTripleCost(c[i], c[j], c[k]); cost < bestCost {
					bestCost, best, ok = cost, [3]int{i, j, k}, true
				}
			}
		}
	}
	return best, ok
}

// orderedTriple is a geometrically valid finder triple expressed as indices
// into a candidate slice, in reading order (top-left, top-right, bottom-left),
// carrying its finderTripleCost. Indices, not centre values, so the caller maps
// back to finder quads without a float-equality lookup.
type orderedTriple struct {
	tl, tr, bl int
	cost       float64
}

// groupFinderTriples is the multi-symbol generalisation of bestFinderTripleIdx:
// instead of the single lowest-cost triple it returns every geometrically valid
// triple among the candidates, ordered (top-left, top-right, bottom-left) and
// sorted by ascending cost. Validity is orderFinderTriple's existing gate (equal
// legs, right angle); there is deliberately no absolute cost ceiling, because
// measuring the tilt corpus showed genuinely decodable symbols reaching
// finderTripleCost around 0.76 at 55 degrees with no clean separation from
// noise, so a fixed gate would drop real steep-tilt decodes and would also
// regress the single-symbol path (which has no ceiling). Sorting by cost puts
// real symbols (cost near zero) first; the caller claims disjoint triples and
// its attempt cap bounds the rest.
func groupFinderTriples(centres []finderCandidate) []orderedTriple {
	n := len(centres)
	var out []orderedTriple
	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			for k := j + 1; k < n; k++ {
				tri, err := orderFinderTriple(centres[i], centres[j], centres[k])
				if err != nil {
					continue
				}
				ids := tripleIndices(centres, i, j, k, tri)
				out = append(out, orderedTriple{
					tl:   ids[0],
					tr:   ids[1],
					bl:   ids[2],
					cost: finderTripleCost(centres[i], centres[j], centres[k]),
				})
			}
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].cost < out[b].cost })
	return out
}

// tripleIndices maps an ordered finder triple back to indices among {i, j, k}.
// orderFinderTriple only permutes its three inputs, so each ordered centre
// equals exactly one input by value; dedupeCCNestings guarantees the candidate
// centres are distinct, so the value match is unambiguous.
func tripleIndices(centres []finderCandidate, i, j, k int, tri finderTriple) [3]int {
	idxOf := func(c finderCandidate) int {
		for _, x := range [3]int{i, j, k} {
			if centres[x] == c {
				return x
			}
		}
		return i // unreachable: tri's centres are a permutation of {i, j, k}
	}
	return [3]int{idxOf(tri.topLeft), idxOf(tri.topRight), idxOf(tri.bottomLeft)}
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

// finderQuad is a detected finder pattern with its centre estimate and the four
// outer corners of its dark ring, in image-pixel coordinates and arbitrary
// order. The corners are the correspondences a projective homography needs to
// undo perspective, which the finder centres alone (an affine map) cannot.
type finderQuad struct {
	center  finderCandidate
	corners [4][2]float64
}

// finderTripleQuad groups three finder quads in reading order (top-left,
// top-right, bottom-left), the quad counterpart of finderTriple.
type finderTripleQuad struct {
	tl, tr, bl finderQuad
}

// allFinderQuads locates finder patterns by connected-component analysis and
// returns each as a quad: its centre plus the four outer corners of its dark
// ring. Corners come from the ring's own pixels, so they carry the perspective
// distortion the projective homography then inverts. There is no minimum
// count: a Micro symbol has a single finder (docs/theory/27-micro-qr-detection.md).
func allFinderQuads(bm *bitmap) []finderQuad {
	return scanComponents(bm).quads
}

// ccScan is one connected-component labelling of a bitmap and the finder quads
// found in it, kept together so the rMQR search can look for finder
// sub-patterns in the same labelling without labelling the bitmap again.
type ccScan struct {
	stats   []ccStats
	labelOf []int
	width   int
	quads   []finderQuad

	subs     []subPattern // the sub-pattern candidates, found on first use
	subsDone bool
}

// scanComponents labels bm once and extracts its finder quads.
func scanComponents(bm *bitmap) *ccScan {
	stats, labelOf := labelComponents(bm)
	nests := finderNestings(stats)
	quads := make([]finderQuad, 0, len(nests))
	for _, c := range nests {
		corners := extractQuadCorners(labelOf, bm.width, stats[c.outer], c.outer)
		quads = append(quads, finderQuad{center: c.center, corners: corners})
	}
	return &ccScan{stats: stats, labelOf: labelOf, width: bm.width, quads: quads}
}

// subPattern is a candidate rMQR finder sub-pattern, or anything of the same
// shape, such as a QR alignment pattern: a closed light ring around a
// one-module dark centre. Only the light ring is used, because the dark ring
// around it has no separator and merges with the modules beside it
// (docs/theory/30-rmqr-detection.md, section 4).
type subPattern struct {
	x, y       float64 // centre, in pixels
	moduleSize float64 // a third of the light ring's side
	light      int     // stats index of the light ring
}

// Sub-pattern shape limits. The light ring is 3 modules across around a
// 1-module centre, so the centre's side is a third of the ring's; the
// tolerances follow the finder nesting's (nestSideTol, concentricFrac).
const (
	minSubRingPx = 3.0 // a light ring under 1 pixel per module is unresolvable
	maxSubAspect = 3.0 // as maxFinderAspect: tilt stretches the bounding box
)

// subPatterns returns the sub-pattern candidates of the scan, finding them on
// the first call. Each small dark component is a possible centre; the light
// component enclosing it is the one holding the pixel just above its bounding
// box, so the search is linear in the number of components rather than a
// comparison of every pair.
func (s *ccScan) subPatterns() []subPattern {
	if s.subsDone {
		return s.subs
	}
	s.subsDone = true
	for ci, c := range s.stats {
		if !c.dark || c.minY == 0 {
			continue
		}
		x := int(math.Round(c.cx()))
		if x < c.minX || x > c.maxX {
			continue
		}
		li := s.labelOf[(c.minY-1)*s.width+x]
		l := s.stats[li]
		if l.dark || l.side() < minSubRingPx || l.aspect() > maxSubAspect || li == ci {
			continue
		}
		if l.minX == 0 || l.minY == 0 || l.maxX == s.width-1 || l.maxY == len(s.labelOf)/s.width-1 {
			continue // a ring is enclosed; this light region reaches the image edge
		}
		if c.minX < l.minX || c.minY < l.minY || c.maxX > l.maxX || c.maxY > l.maxY {
			continue // the light component does not enclose the centre
		}
		if e := math.Abs(c.side()/l.side()-1.0/3.0) * 3; e > nestSideTol {
			continue
		}
		if math.Hypot(c.cx()-l.cx(), c.cy()-l.cy()) > concentricFrac*l.side() {
			continue
		}
		s.subs = append(s.subs, subPattern{
			x:          (c.cx() + l.cx()) / 2,
			y:          (c.cy() + l.cy()) / 2,
			moduleSize: l.side() / 3,
			light:      li,
		})
	}
	return s.subs
}

// ringCorners returns the outer corners of the sub-pattern's light ring, the
// boundary between it and the dark ring, wound as windCorners winds them. As
// in outerRingCorners, each extreme pixel's centre is moved half a pixel
// outward to the pixel's own boundary.
func (s *ccScan) ringCorners(sp subPattern) [4][2]float64 {
	l := s.stats[sp.light]
	c := extractQuadCorners(s.labelOf, s.width, l, sp.light)
	for i, p := range c {
		dx, dy := p[0]-l.cx(), p[1]-l.cy()
		if m := math.Max(math.Abs(dx), math.Abs(dy)); m > 0 {
			c[i] = [2]float64{p[0] + 0.5*dx/m, p[1] + 0.5*dy/m}
		}
	}
	return windCorners(c)
}

// detectFinderQuads is allFinderQuads for the QR paths, which need at least
// three finders and return ErrFinderNotFound otherwise.
func detectFinderQuads(bm *bitmap) ([]finderQuad, error) {
	quads := allFinderQuads(bm)
	if len(quads) < 3 {
		return nil, ErrFinderNotFound
	}
	return quads, nil
}

// bestFinderQuadTriple selects, among detected finder quads, the three whose
// centres best fit a square symbol's right-isosceles geometry, and orders them
// (top-left, top-right, bottom-left) via the rotation-invariant finder ordering
// while keeping each quad's corners. It returns ErrFinderNotFound for fewer
// than three quads, and leaves the caller's slice as it was, so the Micro pass
// can reuse the same quads.
func bestFinderQuadTriple(all []finderQuad) (finderTripleQuad, error) {
	if len(all) < 3 {
		return finderTripleQuad{}, ErrFinderNotFound
	}
	quads := all
	if len(quads) > maxCCCandidates {
		quads = append([]finderQuad(nil), all...)
		sort.Slice(quads, func(i, j int) bool {
			return quads[i].center.moduleSize > quads[j].center.moduleSize
		})
		quads = quads[:maxCCCandidates]
	}
	centres := make([]finderCandidate, len(quads))
	for i := range quads {
		centres[i] = quads[i].center
	}
	idx, ok := bestFinderTripleIdx(centres)
	if !ok {
		return finderTripleQuad{}, ErrFinderNotFound
	}
	tri, err := orderFinderTriple(quads[idx[0]].center, quads[idx[1]].center, quads[idx[2]].center)
	if err != nil {
		return finderTripleQuad{}, err
	}
	pick := func(c finderCandidate) finderQuad {
		for _, q := range quads {
			if q.center.x == c.x && q.center.y == c.y {
				return q
			}
		}
		return finderQuad{center: c}
	}
	return finderTripleQuad{tl: pick(tri.topLeft), tr: pick(tri.topRight), bl: pick(tri.bottomLeft)}, nil
}

// extractQuadCorners collects the pixels of the outer-ring component and returns
// its four corner points. Corners are the extreme vertices of the (convex)
// ring: the point farthest from the centroid, the point farthest from that, and
// the two points farthest to either side of the line joining them. This is
// rotation-agnostic, unlike axis-aligned min/max-of-(x±y).
func extractQuadCorners(labelOf []int, width int, outer ccStats, outerIdx int) [4][2]float64 {
	var pts [][2]float64
	for y := outer.minY; y <= outer.maxY; y++ {
		base := y * width
		for x := outer.minX; x <= outer.maxX; x++ {
			if labelOf[base+x] == outerIdx {
				pts = append(pts, [2]float64{float64(x), float64(y)})
			}
		}
	}
	if len(pts) == 0 {
		c := [2]float64{outer.cx(), outer.cy()}
		return [4][2]float64{c, c, c, c}
	}
	p0 := farthestFrom(pts, outer.cx(), outer.cy())
	p2 := farthestFrom(pts, p0[0], p0[1])
	p1, p3 := farthestAcross(pts, p0, p2)
	return [4][2]float64{p0, p1, p2, p3}
}

// farthestFrom returns the point in pts with the greatest distance from (cx,cy).
func farthestFrom(pts [][2]float64, cx, cy float64) [2]float64 {
	best, bestD := pts[0], math.Inf(-1)
	for _, p := range pts {
		if d := (p[0]-cx)*(p[0]-cx) + (p[1]-cy)*(p[1]-cy); d > bestD {
			best, bestD = p, d
		}
	}
	return best
}

// farthestAcross returns the two points lying farthest to either side of the
// directed line a->b, measured by signed perpendicular distance.
func farthestAcross(pts [][2]float64, a, b [2]float64) ([2]float64, [2]float64) {
	dx, dy := b[0]-a[0], b[1]-a[1]
	posP, negP := a, a
	maxPos, maxNeg := 0.0, 0.0
	for _, p := range pts {
		// Cross product of (b-a) with (p-a): positive on one side, negative on
		// the other. Magnitude is proportional to perpendicular distance.
		s := dx*(p[1]-a[1]) - dy*(p[0]-a[0])
		if s > maxPos {
			maxPos, posP = s, p
		} else if s < maxNeg {
			maxNeg, negP = s, p
		}
	}
	return posP, negP
}
