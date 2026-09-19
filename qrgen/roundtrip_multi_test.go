// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import (
	"testing"

	"github.com/makiuchi-d/gozxing"
	gozxingmulti "github.com/makiuchi-d/gozxing/multi/qrcode"
)

// TestDecodeAllMatchesGozxingMulti cross-checks DecodeAll against gozxing's
// QRCodeMultiReader on a composited three-symbol scene: an independent
// multi-symbol reader must recover the same payload set we do.
func TestDecodeAllMatchesGozxingMulti(t *testing.T) {
	a := renderQR(t, "ALPHA-1")
	b := renderQR(t, "BRAVO-2")
	c := renderQR(t, "CHARLIE-3")
	tw, th := a.Bounds().Dx(), a.Bounds().Dy()
	canvas := compositeTiles(3*tw+120, th+40,
		placedTile{a, 20, 20},
		placedTile{b, tw + 50, 20},
		placedTile{c, 2*tw + 80, 20})

	ours, err := DecodeAll(canvas)
	if err != nil {
		t.Fatalf("DecodeAll: %v", err)
	}
	ourSet := map[string]bool{}
	for _, r := range ours {
		ourSet[r.Text] = true
	}

	bmp, err := gozxing.NewBinaryBitmapFromImage(canvas)
	if err != nil {
		t.Fatalf("NewBinaryBitmapFromImage: %v", err)
	}
	gres, err := gozxingmulti.NewQRCodeMultiReader().DecodeMultipleWithoutHint(bmp)
	if err != nil {
		t.Fatalf("gozxing DecodeMultiple: %v", err)
	}
	gozSet := map[string]bool{}
	for _, r := range gres {
		gozSet[r.GetText()] = true
	}

	want := []string{"ALPHA-1", "BRAVO-2", "CHARLIE-3"}
	for _, w := range want {
		if !ourSet[w] {
			t.Errorf("DecodeAll missing %q (got %v)", w, resultTexts(ours))
		}
		if !gozSet[w] {
			t.Errorf("gozxing missing %q", w)
		}
	}
	if len(ourSet) != len(gozSet) {
		t.Errorf("payload-set size differs: ours=%d gozxing=%d", len(ourSet), len(gozSet))
	}
}
