package main

import "fmt"
import "swiftix/userland"

// Layout: a title bar, the edit area, a status line, and two shortcut rows.
// A frame is written as one output. Only rows whose content changed since the
// last frame are redrawn, and rendered lines are cached, because the Swiftix Go
// VM spends several instructions per byte of text.

const version = "0.1.0"
const escape = "\x1b["
const inverse = "\x1b[7m"
const normal = "\x1b[0m"

var screenRows int
var screenColumns int

// topRow and leftColumn scroll the edit area.
var topRow int
var leftColumn int

var statusMessage string

var shortcutKeys []string
var shortcutLabels []string

// shownRows holds what each screen row displays now; forceRedraw repaints all.
var shownRows []string
var forceRedraw bool

// renderCache maps a line to its visible text for renderLeft and renderWidth.
var renderCache map[string]string
var renderLeft int
var renderWidth int

var titleKey string
var titleText string
var shortcutWidth int
var shortcutText []string

func setupShortcuts() {
	shortcutKeys = []string{"^G", "^O", "^W", "^K", "^C", "^X", "^U", "^A", "^E", "^L"}
	shortcutLabels = []string{"Help", "Write Out", "Where Is", "Cut", "Location",
		"Exit", "Paste", "Home", "End", "Refresh"}
	renderCache = map[string]string{}
	forceRedraw = true
}

func moveTo(row int, column int) string {
	return escape + itoa(row) + ";" + itoa(column) + "H"
}

// updateScreenSize re-reads the terminal size; Swiftix has no SIGWINCH.
func updateScreenSize() {
	rows, columns := userland.WindowSize()
	if rows <= 0 || columns <= 0 {
		rows = 24
		columns = 80
	}
	// The layout needs a title, one text row, the status line, and two
	// shortcut rows; a smaller terminal clips the frame.
	if rows < 5 {
		rows = 5
	}
	if columns < 20 {
		columns = 20
	}
	if rows != screenRows || columns != screenColumns {
		forceRedraw = true
	}
	screenRows = rows
	screenColumns = columns
}

func editRows() int {
	rows := screenRows - 4
	if rows < 1 {
		return 1
	}
	return rows
}

// scrollToCursor keeps the cursor inside the edit area.
func scrollToCursor() {
	if cursorRow < topRow {
		topRow = cursorRow
	}
	if cursorRow >= topRow+editRows() {
		topRow = cursorRow - editRows() + 1
	}
	column := displayColumn(currentLine(), cursorByte)
	if column < leftColumn {
		leftColumn = column
	}
	if column >= leftColumn+screenColumns {
		leftColumn = column - screenColumns + 1
	}
}

// fit pads or truncates plain text to width columns.
func fit(text string, width int) string {
	if width <= 0 {
		return ""
	}
	count := charCount(text)
	if count > width {
		return clip(text, 0, width)
	}
	return text + spaces(width-count)
}

func titleBar() string {
	key := itoa(screenColumns) + "|" + fileName
	if modified {
		key = key + "|modified"
	}
	if key == titleKey {
		return titleText
	}
	left := "  Swiftix nano " + version
	name := fileName
	if name == "" {
		name = "New Buffer"
	}
	right := ""
	if modified {
		right = "Modified  "
	}
	width := screenColumns
	middleStart := (width - charCount(name)) / 2
	if middleStart < charCount(left)+1 {
		middleStart = charCount(left) + 1
	}
	line := fit(left, middleStart) + name
	gap := width - charCount(line) - charCount(right)
	if gap < 1 {
		titleText = inverse + fit(line, width) + normal
	} else {
		titleText = inverse + line + spaces(gap) + right + normal
	}
	titleKey = key
	return titleText
}

func statusLine() string {
	if statusMessage == "" {
		return ""
	}
	text := "[ " + statusMessage + " ]"
	if charCount(text) >= screenColumns {
		return inverse + fit(text, screenColumns) + normal
	}
	padding := (screenColumns - charCount(text)) / 2
	return spaces(padding) + inverse + text + normal
}

// shortcutRow draws half of the shortcut list: keys highlighted, labels plain.
func shortcutRow(half int) string {
	if shortcutWidth != screenColumns {
		shortcutText = []string{}
		count := len(shortcutKeys) / 2
		cell := screenColumns / count
		for first := 0; first < len(shortcutKeys); first = first + count {
			out := ""
			for i := first; i < first+count; i++ {
				if cell < 4 {
					break
				}
				out = out + inverse + shortcutKeys[i] + normal + " " +
					fit(shortcutLabels[i], cell-len(shortcutKeys[i])-1)
			}
			shortcutText = append(shortcutText, out)
		}
		shortcutWidth = screenColumns
	}
	return shortcutText[half]
}

// renderLine returns the visible part of a buffer line, cached per line text.
func renderLine(line string) string {
	if renderLeft != leftColumn || renderWidth != screenColumns || len(renderCache) > 512 {
		renderCache = map[string]string{}
		renderLeft = leftColumn
		renderWidth = screenColumns
	}
	shown, ok := renderCache[line]
	if ok {
		return shown
	}
	shown = clip(line, leftColumn, screenColumns)
	renderCache[line] = shown
	return shown
}

// setRow queues screen row (0-based) for drawing when its content changed.
func setRow(row int, content string) string {
	if !forceRedraw && shownRows[row] == content {
		return ""
	}
	shownRows[row] = content
	return moveTo(row+1, 1) + content + escape + "K"
}

// invalidateRow makes the next frame redraw a row drawn outside drawScreen.
func invalidateRow(row int) {
	if row >= 0 && row < len(shownRows) {
		shownRows[row] = "\x00"
	}
}

// drawScreen renders the editor and places the cursor.
func drawScreen() {
	updateScreenSize()
	scrollToCursor()
	out := escape + "?25l"
	if forceRedraw {
		out = out + escape + "H" + escape + "2J"
		shownRows = []string{}
		for i := 0; i < screenRows; i++ {
			shownRows = append(shownRows, "")
		}
	}
	out = out + setRow(0, titleBar())
	for row := 0; row < editRows(); row++ {
		index := topRow + row
		content := ""
		if index < len(lines) {
			content = renderLine(lines[index])
		}
		out = out + setRow(row+1, content)
	}
	out = out + setRow(screenRows-3, statusLine())
	out = out + setRow(screenRows-2, shortcutRow(0))
	out = out + setRow(screenRows-1, shortcutRow(1))
	forceRedraw = false
	cursorColumn := displayColumn(currentLine(), cursorByte) - leftColumn
	out = out + moveTo(cursorRow-topRow+2, cursorColumn+1) + escape + "?25h"
	fmt.Print(out)
}

// prompt edits a one-line answer on the status row. It returns the answer and
// false when the user cancels with ^C or Escape.
func prompt(label string, initial string) (string, bool) {
	answer := initial
	for {
		updateScreenSize()
		shown := label + ": " + answer
		visible := shown
		if charCount(shown) >= screenColumns {
			start := len(shown)
			for charCount(shown[start:]) < screenColumns-1 && start > 0 {
				start = previousCharStart(shown, start)
			}
			visible = shown[start:]
		}
		fmt.Print(moveTo(screenRows-2, 1) + inverse + fit(visible, screenColumns) + normal +
			moveTo(screenRows-2, charCount(visible)+1))
		invalidateRow(screenRows - 3)
		key := readKey()
		if key.code == keyText {
			answer = answer + key.text
		} else if key.code == keyBackspace {
			answer = answer[:previousCharStart(answer, len(answer))]
		} else if key.code == keyEnter {
			return answer, true
		} else if key.code == ctrlC || key.code == keyEscape || key.code == keyEndOfInput {
			return "", false
		}
	}
}

// askYesNo asks a question on the status row and returns "y", "n", or "" for
// cancel.
func askYesNo(question string) string {
	for {
		updateScreenSize()
		fmt.Print(moveTo(screenRows-2, 1) + inverse + fit(question+" (Y/N/^C)", screenColumns) + normal +
			moveTo(screenRows-2, charCount(question)+12))
		invalidateRow(screenRows - 3)
		key := readKey()
		if key.code == keyText && (key.text == "y" || key.text == "Y") {
			return "y"
		}
		if key.code == keyText && (key.text == "n" || key.text == "N") {
			return "n"
		}
		if key.code == ctrlC || key.code == keyEscape || key.code == keyEndOfInput {
			return ""
		}
	}
}

var helpText []string

func setupHelp() {
	helpText = []string{
		"Swiftix nano help",
		"",
		"Type to insert text. The cursor keys, Home, End, PageUp, and PageDown",
		"move around; Backspace and Delete remove characters.",
		"",
		"^G  Show this help            ^X  Exit (asks to save changes)",
		"^O  Write the file            ^W  Search forward (wraps around)",
		"^K  Cut the current line      ^U  Paste the cut lines",
		"^C  Report cursor position    ^L  Redraw the screen",
		"^A  Start of line             ^E  End of line",
		"^Y  Page up                   ^V  Page down",
		"^P ^N ^B ^F  Up, down, left, right",
		"^D  Delete the character under the cursor",
		"",
		"Differences from GNU nano: one buffer, no undo, no syntax colors,",
		"and every character occupies one column.",
		"",
		"Press any key to return to the file.",
	}
}

func showHelp() {
	updateScreenSize()
	out := escape + "H" + escape + "2J" + inverse + fit("  Swiftix nano "+version+" help", screenColumns) + normal
	for i := 0; i < len(helpText) && i+3 <= screenRows; i++ {
		out = out + moveTo(i+3, 1) + clip(helpText[i], 0, screenColumns)
	}
	fmt.Print(out)
	forceRedraw = true
	readKey()
}
