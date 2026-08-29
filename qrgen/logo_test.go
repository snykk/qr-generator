// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"testing"
)

// solidLogo returns a size-by-size opaque square of colour c, a stand-in logo.
func solidLogo(size int, c color.Color) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(img, img.Bounds(), image.NewUniform(c), image.Point{}, draw.Src)
	return img
}

// A logo forces at least EC-H from any lower level, independent of option order,
// and leaves the level alone when there is no logo.
func TestLogoAutoECBump(t *testing.T) {
	logo := Logo{Image: solidLogo(64, color.RGBA{0, 0, 0xFF, 0xFF})}
	cases := []struct {
		name string
		opts []Option
	}{
		{"default M", []Option{WithLogo(logo)}},
		{"explicit L before logo", []Option{WithECLevel(ECLevelL), WithLogo(logo)}},
		{"explicit L after logo", []Option{WithLogo(logo), WithECLevel(ECLevelL)}},
		{"explicit H", []Option{WithLogo(logo), WithECLevel(ECLevelH)}},
	}
	for _, c := range cases {
		if got := resolveOptions(c.opts...).ec; got != ECLevelH {
			t.Errorf("%s: ec = %v, want H", c.name, got)
		}
	}
	if got := resolveOptions(WithECLevel(ECLevelL)).ec; got != ECLevelL {
		t.Errorf("no logo: ec = %v, want L (untouched)", got)
	}
}

// The box-filter resampler hits the target size and preserves a solid colour on
// both downscale and upscale.
func TestResample(t *testing.T) {
	src := solidLogo(40, color.RGBA{0x20, 0x40, 0x80, 0xFF})
	for _, sz := range []int{10, 40, 100} {
		dst := resample(src, sz, sz)
		if b := dst.Bounds(); b.Dx() != sz || b.Dy() != sz {
			t.Errorf("resample to %d: got %dx%d", sz, b.Dx(), b.Dy())
		}
		r, g, b, a := dst.At(sz/2, sz/2).RGBA()
		if r>>8 != 0x20 || g>>8 != 0x40 || b>>8 != 0x80 || a>>8 != 0xFF {
			t.Errorf("resample to %d: centre = (%x %x %x %x), want (20 40 80 FF)", sz, r>>8, g>>8, b>>8, a>>8)
		}
	}
}

// A symbol with a default-size logo still decodes: the occluded modules become
// Reed-Solomon errors that EC-H recovers.
func TestLogoRoundTrip(t *testing.T) {
	logo := solidLogo(80, color.RGBA{0xC0, 0x10, 0x10, 0xFF})
	cases := []string{
		"HELLO WORLD",
		"https://github.com/snykk/qr-generator",
		"Order #1234567890 with a longer byte payload to spread across blocks",
	}
	for _, text := range cases {
		data, err := Encode(text, WithLogo(Logo{Image: logo}))
		if err != nil {
			t.Fatalf("Encode(%q, logo): %v", text, err)
		}
		got, err := DecodeBytes(data)
		if err != nil {
			t.Fatalf("DecodeBytes(%q, logo): %v", text, err)
		}
		if got != text {
			t.Errorf("logo round-trip: got %q, want %q", got, text)
		}
	}
}

// The logo changes the rendered bytes (and the no-logo output is a plain PNG
// that still decodes), guarding that logo handling is gated on the option.
func TestLogoChangesOutput(t *testing.T) {
	const text = "HELLO WORLD"
	plain, err := Encode(text)
	if err != nil {
		t.Fatal(err)
	}
	withLogo, err := Encode(text, WithLogo(Logo{Image: solidLogo(64, color.Black)}))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(plain, withLogo) {
		t.Error("logo did not change the output")
	}
	if got, err := DecodeBytes(plain); err != nil || got != text {
		t.Errorf("plain round-trip: got %q, err %v", got, err)
	}
}
