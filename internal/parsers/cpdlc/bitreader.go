// Package cpdlc provides parsing for CPDLC (Controller-Pilot Data Link Communications) messages.
// Based on FANS-1/A (Future Air Navigation System) specification.
package cpdlc

import (
	"errors"
)

// ErrInsufficientBits is returned when there are not enough bits to read.
var ErrInsufficientBits = errors.New("insufficient bits in stream")

// BitReader reads bits from a byte slice using UPER (Unaligned PER) rules.
type BitReader struct {
	data   []byte
	offset int // Current bit offset.
	nbits  int // Total number of bits available.
}

// NewBitReader creates a new BitReader from a byte slice.
func NewBitReader(data []byte) *BitReader {
	return &BitReader{
		data:   data,
		offset: 0,
		nbits:  len(data) * 8,
	}
}

// Remaining returns the number of bits remaining.
func (br *BitReader) Remaining() int {
	return br.nbits - br.offset
}

// ReadBits reads up to 31 bits from the stream.
func (br *BitReader) ReadBits(nbits int) (uint32, error) {
	if nbits < 0 || nbits > 31 {
		return 0, errors.New("invalid bit count (must be 0-31)")
	}
	if nbits == 0 {
		return 0, nil
	}
	if br.offset+nbits > br.nbits {
		return 0, ErrInsufficientBits
	}

	// Calculate byte and bit positions.
	byteOffset := br.offset / 8
	bitOffset := br.offset % 8

	// Read enough bytes to cover the requested bits.
	var accum uint64
	bytesNeeded := (bitOffset + nbits + 7) / 8
	for i := 0; i < bytesNeeded && byteOffset+i < len(br.data); i++ {
		accum = (accum << 8) | uint64(br.data[byteOffset+i])
	}

	// Shift right to align and mask.
	shift := (bytesNeeded * 8) - bitOffset - nbits
	accum >>= shift
	accum &= (1 << nbits) - 1

	br.offset += nbits
	return uint32(accum), nil
}

// ReadBit reads a single bit and returns it as a boolean.
func (br *BitReader) ReadBit() (bool, error) {
	v, err := br.ReadBits(1)
	return v == 1, err
}

// ReadBytes reads a number of bytes (byte-aligned in the output, not the input).
func (br *BitReader) ReadBytes(nbytes int) ([]byte, error) {
	result := make([]byte, nbytes)
	for i := 0; i < nbytes; i++ {
		v, err := br.ReadBits(8)
		if err != nil {
			return nil, err
		}
		result[i] = byte(v)
	}
	return result, nil
}

// ReadConstrainedInt reads a constrained integer with the given range.
// For UPER, the number of bits depends on the range (upper - lower).
func (br *BitReader) ReadConstrainedInt(lower, upper int) (int, error) {
	if upper < lower {
		return 0, errors.New("upper bound must be >= lower bound")
	}

	rangeVal := upper - lower
	if rangeVal == 0 {
		return lower, nil // Single value, no bits needed.
	}

	// Calculate bits needed for the range.
	nbits := bitsNeeded(rangeVal)
	v, err := br.ReadBits(nbits)
	if err != nil {
		return 0, err
	}

	return lower + int(v), nil
}

// ReadLength reads a length determinant as per X.691 section 10.9.
func (br *BitReader) ReadLength() (int, error) {
	// First byte determines format.
	v, err := br.ReadBits(8)
	if err != nil {
		return 0, err
	}

	if (v & 0x80) == 0 {
		// Short form: 0xxxxxxx (0-127).
		return int(v & 0x7F), nil
	}

	if (v & 0x40) == 0 {
		// Medium form: 10xxxxxx xxxxxxxx (0-16383).
		v2, err := br.ReadBits(8)
		if err != nil {
			return 0, err
		}
		return int((v&0x3F)<<8 | v2), nil
	}

	// Long form: 11xxxxxx (multiplier for 16384).
	// The remaining 6 bits give multiplier (1-4).
	multiplier := int(v & 0x3F)
	if multiplier < 1 || multiplier > 4 {
		return 0, errors.New("invalid length multiplier")
	}
	return multiplier * 16384, nil
}

// ReadNormallySmallNonNegative reads a "normally small non-negative whole number".
// X.691, section 10.6.
func (br *BitReader) ReadNormallySmallNonNegative() (int, error) {
	v, err := br.ReadBits(7)
	if err != nil {
		return 0, err
	}

	if (v & 64) == 0 {
		// Short form: 0xxxxxx (0-63).
		return int(v), nil
	}

	// Extended form.
	v &= 63
	v <<= 2
	v2, err := br.ReadBits(2)
	if err != nil {
		return 0, err
	}
	v |= v2

	if (v & 128) != 0 {
		return 0, errors.New("invalid normally small number")
	}
	if v == 0 {
		return 0, nil
	}
	if v >= 3 {
		return 0, errors.New("normally small number too large")
	}

	result, err := br.ReadBits(int(8 * v))
	return int(result), err
}

// bitsNeeded calculates the number of bits needed to represent a range.
func bitsNeeded(rangeVal int) int {
	if rangeVal <= 0 {
		return 0
	}
	bits := 0
	for v := rangeVal; v > 0; v >>= 1 {
		bits++
	}
	return bits
}
