package main

import "swiftix/userland"

// Keys decoded from raw terminal input. Control keys keep their byte value
// (^A is 1); special keys use codes above the byte range.

const keyTab = 9
const keyEnter = 13
const keyEscape = 27
const keyBackspace = 127
const keyText = 1000
const keyUp = 1001
const keyDown = 1002
const keyRight = 1003
const keyLeft = 1004
const keyHome = 1005
const keyEnd = 1006
const keyDelete = 1007
const keyPageUp = 1008
const keyPageDown = 1009
const keyUnknown = 1010
const keyEndOfInput = 1011

const ctrlA = 1
const ctrlB = 2
const ctrlC = 3
const ctrlD = 4
const ctrlE = 5
const ctrlF = 6
const ctrlG = 7
const ctrlH = 8
const ctrlK = 11
const ctrlL = 12
const ctrlN = 14
const ctrlO = 15
const ctrlP = 16
const ctrlU = 21
const ctrlV = 22
const ctrlW = 23
const ctrlX = 24
const ctrlY = 25

type Key struct {
	code int
	text string
}

var pendingKeys []Key
var nextPendingKey int

// readKey returns the next key, reading more terminal input when needed.
func readKey() Key {
	for nextPendingKey >= len(pendingKeys) {
		pendingKeys = []Key{}
		nextPendingKey = 0
		data, status := userland.ReadStdin()
		if status != 0 {
			return Key{code: keyEndOfInput}
		}
		decodeKeys(data)
	}
	key := pendingKeys[nextPendingKey]
	nextPendingKey++
	return key
}

func pushKey(code int, text string) {
	pendingKeys = append(pendingKeys, Key{code: code, text: text})
}

// decodeKeys splits one chunk of terminal input into keys. A terminal sends
// an escape sequence in a single write, so an ESC that ends a chunk is the
// Escape key itself.
func decodeKeys(data string) {
	i := 0
	for i < len(data) {
		b := data[i]
		if b == 27 {
			i = decodeEscape(data, i)
		} else if b == 10 || b == 13 {
			pushKey(keyEnter, "")
			i++
		} else if b == 127 || b == 8 {
			pushKey(keyBackspace, "")
			i++
		} else if b == 9 {
			pushKey(keyTab, "")
			i++
		} else if b < 32 {
			pushKey(b, "")
			i++
		} else {
			start := i
			for i < len(data) && data[i] >= 32 && data[i] != 127 {
				i++
			}
			pushKey(keyText, data[start:i])
		}
	}
}

// decodeEscape decodes the sequence starting at the ESC at index i and returns
// the index after it.
func decodeEscape(data string, i int) int {
	if i+1 >= len(data) {
		pushKey(keyEscape, "")
		return i + 1
	}
	introducer := data[i+1]
	if introducer != 91 && introducer != 79 {
		// ESC followed by a key is a Meta combination; nano's Meta
		// bindings are not implemented.
		pushKey(keyUnknown, "")
		return nextCharStart(data, i+1)
	}
	j := i + 2
	parameter := 0
	for j < len(data) && data[j] >= 48 && data[j] <= 57 {
		parameter = parameter*10 + data[j] - 48
		j++
	}
	for j < len(data) && (data[j] < 64 || data[j] > 126) {
		j++
	}
	if j >= len(data) {
		pushKey(keyUnknown, "")
		return j
	}
	final := data[j]
	code := keyUnknown
	if final == 65 {
		code = keyUp
	} else if final == 66 {
		code = keyDown
	} else if final == 67 {
		code = keyRight
	} else if final == 68 {
		code = keyLeft
	} else if final == 72 {
		code = keyHome
	} else if final == 70 {
		code = keyEnd
	} else if final == 126 {
		if parameter == 1 || parameter == 7 {
			code = keyHome
		} else if parameter == 4 || parameter == 8 {
			code = keyEnd
		} else if parameter == 3 {
			code = keyDelete
		} else if parameter == 5 {
			code = keyPageUp
		} else if parameter == 6 {
			code = keyPageDown
		}
	}
	pushKey(code, "")
	return j + 1
}
