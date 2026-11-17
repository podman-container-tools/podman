// Copyright 2019+ Klaus Post. All rights reserved.
// License information can be found in the LICENSE file.
// Based on work by Yann Collet, released under BSD License.

package zstd

import (
	"errors"
	"fmt"
	"io"
)

type seq struct {
	litLen   uint32
	matchLen uint32
	offset   uint32

	// Codes are stored here for the encoder
	// so they only have to be looked up once.
	llCode, mlCode, ofCode uint8
}

type seqVals struct {
	ll, ml, mo int
}

func (s seq) String() string {
	if s.offset <= 3 {
		if s.offset == 0 {
			return fmt.Sprint("litLen:", s.litLen, ", matchLen:", s.matchLen+zstdMinMatch, ", offset: INVALID (0)")
		}
		return fmt.Sprint("litLen:", s.litLen, ", matchLen:", s.matchLen+zstdMinMatch, ", offset:", s.offset, " (repeat)")
	}
	return fmt.Sprint("litLen:", s.litLen, ", matchLen:", s.matchLen+zstdMinMatch, ", offset:", s.offset-3, " (new)")
}

type seqCompMode uint8

const (
	compModePredefined seqCompMode = iota
	compModeRLE
	compModeFSE
	compModeRepeat
)

type sequenceDec struct {
	// decoder keeps track of the current state and updates it from the bitstream.
	fse    *fseDecoder
	state  fseState
	repeat bool
}

// init the state of the decoder with input from stream.
func (s *sequenceDec) init(br *bitReader) error {
	if s.fse == nil {
		return errors.New("sequence decoder not defined")
	}
	s.state.init(br, s.fse.actualTableLog, s.fse.dt[:1<<s.fse.actualTableLog])
	return nil
}

// sequenceDecs contains all 3 sequence decoders and their state.
type sequenceDecs struct {
	litLengths   sequenceDec
	offsets      sequenceDec
	matchLengths sequenceDec
	prevOffset   [3]int
	dict         []byte
	literals     []byte
	out          []byte
	nSeqs        int
	br           *bitReader
	seqSize      int
	windowSize   int
	maxBits      uint8
	maxSyncLen   uint64
}

// twoPassFarCode is the offset code from which a match counts as far: 20 is
// 1 MiB, the distance every measured arm64 core gains from. A Neoverse N1
// (Graviton2, Ampere Altra) gains from 128 KiB too, but that needs a
// per-core threshold override; not worth the CPU-specific special case for
// a core class already being phased out of the fleet.
const twoPassFarCode = 20

// useTwoPass reports whether the block is decoded in two passes with the
// match-source prefetch (seqdec_arm64.go): a window of at least
// decodeTwoPassMinWindow and, unless the offset table is predefined or
// RLE, a share of at least twoPassMinFarShare of codes twoPassFarCode+.
func (s *sequenceDecs) useTwoPass() bool {
	if s.windowSize < decodeTwoPassMinWindow {
		return false
	}
	fse := s.offsets.fse
	if fse != nil && !fse.preDefined && fse.actualTableLog != 0 && int(fse.symbolLen) <= twoPassFarCode {
		// No symbol reaches twoPassFarCode, so codeShare can only return 0:
		// skip finding it.
		return false
	}
	share := fse.codeShare(twoPassFarCode)
	return share < 0 || share >= twoPassMinFarShare
}

// initialize all 3 decoders from the stream input.
func (s *sequenceDecs) initialize(br *bitReader, hist *history, out []byte) error {
	if err := s.litLengths.init(br); err != nil {
		return errors.New("litLengths:" + err.Error())
	}
	if err := s.offsets.init(br); err != nil {
		return errors.New("offsets:" + err.Error())
	}
	if err := s.matchLengths.init(br); err != nil {
		return errors.New("matchLengths:" + err.Error())
	}
	s.br = br
	s.prevOffset = hist.recentOffsets
	s.maxBits = s.litLengths.fse.maxBits + s.offsets.fse.maxBits + s.matchLengths.fse.maxBits
	s.windowSize = hist.windowSize
	s.out = out
	s.dict = nil
	if hist.dict != nil {
		s.dict = hist.dict.content
	}
	return nil
}

// consumeSyncLen accounts for n bytes of frame output produced outside
// decodeSync. maxSyncLen bounds how much the frame may still produce and
// decodeSyncSimple compares it with the output buffer's slack to decide
// whether the extended 16-byte copies are safe. Blocks with sequences
// subtracted their output; raw blocks, RLE blocks and compressed blocks
// holding only literals did not, so a frame that opened with a raw block
// overstated its remaining size for every block after it and ran the
// bounds-exact copies for the rest of the frame.
//
// Zero means "no bound" and selects the conservative paths, so a block that
// meets or exceeds the bound lands there; the frame-size checks in
// runDecoder reject the excess afterwards.
func (s *sequenceDecs) consumeSyncLen(n int) {
	if s.maxSyncLen == 0 {
		return
	}
	if uint64(n) >= s.maxSyncLen {
		s.maxSyncLen = 0
		return
	}
	s.maxSyncLen -= uint64(n)
}

func (s *sequenceDecs) freeDecoders() {
	if f := s.litLengths.fse; f != nil && !f.preDefined {
		fseDecoderPool.Put(f)
		s.litLengths.fse = nil
	}
	if f := s.offsets.fse; f != nil && !f.preDefined {
		fseDecoderPool.Put(f)
		s.offsets.fse = nil
	}
	if f := s.matchLengths.fse; f != nil && !f.preDefined {
		fseDecoderPool.Put(f)
		s.matchLengths.fse = nil
	}
}

// execute will execute the decoded sequence with the provided history.
// The sequence must be evaluated before being sent.
func (s *sequenceDecs) execute(seqs []seqVals, hist []byte) error {
	if len(s.dict) == 0 {
		return s.executeSimple(seqs, hist)
	}

	// Ensure we have enough output size...
	if len(s.out)+s.seqSize > cap(s.out) {
		addBytes := s.seqSize + len(s.out)
		s.out = append(s.out, make([]byte, addBytes)...)
		s.out = s.out[:len(s.out)-addBytes]
	}

	if debugDecoder {
		printf("Execute %d seqs with hist %d, dict %d, literals: %d into %d bytes\n", len(seqs), len(hist), len(s.dict), len(s.literals), s.seqSize)
	}

	var t = len(s.out)
	out := s.out[:t+s.seqSize]
	literals := s.literals
	windowSize := s.windowSize
	dict := s.dict

	for _, seq := range seqs {
		// Short sequences copy 16 bytes for each part.
		if seq.ll <= 16 && seq.ml <= 16 && t+seq.ll+16 <= cap(out) && cap(literals) >= 16 {
			if src := matchSource(out, hist, dict, t+seq.ll, seq.mo, 16, windowSize); src != nil {
				*(*[16]byte)(out[t : t+16]) = *(*[16]byte)(literals[:16])
				t += seq.ll
				*(*[16]byte)(out[t : t+16]) = *(*[16]byte)(src)
				t += seq.ml
				literals = literals[seq.ll:]
				continue
			}
		}

		// Add literals
		copy(out[t:], literals[:seq.ll])
		t += seq.ll
		literals = literals[seq.ll:]

		// Copy from dictionary...
		if seq.mo > t+len(hist) || seq.mo > windowSize {
			if len(s.dict) == 0 {
				return fmt.Errorf("match offset (%d) bigger than current history (%d)", seq.mo, t+len(hist))
			}

			// we may be in dictionary.
			dictO := len(s.dict) - (seq.mo - (t + len(hist)))
			if dictO < 0 || dictO >= len(s.dict) {
				return fmt.Errorf("match offset (%d) bigger than current history+dict (%d)", seq.mo, t+len(hist)+len(s.dict))
			}
			end := dictO + seq.ml
			if end > len(s.dict) {
				n := len(s.dict) - dictO
				copy(out[t:], s.dict[dictO:])
				t += n
				seq.ml -= n
			} else {
				copy(out[t:], s.dict[dictO:end])
				t += end - dictO
				continue
			}
		}

		// Copy from history.
		if v := seq.mo - t; v > 0 {
			// v is the start position in history from end.
			start := len(hist) - v
			if seq.ml > v {
				// Some goes into current block.
				// Copy remainder of history
				copy(out[t:], hist[start:])
				t += v
				seq.ml -= v
			} else {
				copy(out[t:], hist[start:start+seq.ml])
				t += seq.ml
				continue
			}
		}
		// We must be in current buffer now
		if seq.ml > 0 {
			start := t - seq.mo
			if seq.ml <= t-start {
				// No overlap
				copy(out[t:], out[start:start+seq.ml])
				t += seq.ml
				continue
			} else {
				// Overlapping copy
				overlapCopy(out, start, t, seq.ml)
				t += seq.ml
			}
		}
	}

	// Add final literals
	copy(out[t:], literals)
	s.literals = literals
	if debugDecoder {
		t += len(literals)
		if t != len(out) {
			panic(fmt.Errorf("length mismatch, want %d, got %d, ss: %d", len(out), t, s.seqSize))
		}
	}
	s.out = out

	return nil
}

// decodeSyncGoOnly makes decodeSync skip the assembly decoder, so tests can
// compare it with the Go one in the same binary.
var decodeSyncGoOnly bool

// decode sequences from the stream with the provided history.
func (s *sequenceDecs) decodeSync(hist []byte) error {
	if !decodeSyncGoOnly {
		supported, err := s.decodeSyncSimple(hist)
		if supported {
			return err
		}
	}

	// The bit reader, repeat offsets and literals are held in locals so
	// they stay in registers; they are written back on success.
	br := s.br
	in, bs := br.in, br.state()
	prev0, prev1, prev2 := s.prevOffset[0], s.prevOffset[1], s.prevOffset[2]
	literals := s.literals
	maxBits := s.maxBits
	windowSize := s.windowSize
	dict := s.dict
	seqs := s.nSeqs
	startSize := len(s.out)
	// Grab full sizes tables, to avoid bounds checks.
	llTable, mlTable, ofTable := s.litLengths.fse.dt[:maxTablesize], s.matchLengths.fse.dt[:maxTablesize], s.offsets.fse.dt[:maxTablesize]
	llState, mlState, ofState := s.litLengths.state.state, s.matchLengths.state.state, s.offsets.state.state
	out := s.out
	maxBlockSize := min(windowSize, maxCompressedBlockSize)

	if debugDecoder {
		println("decodeSync: decoding", seqs, "sequences", br.remain(), "bits remain on stream")
	}
	for i := seqs - 1; i >= 0; i-- {
		if bs.bitsRead > 64 {
			br.setState(bs)
			printf("reading sequence %d, exceeded available data. Overread by %d\n", seqs-i, -br.remain())
			return io.ErrUnexpectedEOF
		}
		var ll, mo, ml int
		// Final will not read from stream.
		var llB, mlB, moB uint8
		ll, llB = llState.final()
		ml, mlB = mlState.final()
		mo, moB = ofState.final()

		// extra bits are stored in reverse order.
		var v int
		bs = bs.fill(in)
		bs, v = bs.getBits(moB)
		mo += v
		if maxBits > 32 {
			bs = bs.fill(in)
		}
		bs, v = bs.getBits(mlB)
		ml += v
		bs, v = bs.getBits(llB)
		ll += v

		if moB > 1 {
			prev2 = prev1
			prev1 = prev0
			prev0 = mo
		} else {
			// mo = s.adjustOffset(mo, ll, moB)
			// Inlined for rather big speedup
			if ll == 0 {
				// There is an exception though, when current sequence's literals_length = 0.
				// In this case, repeated offsets are shifted by one, so an offset_value of 1 means Repeated_Offset2,
				// an offset_value of 2 means Repeated_Offset3, and an offset_value of 3 means Repeated_Offset1 - 1_byte.
				mo++
			}

			if mo == 0 {
				mo = prev0
			} else {
				var temp int
				switch mo {
				case 1:
					temp = prev1
				case 2:
					temp = prev2
				default:
					temp = prev0 - 1
				}

				if temp == 0 {
					// 0 is not valid; input is corrupted; force offset to 1
					println("WARNING: temp was 0")
					temp = 1
				}

				if mo != 1 {
					prev2 = prev1
				}
				prev1 = prev0
				prev0 = temp
				mo = temp
			}
		}
		bs = bs.fill(in)

		if debugSequences {
			println("Seq", seqs-i-1, "Litlen:", ll, "mo:", mo, "(abs) ml:", ml)
		}

		if ll > len(literals) {
			return fmt.Errorf("unexpected literal count, want %d bytes, but only %d is available", ll, len(literals))
		}
		size := ll + ml + len(out)
		if size-startSize > maxBlockSize {
			return fmt.Errorf("output bigger than max block size (%d)", maxBlockSize)
		}
		// Matches from earlier output, the history or the dictionary are
		// copied without calling executeSeq; short ones with fixed 16-byte copies, with no
		// calls so the loop state stays in registers.
		t := len(out)
		p := t + ll
		short := ll <= 16 && ml <= 16 && p+16 <= cap(out) && cap(literals) >= 16
		var src []byte
		if short {
			src = matchSource(out, hist, dict, p, mo, 16, windowSize)
		}
		if src == nil && p+ml <= cap(out) {
			short = false
			src = matchSource(out, hist, dict, p, mo, ml, windowSize)
		}
		if short {
			*(*[16]byte)(out[t : t+16]) = *(*[16]byte)(literals[:16])
			*(*[16]byte)(out[p : p+16]) = *(*[16]byte)(src)
			out = out[:p+ml]
			literals = literals[ll:]
		} else if src != nil {
			out = out[:p+ml]
			copy(out[t:p], literals[:ll])
			copy(out[p:], src)
			literals = literals[ll:]
		} else {
			var err error
			out, literals, err = s.executeSeq(out, literals, hist, ll, mo, ml, startSize, maxBlockSize)
			if err != nil {
				return err
			}
		}
		if i == 0 {
			// This is the last sequence, so we shouldn't update state.
			break
		}

		// Manually inlined, ~ 5-20% faster
		// Update all 3 states at once. Approx 20% faster.
		nBits := llState.nbBits() + mlState.nbBits() + ofState.nbBits()
		if nBits == 0 {
			llState = llTable[llState.newState()&maxTableMask]
			mlState = mlTable[mlState.newState()&maxTableMask]
			ofState = ofTable[ofState.newState()&maxTableMask]
		} else {
			var bits uint32
			bs, bits = bs.get32BitsFast(nBits)

			lowBits := uint16(bits >> ((ofState.nbBits() + mlState.nbBits()) & 31))
			llState = llTable[(llState.newState()+lowBits)&maxTableMask]

			lowBits = uint16(bits >> (ofState.nbBits() & 31))
			lowBits &= bitMask[mlState.nbBits()&15]
			mlState = mlTable[(mlState.newState()+lowBits)&maxTableMask]

			lowBits = uint16(bits) & bitMask[ofState.nbBits()&15]
			ofState = ofTable[(ofState.newState()+lowBits)&maxTableMask]
		}
	}

	if size := len(literals) + len(out) - startSize; size > maxBlockSize {
		return fmt.Errorf("output bigger than max block size (%d)", maxBlockSize)
	}

	// Add final literals
	s.out = append(out, literals...)
	s.literals = literals
	s.prevOffset = [3]int{prev0, prev1, prev2}
	br.setState(bs)
	return br.close()
}

// overlapCopy writes n bytes at out[t:] repeating out[start:t], which
// overlaps the destination. Each copy doubles the repeated span, so it takes
// about log2(n/(t-start)) copies instead of n byte moves.
func overlapCopy(out []byte, start, t, n int) {
	if start >= t {
		// Nothing to repeat; callers reject a zero offset.
		return
	}
	for end := t + n; t < end; {
		t += copy(out[t:end], out[start:t])
	}
}

// matchSource returns the n bytes a match of length n at p in out copies
// from: earlier output it does not overlap, the history or the dictionary.
// It returns nil when the match needs the general path. It must stay
// inlinable: a call would spill the decode loop's state.
func matchSource(out, hist, dict []byte, p, mo, n, windowSize int) []byte {
	// v is how far the match starts back from the end of hist, and d how
	// far it reaches past hist into dict.
	if mo <= windowSize {
		if mo >= n && mo <= p {
			return out[p-mo : p-mo+n]
		}
		if v := mo - p; v >= n && v <= len(hist) {
			return hist[len(hist)-v:][:n]
		}
	}
	if d := mo - (p + len(hist)); d >= n && d <= len(dict) {
		return dict[len(dict)-d:][:n]
	}
	return nil
}

// executeSeq appends the literals and match of one sequence to out and
// returns the updated out and remaining literals.
func (s *sequenceDecs) executeSeq(out, literals, hist []byte, ll, mo, ml, startSize, maxBlockSize int) ([]byte, []byte, error) {
	if ll+ml+len(out) > cap(out) {
		// Not enough size, which can happen under high volume block streaming conditions
		// but could be if destination slice is too small for sync operations.
		// over-allocating here can create a large amount of GC pressure so we try to keep
		// it as contained as possible
		used := len(out) - startSize
		addBytes := 256 + ll + ml + used>>2
		// Clamp to max block size.
		if used+addBytes > maxBlockSize {
			addBytes = maxBlockSize - used
		}
		out = append(out, make([]byte, addBytes)...)
		out = out[:len(out)-addBytes]
	}
	if ml > maxMatchLen {
		return nil, nil, fmt.Errorf("match len (%d) bigger than max allowed length", ml)
	}

	// Add literals
	out = append(out, literals[:ll]...)
	literals = literals[ll:]

	if mo == 0 && ml > 0 {
		return nil, nil, fmt.Errorf("zero matchoff and matchlen (%d) > 0", ml)
	}

	if mo > len(out)+len(hist) || mo > s.windowSize {
		if len(s.dict) == 0 {
			return nil, nil, fmt.Errorf("match offset (%d) bigger than current history (%d)", mo, len(out)-startSize)
		}

		// we may be in dictionary.
		dictO := len(s.dict) - (mo - (len(out) + len(hist)))
		if dictO < 0 || dictO >= len(s.dict) {
			return nil, nil, fmt.Errorf("match offset (%d) bigger than current history (%d)", mo, len(out)-startSize)
		}
		end := dictO + ml
		if end > len(s.dict) {
			out = append(out, s.dict[dictO:]...)
			ml -= len(s.dict) - dictO
		} else {
			out = append(out, s.dict[dictO:end]...)
			mo = 0
			ml = 0
		}
	}

	// Copy from history.
	// TODO: Blocks without history could be made to ignore this completely.
	if v := mo - len(out); v > 0 {
		// v is the start position in history from end.
		start := len(hist) - v
		if ml > v {
			// Some goes into current block.
			// Copy remainder of history
			out = append(out, hist[start:]...)
			ml -= v
		} else {
			out = append(out, hist[start:start+ml]...)
			ml = 0
		}
	}
	// We must be in current buffer now
	if ml > 0 {
		start := len(out) - mo
		if ml <= len(out)-start {
			// No overlap
			out = append(out, out[start:start+ml]...)
		} else {
			// Overlapping copy
			out = out[:len(out)+ml]
			overlapCopy(out, start, len(out)-ml, ml)
		}
	}
	return out, literals, nil
}

var bitMask [16]uint16

func init() {
	for i := range bitMask[:] {
		bitMask[i] = uint16((1 << uint(i)) - 1)
	}
}

func (s *sequenceDecs) adjustOffset(offset, litLen int, offsetB uint8) int {
	if offsetB > 1 {
		s.prevOffset[2] = s.prevOffset[1]
		s.prevOffset[1] = s.prevOffset[0]
		s.prevOffset[0] = offset
		return offset
	}

	if litLen == 0 {
		// There is an exception though, when current sequence's literals_length = 0.
		// In this case, repeated offsets are shifted by one, so an offset_value of 1 means Repeated_Offset2,
		// an offset_value of 2 means Repeated_Offset3, and an offset_value of 3 means Repeated_Offset1 - 1_byte.
		offset++
	}

	if offset == 0 {
		return s.prevOffset[0]
	}
	var temp int
	if offset == 3 {
		temp = s.prevOffset[0] - 1
	} else {
		temp = s.prevOffset[offset]
	}

	if temp == 0 {
		// 0 is not valid; input is corrupted; force offset to 1
		println("temp was 0")
		temp = 1
	}

	if offset != 1 {
		s.prevOffset[2] = s.prevOffset[1]
	}
	s.prevOffset[1] = s.prevOffset[0]
	s.prevOffset[0] = temp
	return temp
}
