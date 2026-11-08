// Copyright 2026 Najib Fikri
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0

package qrgen

import (
	"fmt"
	"strconv"
	"unicode/utf8"
)

// ModuleRole says what a module of a symbol is for: part of a function pattern
// that a reader uses to find, orient, and size the symbol, part of the format
// or version information, or a bit of the encoded message.
type ModuleRole uint8

// The module roles reported by [Inspect]. RoleData is the zero value.
const (
	RoleData             ModuleRole = iota // a bit of a data or error-correction codeword
	RoleRemainder                          // a remainder bit after the last codeword, light before masking
	RoleFinder                             // a 7 by 7 finder pattern
	RoleSeparator                          // the light border between a finder and the data
	RoleTiming                             // a timing pattern of alternating modules
	RoleAlignment                          // an alignment pattern
	RoleFormat                             // format information: error-correction level and mask
	RoleVersion                            // version information, in QR versions 7 and up
	RoleDarkModule                         // the single always-dark module of a QR symbol
	RoleFinderSubPattern                   // the 5 by 5 finder sub-pattern of an rMQR symbol
	RoleCornerPattern                      // a corner pattern of an rMQR symbol
)

var moduleRoleNames = [...]string{
	RoleData:             "data",
	RoleRemainder:        "remainder",
	RoleFinder:           "finder",
	RoleSeparator:        "separator",
	RoleTiming:           "timing",
	RoleAlignment:        "alignment",
	RoleFormat:           "format",
	RoleVersion:          "version",
	RoleDarkModule:       "dark module",
	RoleFinderSubPattern: "finder sub-pattern",
	RoleCornerPattern:    "corner pattern",
}

// String returns a short lower-case name such as "finder" or "dark module".
func (r ModuleRole) String() string {
	if int(r) < len(moduleRoleNames) {
		return moduleRoleNames[r]
	}
	return fmt.Sprintf("ModuleRole(%d)", int(r))
}

// IsFunction reports whether r is a function module: anything other than a
// data or remainder bit. Masking never touches function modules.
func (r ModuleRole) IsFunction() bool { return r != RoleData && r != RoleRemainder }

// Inspection is a symbol taken apart: the segments the text was split into,
// the codewords and blocks, and, for every module, its role, whether it was
// dark before the mask, and which codeword bit it carries. [Inspect] returns
// it; it exists for study and visualisation, not for encoding.
type Inspection struct {
	// Symbology is "QR Code", "Micro QR Code", or "rMQR Code".
	Symbology string
	// Version names the symbol: "1" to "40", "M1" to "M4", or "R7x43" to
	// "R17x139".
	Version string
	// ECLevel is the error-correction level. Micro QR M1 has none and only
	// detects errors; DetectionOnly reports that case, and ECLevel is then
	// meaningless.
	ECLevel       ECLevel
	DetectionOnly bool

	// Mask is the mask applied: 0..7 for QR and 0..3 for Micro QR, in each
	// symbology's own numbering. rMQR has a single fixed mask, which is QR's
	// mask 4, and reports 4.
	Mask int
	// MaskScores scores every candidate mask with the text and level
	// unchanged: for QR the penalty of ISO/IEC 18004 (lower wins) and for
	// Micro QR the edge score (higher wins). It is computed even when the mask
	// was forced, and is nil for rMQR, which has nothing to choose.
	MaskScores []int
	// MaskPenalties splits each QR penalty into its four rules, N1 to N4. It
	// is nil for Micro QR and rMQR.
	MaskPenalties [][4]int

	// Segments are the parts of the encoded message in order: an ECI header
	// if one was declared, then one segment per run of a single mode.
	Segments []InspectSegment
	// DataBits is the length of the segments together, in bits.
	DataBits int
	// TerminatorBits is the length of the terminator after them, shortened
	// when the capacity runs out first. Zero bits up to the next codeword
	// boundary and the pad codewords fill the rest of the capacity.
	TerminatorBits int
	// CapacityBits is the symbol's data capacity in bits.
	CapacityBits int

	// Blocks are the Reed-Solomon blocks: each block's data codewords, in
	// message order, and the error-correction codewords computed over them.
	Blocks []InspectBlock
	// Codewords is the final codeword stream in placement order: the blocks
	// interleaved, data first. For Micro QR M1 and M3 the last data codeword
	// is 4 bits long and sits in the high nibble of its byte.
	Codewords []byte

	// Modules is the finished symbol, as Matrix returns it, rows top to
	// bottom: true is a dark module.
	Modules [][]bool
	// Unmasked is the symbol before the mask was applied to its data and
	// remainder modules; function modules are as in Modules.
	Unmasked [][]bool
	// MaskPattern marks every module the mask formula selects. Only data and
	// remainder modules are actually inverted.
	MaskPattern [][]bool
	// Roles gives each module's role.
	Roles [][]ModuleRole
	// ModuleCodeword is, for each data module, the index in Codewords of the
	// codeword it carries a bit of, and -1 for every other module.
	ModuleCodeword [][]int
	// ModuleBit is, for each data module, the bit of that codeword it
	// carries, 7 for the most significant, and -1 for every other module.
	ModuleBit [][]int
}

// InspectSegment is one part of the encoded message.
type InspectSegment struct {
	// Mode is the segment's mode. An ECI header has ModeECI.
	Mode Mode
	// Text is the run of text the segment encodes; empty for an ECI header.
	Text string
	// Count is the value of the character-count field: characters for
	// numeric and alphanumeric, bytes for byte mode (after any ECI
	// transcoding), and characters for Kanji. For an ECI header it is the ECI
	// assignment number.
	Count int
	// IndicatorBits, CountBits, and PayloadBits are the widths of the mode
	// indicator, the character-count field, and the encoded characters (for
	// an ECI header, the designator).
	IndicatorBits int
	CountBits     int
	PayloadBits   int
}

// Bits is the segment's total length.
func (s InspectSegment) Bits() int { return s.IndicatorBits + s.CountBits + s.PayloadBits }

// InspectBlock is one Reed-Solomon block.
type InspectBlock struct {
	Data []byte // data codewords
	EC   []byte // error-correction codewords
}

// matrixTrace is what a traced matrix records: the role of every module and,
// for every data module, the index of the stream bit placed in it (-1 for
// function modules).
type matrixTrace struct {
	roles [][]ModuleRole
	bits  [][]int
}

// startTrace turns on tracing for an empty matrix, before any pattern is drawn.
func (m *matrix) startTrace() {
	t := &matrixTrace{roles: make([][]ModuleRole, m.rows()), bits: make([][]int, m.rows())}
	for r := range t.roles {
		t.roles[r] = make([]ModuleRole, m.cols())
		t.bits[r] = make([]int, m.cols())
		for c := range t.bits[r] {
			t.bits[r][c] = -1
		}
	}
	m.trace = t
}

// Inspect encodes text exactly as [Matrix] does, with the same options and
// the same choices of symbol, level, and mask, and reports how the symbol was
// built: the segments, the codewords and blocks, the mask candidates, and the
// role and codeword bit of every module. Modules equals what Matrix returns
// for the same arguments. Rendering options have no effect.
//
// Inspect is for learning and visualisation; it does more work than Matrix,
// scoring every mask even when one is forced. See docs/theory/05-matrix-construction.md
// and docs/theory/06-masking.md for what the layers mean.
func Inspect(text string, opts ...Option) (*Inspection, error) {
	o := resolveOptions(opts...)
	if err := o.validate(); err != nil {
		return nil, err
	}
	o.trace = true
	m, mask, err := buildMatrix(text, o)
	if err != nil {
		return nil, err
	}

	var (
		in     *Inspection
		qrMask int                       // the QR mask formula behind the applied mask
		locate func(bit int) (cw, b int) // stream bit to codeword and bit; cw -1 for remainder
		score  func(unmasked *matrix)    // fills the mask scores; nil for rMQR
	)
	switch {
	case o.rmqr:
		in, err = inspectRMQR(text, o)
		qrMask, locate = rmqrMask, byteLocator(len(in.Codewords))
	case o.micro:
		var sym microSymbol
		in, sym, err = inspectMicro(text, o)
		qrMask, locate = microMaskToQR[mask], microLocator(sym)
		score = func(u *matrix) { in.MaskScores = microMaskScores(u) }
	default:
		in, err = inspectQR(text, o, m.version)
		qrMask, locate = mask, byteLocator(len(in.Codewords))
		score = func(u *matrix) { in.MaskScores, in.MaskPenalties = qrMaskScores(u, o.ec) }
	}
	if err != nil {
		return nil, err
	}
	in.Mask = mask

	rows, cols := m.rows(), m.cols()
	unmasked := m.clone()
	unmasked.applyMask(qrMask)
	if score != nil {
		score(unmasked)
	}

	in.Modules = make([][]bool, rows)
	in.Unmasked = make([][]bool, rows)
	in.MaskPattern = make([][]bool, rows)
	in.Roles = m.trace.roles
	in.ModuleCodeword = make([][]int, rows)
	in.ModuleBit = make([][]int, rows)
	for r := range rows {
		in.Modules[r] = append([]bool(nil), m.modules[r]...)
		in.Unmasked[r] = append([]bool(nil), unmasked.modules[r]...)
		in.MaskPattern[r] = make([]bool, cols)
		in.ModuleCodeword[r] = make([]int, cols)
		in.ModuleBit[r] = make([]int, cols)
		for c := range cols {
			in.MaskPattern[r][c] = maskCondition(qrMask, r, c)
			in.ModuleCodeword[r][c], in.ModuleBit[r][c] = -1, -1
			if m.reserved[r][c] {
				continue
			}
			cw, b := locate(m.trace.bits[r][c])
			if cw < 0 {
				in.Roles[r][c] = RoleRemainder
				continue
			}
			in.ModuleCodeword[r][c], in.ModuleBit[r][c] = cw, b
		}
	}
	return in, nil
}

// inspectQR rebuilds the message of a QR symbol of version v: the version is
// already chosen, so forcing it reproduces the encoder's segments and
// codewords exactly.
func inspectQR(text string, o *options, v Version) (*Inspection, error) {
	data, _, err := encodeTextECI(text, o.ec, v, o.eci)
	if err != nil {
		return nil, err
	}
	in := &Inspection{Symbology: "QR Code", Version: strconv.Itoa(int(v)), ECLevel: o.ec, CapacityBits: 8 * v.DataCodewords(o.ec)}
	if o.eci != ECINone {
		in.Segments = append(in.Segments, eciSegment(o.eci, 4))
	}
	for _, s := range segmentText(text, v) {
		seg, err := inspectSegment(s, o.eci, 4, s.mode.CharCountBits(v))
		if err != nil {
			return nil, err
		}
		in.Segments = append(in.Segments, seg)
	}
	in.finishSegments(4)
	dataBlocks, ecBlocks := splitAndEncodeBlocks(data, v, o.ec)
	in.setBlocks(dataBlocks, ecBlocks)
	in.Codewords = interleaveBlocks(dataBlocks, ecBlocks)
	return in, nil
}

// inspectMicro rebuilds the message of a Micro QR symbol, choosing the symbol
// type as the encoder does.
func inspectMicro(text string, o *options) (*Inspection, microSymbol, error) {
	sym, _, err := encodeMicroText(text, o.microVersion, o.ec, o.ecSet)
	if err != nil {
		return nil, microSymbol{}, err
	}
	v := sym.version
	in := &Inspection{Symbology: "Micro QR Code", Version: v.String(), ECLevel: sym.ec, DetectionOnly: sym.detectOnly, CapacityBits: sym.dataBits}
	segs, _, _ := segmentWith(text, microSegmentCost(v))
	for _, s := range segs {
		seg, err := inspectSegment(s, ECINone, v.modeIndicatorBits(), microCountBits(s.mode, v))
		if err != nil {
			return nil, microSymbol{}, err
		}
		in.Segments = append(in.Segments, seg)
	}
	in.finishSegments(v.terminatorBits())
	words := microData(segs, sym).bytes()
	ecWords := encodeBlock(words, sym.ecWords)
	in.setBlocks([][]byte{words}, [][]byte{ecWords})
	in.Codewords = append(append([]byte(nil), words...), ecWords...)
	return in, sym, nil
}

// inspectRMQR rebuilds the message of an rMQR symbol, choosing the version
// and level as the encoder does.
func inspectRMQR(text string, o *options) (*Inspection, error) {
	sym, stream, err := encodeRMQRText(text, o.rmqrVersion, o.rmqrHeight, o.ec, o.ecSet, o.eci)
	if err != nil {
		return nil, err
	}
	v := sym.version
	in := &Inspection{Symbology: "rMQR Code", Version: v.String(), ECLevel: sym.ecLevel(), CapacityBits: 8 * sym.dataWords()}
	if o.eci != ECINone {
		in.Segments = append(in.Segments, eciSegment(o.eci, 3))
	}
	segs, _, _ := segmentWith(text, rmqrSegmentCost(v))
	for _, s := range segs {
		seg, err := inspectSegment(s, o.eci, 3, rmqrCountBits(s.mode, v))
		if err != nil {
			return nil, err
		}
		in.Segments = append(in.Segments, seg)
	}
	in.finishSegments(3)
	data, err := rmqrDataCodewords(segs, sym, o.eci)
	if err != nil {
		return nil, err
	}
	in.setBlocks(rmqrBlocks(data, sym))
	in.Codewords = stream
	return in, nil
}

// eciSegment describes an ECI header whose mode indicator is indicatorBits
// wide.
func eciSegment(eci ECI, indicatorBits int) InspectSegment {
	return InspectSegment{Mode: ModeECI, Count: int(eci), IndicatorBits: indicatorBits, PayloadBits: eci.designatorBits()}
}

// inspectSegment describes segment s with the given field widths. Under an
// ECI a byte segment is counted and sized in transcoded bytes, as written.
func inspectSegment(s segment, eci ECI, indicatorBits, countBits int) (InspectSegment, error) {
	count, payload := len(s.text), payloadBitLength(s.mode, s.text)
	switch {
	case s.mode == ModeKanji:
		count = utf8.RuneCountInString(s.text)
	case s.mode == ModeByte && eci != ECINone:
		b, err := transcodeTo(s.text, eci)
		if err != nil {
			return InspectSegment{}, err
		}
		count, payload = len(b), 8*len(b)
	}
	return InspectSegment{Mode: s.mode, Text: s.text, Count: count, IndicatorBits: indicatorBits, CountBits: countBits, PayloadBits: payload}, nil
}

// finishSegments totals the segments and sizes the terminator, which is
// terminatorBits long unless the capacity runs out first.
func (in *Inspection) finishSegments(terminatorBits int) {
	for _, s := range in.Segments {
		in.DataBits += s.Bits()
	}
	in.TerminatorBits = min(terminatorBits, in.CapacityBits-in.DataBits)
}

// setBlocks copies the Reed-Solomon blocks into in.
func (in *Inspection) setBlocks(dataBlocks, ecBlocks [][]byte) {
	in.Blocks = make([]InspectBlock, len(dataBlocks))
	for i := range dataBlocks {
		in.Blocks[i] = InspectBlock{Data: append([]byte(nil), dataBlocks[i]...), EC: append([]byte(nil), ecBlocks[i]...)}
	}
}

// byteLocator maps a stream bit to its codeword and bit for a stream of n
// whole codewords; bits past them are remainder bits.
func byteLocator(n int) func(int) (int, int) {
	return func(bit int) (int, int) {
		if bit < 0 || bit >= 8*n {
			return -1, -1
		}
		return bit / 8, 7 - bit%8
	}
}

// microLocator maps a stream bit of Micro symbol sym to its codeword and bit.
// In M1 and M3 the last data codeword is 4 bits long, so the codewords after
// it start 4 bits earlier in the stream than their index suggests.
func microLocator(sym microSymbol) func(int) (int, int) {
	if !sym.hasHalfCodeword() {
		return byteLocator(sym.dataWords + sym.ecWords)
	}
	full := sym.dataWords - 1 // whole data codewords before the half one
	return func(bit int) (int, int) {
		switch {
		case bit < 0 || bit >= sym.streamBits():
			return -1, -1
		case bit < 8*full:
			return bit / 8, 7 - bit%8
		case bit < 8*full+4:
			return full, 7 - (bit - 8*full)
		}
		j := bit - 8*full - 4
		return full + 1 + j/8, 7 - j%8
	}
}

// qrMaskScores applies each QR mask to a copy of the unmasked symbol u, with
// its format information, and returns the penalties and their rule split, as
// selectAndApplyMask computes them.
func qrMaskScores(u *matrix, ec ECLevel) ([]int, [][4]int) {
	scores := make([]int, numMasks)
	rules := make([][4]int, numMasks)
	for k := range numMasks {
		t := u.clone()
		t.applyMask(k)
		t.writeFormatInfo(ec, k)
		rules[k] = [4]int{t.penaltyRule1(), t.penaltyRule2(), t.penaltyRule3(), t.penaltyRule4()}
		scores[k] = rules[k][0] + rules[k][1] + rules[k][2] + rules[k][3]
	}
	return scores, rules
}

// microMaskScores applies each Micro mask to a copy of the unmasked symbol u
// and returns the edge scores, as selectMicroMask computes them.
func microMaskScores(u *matrix) []int {
	scores := make([]int, numMicroMasks)
	for k := range numMicroMasks {
		t := u.clone()
		t.applyMask(microMaskToQR[k])
		scores[k] = t.microMaskScore()
	}
	return scores
}
