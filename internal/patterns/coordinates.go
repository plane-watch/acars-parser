// Package patterns provides shared regex patterns and helper functions for ACARS parsing.
// This file contains coordinate conversion utilities.

package patterns

import (
	"strconv"
	"strings"
)

// ParseDMSCoord parses coordinates in various DMS formats and returns decimal degrees.
// Supported formats:
//   - DDMM.M (e.g., 3413.8 = 34°13.8')
//   - DDDMM.M (e.g., 15123.5 = 151°23.5')
//   - DDMMSS (e.g., 341348 = 34°13'48")
//   - DDDMMSS (e.g., 1512335 = 151°23'35")
//   - DDMMD (e.g., 34138 = 34°13.8')
//   - DDDMMD (e.g., 151235 = 151°23.5')
//
// degDigits specifies how many digits are degrees (2 for lat, 3 for lon).
// dir is the direction (N/S/E/W) - S and W result in negative values.
func ParseDMSCoord(s string, degDigits int, dir string) float64 {
	if s == "" {
		return 0
	}

	var deg, min float64

	// Check if it contains a decimal point.
	if strings.Contains(s, ".") {
		parts := strings.Split(s, ".")
		if len(parts) != 2 {
			return 0
		}

		wholePart := parts[0]
		decPart := parts[1]

		if len(wholePart) < degDigits {
			return 0
		}

		// Extract degrees.
		degStr := wholePart[:degDigits]
		degVal, err := strconv.Atoi(degStr)
		if err != nil {
			return 0
		}
		deg = float64(degVal)

		// Extract minutes (rest of whole part + decimal).
		minStr := wholePart[degDigits:] + "." + decPart
		minVal, err := strconv.ParseFloat(minStr, 64)
		if err != nil {
			return 0
		}
		min = minVal
	} else {
		// No decimal point - determine format by length.
		switch len(s) {
		case 5: // DDMMD format (latitude).
			if degDigits != 2 {
				return 0
			}
			degVal, err := strconv.Atoi(s[0:2])
			if err != nil {
				return 0
			}
			deg = float64(degVal)

			minWhole, err := strconv.Atoi(s[2:4])
			if err != nil {
				return 0
			}
			minTenths, err := strconv.Atoi(s[4:5])
			if err != nil {
				return 0
			}
			min = float64(minWhole) + float64(minTenths)/10.0

		case 6: // DDDMMD or DDMMSS format.
			if degDigits == 3 {
				// DDDMMD format (longitude).
				degVal, err := strconv.Atoi(s[0:3])
				if err != nil {
					return 0
				}
				deg = float64(degVal)

				minWhole, err := strconv.Atoi(s[3:5])
				if err != nil {
					return 0
				}
				minTenths, err := strconv.Atoi(s[5:6])
				if err != nil {
					return 0
				}
				min = float64(minWhole) + float64(minTenths)/10.0
			} else {
				// DDMMSS format (latitude).
				degVal, err := strconv.Atoi(s[0:2])
				if err != nil {
					return 0
				}
				deg = float64(degVal)

				minVal, err := strconv.Atoi(s[2:4])
				if err != nil {
					return 0
				}
				secVal, err := strconv.Atoi(s[4:6])
				if err != nil {
					return 0
				}
				min = float64(minVal) + float64(secVal)/60.0
			}

		case 7: // DDDMMSS format (longitude).
			if degDigits != 3 {
				return 0
			}
			degVal, err := strconv.Atoi(s[0:3])
			if err != nil {
				return 0
			}
			deg = float64(degVal)

			minVal, err := strconv.Atoi(s[3:5])
			if err != nil {
				return 0
			}
			secVal, err := strconv.Atoi(s[5:7])
			if err != nil {
				return 0
			}
			min = float64(minVal) + float64(secVal)/60.0

		default:
			// Try simple integer interpretation.
			val, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return 0
			}
			// If it's already a reasonable decimal value, return it.
			if val < 180 && val > -180 {
				if dir == "S" || dir == "W" {
					return -val
				}
				return val
			}
			return 0
		}
	}

	// Convert to decimal degrees.
	result := deg + min/60.0

	// Apply direction.
	if dir == "S" || dir == "W" {
		result = -result
	}

	return result
}

// ParseLatitude parses a latitude value with direction.
// Expects 2 degree digits.
func ParseLatitude(value, dir string) float64 {
	return ParseDMSCoord(value, 2, dir)
}

// ParseLongitude parses a longitude value with direction.
// Expects 3 degree digits.
func ParseLongitude(value, dir string) float64 {
	return ParseDMSCoord(value, 3, dir)
}

// ParseDecimalCoord parses coordinates that are already in decimal format.
// Applies direction sign (S/W = negative).
func ParseDecimalCoord(s string, dir string) float64 {
	if s == "" {
		return 0
	}

	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}

	if dir == "S" || dir == "W" {
		return -val
	}
	return val
}