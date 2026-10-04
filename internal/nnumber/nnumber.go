// Package nnumber derives the 24-bit ICAO aircraft address of a US civil
// aircraft from its N-number registration.
//
// The FAA assigns US civil aircraft the addresses A00001 to ADF7C7 in a fixed
// order of registrations (N1, N1A, N1AA, N1AB, ..., N99999), so the address
// is a function of the registration. The derivation is exact for current
// registrations, but it is a derivation from the transmitted tail rather than
// a transmitted address, and callers record it as such.
package nnumber

import "strings"

// letters are the letters used in N-numbers: A to Z without I and O.
const letters = "ABCDEFGHJKLMNPQRSTUVWXYZ"

const (
	firstAddress = 0xA00001

	// suffixSize is the number of registrations formed by an optional letter
	// suffix of one or two letters: none, 24 single letters, and 24 x 24
	// pairs.
	suffixSize = 1 + len(letters)*(1+len(letters)) // 601

	// bucketNSize is the number of registrations that share their first N
	// digits.
	bucket4Size = 1 + len(letters) + 10       // 35
	bucket3Size = 10*bucket4Size + suffixSize // 951
	bucket2Size = 10*bucket3Size + suffixSize // 10111
	bucket1Size = 10*bucket2Size + suffixSize // 101711
)

// ICAOAddress returns the ICAO address, as six upper-case hex digits, of a
// US N-number registration, and false if tail is not a valid N-number. A
// leading "." (as some messages transmit the tail) is ignored.
func ICAOAddress(tail string) (string, bool) {
	s := strings.ToUpper(strings.TrimPrefix(strings.TrimSpace(tail), "."))
	if len(s) < 2 || len(s) > 6 || s[0] != 'N' {
		return "", false
	}
	s = s[1:]
	if s[0] < '1' || s[0] > '9' {
		return "", false
	}

	addr := firstAddress + int(s[0]-'1')*bucket1Size
	bucketSizes := []int{bucket2Size, bucket3Size, bucket4Size}

	// Up to four further digits, each optionally ending in a letter suffix.
	for i := 1; i < len(s); i++ {
		c := s[i]
		if isLetter(c) {
			// A letter suffix ends the registration: one letter, or two
			// letters when it starts before the fifth character.
			rest := s[i:]
			offset, ok := suffixOffset(rest, i)
			if !ok {
				return "", false
			}
			addr += offset
			return hexAddress(addr), true
		}
		if c < '0' || c > '9' {
			return "", false
		}
		if i == 4 {
			// The fifth character: a digit after the 35 single-character
			// options (none, 24 letters) of the fourth level.
			addr += 1 + len(letters) + int(c-'0')
			continue
		}
		addr += suffixSize + int(c-'0')*bucketSizes[i-1]
	}
	return hexAddress(addr), true
}

// suffixOffset returns the offset of a letter suffix starting at position
// pos (1-based after the N's first digit). Up to two letters are allowed,
// except at the last position, where only one is.
func suffixOffset(suffix string, pos int) (int, bool) {
	first := strings.IndexByte(letters, suffix[0])
	if first < 0 {
		return 0, false
	}
	if pos == 4 {
		// Only a single letter fits in the fifth character.
		if len(suffix) != 1 {
			return 0, false
		}
		return first + 1, true
	}
	offset := first*(len(letters)+1) + 1
	switch len(suffix) {
	case 1:
		return offset, true
	case 2:
		second := strings.IndexByte(letters, suffix[1])
		if second < 0 {
			return 0, false
		}
		return offset + second + 1, true
	default:
		return 0, false
	}
}

func isLetter(c byte) bool {
	return c >= 'A' && c <= 'Z'
}

func hexAddress(addr int) string {
	const hexDigits = "0123456789ABCDEF"
	out := make([]byte, 6)
	for i := 5; i >= 0; i-- {
		out[i] = hexDigits[addr&0xF]
		addr >>= 4
	}
	return string(out)
}
