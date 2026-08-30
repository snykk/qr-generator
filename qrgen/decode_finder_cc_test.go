package qrgen

import (
	"math"
	"testing"
)

// finderModules is the 7x7 finder pattern: a dark border ring, a light ring,
// and a solid 3x3 dark centre (1 = dark, 0 = light).
var finderModules = [7][7]int{
	{1, 1, 1, 1, 1, 1, 1},
	{1, 0, 0, 0, 0, 0, 1},
	{1, 0, 1, 1, 1, 0, 1},
	{1, 0, 1, 1, 1, 0, 1},
	{1, 0, 1, 1, 1, 0, 1},
	{1, 0, 0, 0, 0, 0, 1},
	{1, 1, 1, 1, 1, 1, 1},
}

func newBlankBitmap(w, h int) *bitmap {
	return &bitmap{width: w, height: h, pixels: make([]bool, w*h)}
}

// paintModules paints a 7x7 module grid at pixel origin (ox, oy) with each
// module scaled to s by s pixels; dark modules are set true.
func paintModules(bm *bitmap, ox, oy, s int, mod [7][7]int) {
	for my := 0; my < 7; my++ {
		for mx := 0; mx < 7; mx++ {
			dark := mod[my][mx] == 1
			for dy := 0; dy < s; dy++ {
				for dx := 0; dx < s; dx++ {
					px, py := ox+mx*s+dx, oy+my*s+dy
					if px >= 0 && py >= 0 && px < bm.width && py < bm.height {
						bm.pixels[py*bm.width+px] = dark
					}
				}
			}
		}
	}
}

func TestFinderCandidatesCCSingle(t *testing.T) {
	const s = 5
	bm := newBlankBitmap(100, 100)
	paintModules(bm, 10, 10, s, finderModules)

	cands := finderCandidatesCC(bm)
	if len(cands) != 1 {
		t.Fatalf("got %d candidates, want 1", len(cands))
	}
	// The finder centre is 3.5 modules from the origin on both axes.
	wantX, wantY := 10+3.5*s, 10+3.5*s
	if math.Abs(cands[0].x-wantX) > 1 || math.Abs(cands[0].y-wantY) > 1 {
		t.Errorf("centre = (%.2f, %.2f), want (%.2f, %.2f)", cands[0].x, cands[0].y, wantX, wantY)
	}
	if math.Abs(cands[0].moduleSize-s) > 1 {
		t.Errorf("moduleSize = %.2f, want ~%d", cands[0].moduleSize, s)
	}
}

func TestFinderCandidatesCCRejectsSolidSquare(t *testing.T) {
	bm := newBlankBitmap(80, 80)
	solid := [7][7]int{}
	for y := range solid {
		for x := range solid[y] {
			solid[y][x] = 1
		}
	}
	paintModules(bm, 10, 10, 5, solid)

	if cands := finderCandidatesCC(bm); len(cands) != 0 {
		t.Errorf("solid square produced %d candidates, want 0", len(cands))
	}
}

func TestFinderCandidatesCCRequiresDarkCentre(t *testing.T) {
	// A dark ring around a light interior with no dark centre must not match:
	// the nesting is dark -> light only, missing the innermost dark region.
	noCentre := [7][7]int{
		{1, 1, 1, 1, 1, 1, 1},
		{1, 0, 0, 0, 0, 0, 1},
		{1, 0, 0, 0, 0, 0, 1},
		{1, 0, 0, 0, 0, 0, 1},
		{1, 0, 0, 0, 0, 0, 1},
		{1, 0, 0, 0, 0, 0, 1},
		{1, 1, 1, 1, 1, 1, 1},
	}
	bm := newBlankBitmap(80, 80)
	paintModules(bm, 10, 10, 5, noCentre)

	if cands := finderCandidatesCC(bm); len(cands) != 0 {
		t.Errorf("ring without dark centre produced %d candidates, want 0", len(cands))
	}
}

func TestFindFindersCCThreeFinders(t *testing.T) {
	const s = 4
	span := 7 * s
	bm := newBlankBitmap(200, 200)
	// Right angle at the top-left finder: TR shares its row, BL shares its column.
	paintModules(bm, 20, 20, s, finderModules)  // top-left
	paintModules(bm, 140, 20, s, finderModules) // top-right
	paintModules(bm, 20, 140, s, finderModules) // bottom-left

	cands := finderCandidatesCC(bm)
	if len(cands) != 3 {
		t.Fatalf("got %d candidates, want 3", len(cands))
	}

	tri, err := findFindersCC(bm)
	if err != nil {
		t.Fatalf("findFindersCC: %v", err)
	}
	half := float64(span) / 2
	wantTLx, wantTLy := 20+half, 20+half
	if math.Abs(tri.topLeft.x-wantTLx) > 2 || math.Abs(tri.topLeft.y-wantTLy) > 2 {
		t.Errorf("topLeft = (%.1f, %.1f), want (%.1f, %.1f)", tri.topLeft.x, tri.topLeft.y, wantTLx, wantTLy)
	}
	// top-right shares the top-left row; bottom-left shares its column.
	if math.Abs(tri.topRight.y-tri.topLeft.y) > 3 {
		t.Errorf("topRight.y=%.1f not aligned with topLeft.y=%.1f", tri.topRight.y, tri.topLeft.y)
	}
	if math.Abs(tri.bottomLeft.x-tri.topLeft.x) > 3 {
		t.Errorf("bottomLeft.x=%.1f not aligned with topLeft.x=%.1f", tri.bottomLeft.x, tri.topLeft.x)
	}
}

func TestFindFindersCCTooFew(t *testing.T) {
	bm := newBlankBitmap(80, 80)
	paintModules(bm, 10, 10, 5, finderModules) // only one finder
	if _, err := findFindersCC(bm); err == nil {
		t.Error("expected ErrFinderNotFound with a single finder, got nil")
	}
}
