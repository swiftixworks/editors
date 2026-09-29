package main

import "strings"

// Text helpers for byte strings that hold UTF-8. Swiftix Go has no bit
// operators, rune literals, or strconv.Itoa, so byte classes are compared
// numerically and numbers are formatted by hand.
//
// Every Go statement costs several VM instructions and each call more, so the
// loops that run per byte are written inline rather than through helpers.

const tabSize = 8

// isContinuation reports whether b is a UTF-8 continuation byte.
func isContinuation(b int) bool {
	return b >= 128 && b < 192
}

// nextCharStart returns the byte index of the character after the one at i.
func nextCharStart(s string, i int) int {
	if i >= len(s) {
		return len(s)
	}
	i++
	for i < len(s) && s[i] >= 128 && s[i] < 192 {
		i++
	}
	return i
}

// previousCharStart returns the byte index of the character before i.
func previousCharStart(s string, i int) int {
	if i <= 0 {
		return 0
	}
	i--
	for i > 0 && s[i] >= 128 && s[i] < 192 {
		i--
	}
	return i
}

// displayColumn converts byte index end of s to a display column. Every
// non-control character takes one cell, matching the Swiftix terminal
// renderer; tabs advance to the next multiple of tabSize and control bytes
// show as two-cell caret sequences.
func displayColumn(s string, end int) int {
	if end > len(s) {
		end = len(s)
	}
	column := 0
	for i := 0; i < end; i++ {
		b := s[i]
		if b >= 32 && b < 127 {
			column++
		} else if b >= 192 {
			column++
		} else if b == 9 {
			column = column + tabSize - column%tabSize
		} else if b < 128 {
			column = column + 2
		}
	}
	return column
}

// byteIndexForColumn returns the start of the character that covers display
// column target, or len(s) when the line is shorter.
func byteIndexForColumn(s string, target int) int {
	column := 0
	for i := 0; i < len(s); i++ {
		b := s[i]
		width := 0
		if b >= 32 && b < 127 {
			width = 1
		} else if b >= 192 {
			width = 1
		} else if b == 9 {
			width = tabSize - column%tabSize
		} else if b < 128 {
			width = 2
		}
		if width > 0 && column+width > target {
			return i
		}
		column = column + width
	}
	return len(s)
}

// charCount returns the number of UTF-8 characters in s.
func charCount(s string) int {
	count := 0
	for i := 0; i < len(s); i++ {
		if s[i] < 128 || s[i] >= 192 {
			count++
		}
	}
	return count
}

// isPlain reports whether s is printable ASCII, where bytes are columns.
func isPlain(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 32 || s[i] > 126 {
			return false
		}
	}
	return true
}

// itoa formats an integer.
func itoa(n int) string {
	if n < 0 {
		return "-" + itoa(-n)
	}
	if n < 10 {
		return "0123456789"[n : n+1]
	}
	return itoa(n/10) + "0123456789"[n%10:n%10+1]
}

// spaces returns n spaces.
func spaces(n int) string {
	if n <= 0 {
		return ""
	}
	return strings.Repeat(" ", n)
}

// caretLetter returns the letter shown after ^ for control byte b.
func caretLetter(b int) string {
	if b == 127 {
		return "?"
	}
	return "@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_"[b : b+1]
}

// clip returns the part of s visible in width display columns starting at
// display column offset, with tabs expanded and control bytes shown as ^X.
func clip(s string, offset int, width int) string {
	if isPlain(s) {
		if offset >= len(s) {
			return ""
		}
		end := offset + width
		if end > len(s) {
			end = len(s)
		}
		return s[offset:end]
	}
	out := ""
	column := 0
	limit := offset + width
	i := 0
	for i < len(s) && column < limit {
		b := s[i]
		if b >= 32 && b < 127 {
			// Copy a run of printable ASCII as one slice.
			start := i
			for i < len(s) && s[i] >= 32 && s[i] < 127 && column < limit {
				if column < offset {
					start = i + 1
				}
				column++
				i++
			}
			out = out + s[start:i]
		} else if b == 9 {
			next := column + tabSize - column%tabSize
			for column < next {
				if column >= offset && column < limit {
					out = out + " "
				}
				column++
			}
			i++
		} else if b < 128 {
			if column >= offset && column < limit {
				out = out + "^"
			}
			if column+1 >= offset && column+1 < limit {
				out = out + caretLetter(b)
			}
			column = column + 2
			i++
		} else {
			next := nextCharStart(s, i)
			if column >= offset {
				out = out + s[i:next]
			}
			column++
			i = next
		}
	}
	return out
}
