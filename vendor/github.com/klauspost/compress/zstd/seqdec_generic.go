//go:build (!amd64 && !arm64) || !gc || noasm

package zstd

import (
	"fmt"
	"io"
	"math"
)

// See seqdec_arm64.go; the pure Go decoder has neither path.
var (
	twoPassMinFarShare     = 257 // never: the share is at most 256
	decodeTwoPassMinWindow = math.MaxInt
)

// decode sequences from the stream with the provided history but without dictionary.
func (s *sequenceDecs) decodeSyncSimple(hist []byte) (bool, error) {
	return false, nil
}

// decode sequences from the stream without the provided history.
func (s *sequenceDecs) decode(seqs []seqVals) error {
	// The bit reader and repeat offsets are held in locals so they stay in
	// registers; they are written back on success.
	br := s.br
	in, bs := br.in, br.state()
	prev0, prev1, prev2 := s.prevOffset[0], s.prevOffset[1], s.prevOffset[2]
	maxBits := s.maxBits
	seqSize := 0

	// Grab full sizes tables, to avoid bounds checks.
	llTable, mlTable, ofTable := s.litLengths.fse.dt[:maxTablesize], s.matchLengths.fse.dt[:maxTablesize], s.offsets.fse.dt[:maxTablesize]
	llState, mlState, ofState := s.litLengths.state.state, s.matchLengths.state.state, s.offsets.state.state
	litRemain := len(s.literals)

	maxBlockSize := maxCompressedBlockSize
	if s.windowSize < maxBlockSize {
		maxBlockSize = s.windowSize
	}
	for i := range seqs {
		if bs.bitsRead > 64 {
			if debugDecoder {
				printf("reading sequence %d, exceeded available data\n", i)
			}
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
			println("Seq", i, "Litlen:", ll, "mo:", mo, "(abs) ml:", ml)
		}
		// Evaluate.
		// We might be doing this async, so do it early.
		if mo == 0 && ml > 0 {
			return fmt.Errorf("zero matchoff and matchlen (%d) > 0", ml)
		}
		if ml > maxMatchLen {
			return fmt.Errorf("match len (%d) bigger than max allowed length", ml)
		}
		seqSize += ll + ml
		if seqSize > maxBlockSize {
			return fmt.Errorf("output bigger than max block size (%d)", maxBlockSize)
		}
		litRemain -= ll
		if litRemain < 0 {
			return fmt.Errorf("unexpected literal count, want %d bytes, but only %d is available", ll, litRemain+ll)
		}
		seqs[i] = seqVals{
			ll: ll,
			ml: ml,
			mo: mo,
		}
		if i == len(seqs)-1 {
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
	s.seqSize = seqSize + litRemain
	if s.seqSize > maxBlockSize {
		return fmt.Errorf("output bigger than max block size (%d)", maxBlockSize)
	}
	s.prevOffset = [3]int{prev0, prev1, prev2}
	br.setState(bs)
	err := br.close()
	if err != nil {
		printf("Closing sequences: %v, %+v\n", err, *br)
	}
	return err
}

// executeSimple handles cases when a dictionary is not used.
func (s *sequenceDecs) executeSimple(seqs []seqVals, hist []byte) error {
	// Ensure we have enough output size...
	if len(s.out)+s.seqSize > cap(s.out) {
		addBytes := s.seqSize + len(s.out)
		s.out = append(s.out, make([]byte, addBytes)...)
		s.out = s.out[:len(s.out)-addBytes]
	}

	if debugDecoder {
		printf("Execute %d seqs with literals: %d into %d bytes\n", len(seqs), len(s.literals), s.seqSize)
	}

	var t = len(s.out)
	out := s.out[:t+s.seqSize]
	literals := s.literals
	windowSize := s.windowSize

	for _, seq := range seqs {
		// Short sequences copy 16 bytes for each part.
		if seq.ll <= 16 && seq.ml <= 16 && t+seq.ll+16 <= cap(out) && cap(literals) >= 16 {
			if src := matchSource(out, hist, nil, t+seq.ll, seq.mo, 16, windowSize); src != nil {
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

		// Malformed input
		if seq.mo > t+len(hist) || seq.mo > windowSize {
			return fmt.Errorf("match offset (%d) bigger than current history (%d)", seq.mo, t+len(hist))
		}

		// Copy from history.
		if v := seq.mo - t; v > 0 {
			// v is the start position in history from end.
			start := len(hist) - v
			if seq.ml > v {
				// Some goes into the current block.
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

		// We must be in the current buffer now
		if seq.ml > 0 {
			start := t - seq.mo
			if seq.ml <= t-start {
				// No overlap
				copy(out[t:], out[start:start+seq.ml])
				t += seq.ml
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
