package main

import "strings"
import "swiftix/userland"

// The buffer is a slice of lines without their newlines. The cursor is a line
// index and a byte offset that always sits on a character boundary.

// maxFileBytes bounds the files nano opens. Loading, searching, saving, and
// inserting or removing lines run through native strings calls, whose joined
// text must stay within the 1 MiB Swiftix Go string limit while editing.
const maxFileBytes = 524288

var lines []string
var cursorRow int
var cursorByte int

// wantedColumn is the display column vertical movement tries to keep.
var wantedColumn int

var fileName string
var modified bool

// cutLines holds the lines removed by consecutive ^K presses.
var cutLines []string
var lastKeyWasCut bool

var lastSearch string

// loadFile replaces the buffer with the file at path and returns the status
// message to show. A missing file starts an empty buffer.
func loadFile(path string) string {
	fileName = path
	modified = false
	lines = []string{""}
	cursorRow = 0
	cursorByte = 0
	wantedColumn = 0
	if path == "" {
		return ""
	}
	data, status := userland.ReadInput("nano", []string{path})
	if status != 0 {
		return "New File"
	}
	if len(data) > maxFileBytes {
		fileName = ""
		return "File is too large to open (limit " + itoa(maxFileBytes/1024) + " KiB)"
	}
	lines = strings.Split(data, "\n")
	if len(lines) > 1 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return "Read " + plural(len(lines), "line")
}

// bufferText returns the file contents: every line ends with a newline, and
// a buffer holding one empty line is an empty file.
func bufferText() string {
	if len(lines) == 1 && lines[0] == "" {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

// saveFile writes the buffer to path and returns the status message.
func saveFile(path string) (string, bool) {
	if userland.WriteFile(path, bufferText()) != 0 {
		return "Error writing " + path, false
	}
	fileName = path
	modified = false
	return "Wrote " + plural(len(lines), "line"), true
}

func plural(n int, noun string) string {
	if n == 1 {
		return itoa(n) + " " + noun
	}
	return itoa(n) + " " + noun + "s"
}

func currentLine() string {
	return lines[cursorRow]
}

func rememberColumn() {
	wantedColumn = displayColumn(currentLine(), cursorByte)
}

// insertLineAt inserts text as a new line before index at. Lines never contain
// newlines, so the buffer is rebuilt with native Join and Split: shifting the
// slice in a guest loop would cost several VM instructions per line.
func insertLineAt(at int, text string) {
	if at >= len(lines) {
		lines = append(lines, text)
	} else if at == 0 {
		lines = strings.Split(text+"\n"+strings.Join(lines, "\n"), "\n")
	} else {
		lines = strings.Split(strings.Join(lines[:at], "\n")+"\n"+text+"\n"+
			strings.Join(lines[at:], "\n"), "\n")
	}
}

// removeLineAt deletes the line at index at; the buffer keeps one line.
func removeLineAt(at int) {
	if len(lines) == 1 {
		lines = []string{""}
	} else if at == 0 {
		lines = lines[1:]
	} else if at == len(lines)-1 {
		lines = lines[:at]
	} else {
		lines = strings.Split(strings.Join(lines[:at], "\n")+"\n"+
			strings.Join(lines[at+1:], "\n"), "\n")
	}
}

// insertText inserts text without newlines at the cursor.
func insertText(text string) {
	line := currentLine()
	lines[cursorRow] = line[:cursorByte] + text + line[cursorByte:]
	cursorByte = cursorByte + len(text)
	modified = true
	rememberColumn()
}

// insertNewline splits the current line at the cursor.
func insertNewline() {
	line := currentLine()
	lines[cursorRow] = line[:cursorByte]
	insertLineAt(cursorRow+1, line[cursorByte:])
	cursorRow++
	cursorByte = 0
	modified = true
	wantedColumn = 0
}

// insertMultiline inserts text that may contain newlines at the cursor.
func insertMultiline(text string) {
	start := 0
	for i := 0; i < len(text); i++ {
		if text[i] == 10 {
			if i > start {
				insertText(text[start:i])
			}
			insertNewline()
			start = i + 1
		}
	}
	if start < len(text) {
		insertText(text[start:])
	}
}

// deleteBackward removes the character before the cursor, joining lines at
// the start of a line.
func deleteBackward() {
	if cursorByte > 0 {
		line := currentLine()
		start := previousCharStart(line, cursorByte)
		lines[cursorRow] = line[:start] + line[cursorByte:]
		cursorByte = start
	} else if cursorRow > 0 {
		previous := lines[cursorRow-1]
		lines[cursorRow-1] = previous + currentLine()
		removeLineAt(cursorRow)
		cursorRow--
		cursorByte = len(previous)
	} else {
		return
	}
	modified = true
	rememberColumn()
}

// deleteForward removes the character under the cursor, joining the next line
// at the end of a line.
func deleteForward() {
	line := currentLine()
	if cursorByte < len(line) {
		end := nextCharStart(line, cursorByte)
		lines[cursorRow] = line[:cursorByte] + line[end:]
	} else if cursorRow+1 < len(lines) {
		lines[cursorRow] = line + lines[cursorRow+1]
		removeLineAt(cursorRow + 1)
	} else {
		return
	}
	modified = true
	rememberColumn()
}

func moveLeft() {
	if cursorByte > 0 {
		cursorByte = previousCharStart(currentLine(), cursorByte)
	} else if cursorRow > 0 {
		cursorRow--
		cursorByte = len(currentLine())
	}
	rememberColumn()
}

func moveRight() {
	if cursorByte < len(currentLine()) {
		cursorByte = nextCharStart(currentLine(), cursorByte)
	} else if cursorRow+1 < len(lines) {
		cursorRow++
		cursorByte = 0
	}
	rememberColumn()
}

// moveVertically moves the cursor by delta lines, keeping wantedColumn.
func moveVertically(delta int) {
	row := cursorRow + delta
	if row < 0 {
		row = 0
	}
	if row > len(lines)-1 {
		row = len(lines) - 1
	}
	cursorRow = row
	cursorByte = byteIndexForColumn(currentLine(), wantedColumn)
}

func moveHome() {
	cursorByte = 0
	rememberColumn()
}

func moveEnd() {
	cursorByte = len(currentLine())
	rememberColumn()
}

// cutLine removes the current line into the cut buffer. Consecutive cuts
// accumulate, as in nano.
func cutLine() {
	if !lastKeyWasCut {
		cutLines = []string{}
	}
	cutLines = append(cutLines, currentLine())
	if len(lines) == 1 {
		lines[0] = ""
	} else {
		removeLineAt(cursorRow)
		if cursorRow >= len(lines) {
			cursorRow = len(lines) - 1
		}
	}
	cursorByte = 0
	wantedColumn = 0
	modified = true
}

// pasteCut inserts the cut lines at the cursor.
func pasteCut() bool {
	if len(cutLines) == 0 {
		return false
	}
	text := ""
	for i := 0; i < len(cutLines); i++ {
		text = text + cutLines[i] + "\n"
	}
	insertMultiline(text)
	return true
}

// searchForward moves the cursor to the next occurrence of query after the
// cursor, wrapping around the buffer once. It reports whether it found one
// and whether the search wrapped. The search runs natively over the joined
// buffer, whose newlines map byte offsets back to lines.
func searchForward(query string) (bool, bool) {
	text := strings.Join(lines, "\n")
	start := cursorByte
	if cursorRow > 0 {
		start = start + len(strings.Join(lines[:cursorRow], "\n")) + 1
	}
	// Skip the character under the cursor, or the newline at a line's end.
	from := start + 1
	if cursorByte < len(currentLine()) {
		from = start + nextCharStart(currentLine(), cursorByte) - cursorByte
	}
	index := -1
	if from <= len(text) {
		found := strings.Index(text[from:], query)
		if found >= 0 {
			index = from + found
		}
	}
	wrapped := false
	if index < 0 {
		wrapped = true
		index = strings.Index(text, query)
	}
	if index < 0 {
		return false, wrapped
	}
	before := text[:index]
	cursorRow = strings.Count(before, "\n")
	cursorByte = index - strings.LastIndex(before, "\n") - 1
	rememberColumn()
	return true, wrapped
}
