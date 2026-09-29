package main

// A nano-style full-screen editor for Swiftix terminals. It uses the
// swiftix/userland terminal ABI: raw mode, window size, suspending stdin
// reads, and whole-file writes.

import "fmt"
import "os"
import "swiftix/userland"

func usage() {
	fmt.Print("Usage: nano [FILE]\n")
	fmt.Print("A nano-style text editor for Swiftix. Press ^G inside nano for help.\n")
}

func main() {
	path := ""
	for i := 1; i < len(os.Args); i++ {
		argument := os.Args[i]
		if argument == "-h" || argument == "--help" {
			usage()
			os.Exit(0)
		}
		if argument == "-V" || argument == "--version" {
			fmt.Print("Swiftix nano " + version + "\n")
			os.Exit(0)
		}
		if len(argument) > 1 && argument[0] == 45 {
			fmt.Print("nano: unknown option " + argument + "\n")
			usage()
			os.Exit(2)
		}
		if path != "" {
			fmt.Print("nano: only one file can be edited at a time\n")
			os.Exit(2)
		}
		path = argument
	}
	if !userland.SetRawMode(true) {
		fmt.Print("nano: standard input is not a terminal\n")
		os.Exit(1)
	}
	setupShortcuts()
	setupHelp()
	// Enter the alternate screen before loading, so a read error printed by
	// the loader is overwritten by the first frame.
	fmt.Print(escape + "?1049h" + escape + "H" + escape + "2J")
	statusMessage = loadFile(path)
	run()
	fmt.Print(escape + "?1049l" + escape + "?25h")
	userland.SetRawMode(false)
}

// run is the edit loop; it returns when the user exits.
func run() {
	for {
		drawScreen()
		key := readKey()
		message := statusMessage
		statusMessage = ""
		cutting := false
		if key.code == keyEndOfInput {
			return
		} else if key.code == keyText {
			insertText(key.text)
		} else if key.code == keyTab {
			insertText("\t")
		} else if key.code == keyEnter {
			insertNewline()
		} else if key.code == keyBackspace {
			deleteBackward()
		} else if key.code == keyDelete || key.code == ctrlD {
			deleteForward()
		} else if key.code == keyLeft || key.code == ctrlB {
			moveLeft()
		} else if key.code == keyRight || key.code == ctrlF {
			moveRight()
		} else if key.code == keyUp || key.code == ctrlP {
			moveVertically(-1)
		} else if key.code == keyDown || key.code == ctrlN {
			moveVertically(1)
		} else if key.code == keyHome || key.code == ctrlA {
			moveHome()
		} else if key.code == keyEnd || key.code == ctrlE {
			moveEnd()
		} else if key.code == keyPageUp || key.code == ctrlY {
			moveVertically(-editRows())
		} else if key.code == keyPageDown || key.code == ctrlV {
			moveVertically(editRows())
		} else if key.code == ctrlK {
			cutLine()
			cutting = true
		} else if key.code == ctrlU {
			if !pasteCut() {
				statusMessage = "Cutbuffer is empty"
			}
		} else if key.code == ctrlO {
			writeOut()
		} else if key.code == ctrlW {
			whereIs()
		} else if key.code == ctrlC {
			statusMessage = location()
		} else if key.code == ctrlG {
			showHelp()
		} else if key.code == ctrlL {
			statusMessage = message
			forceRedraw = true
		} else if key.code == ctrlX {
			if exitEditor() {
				return
			}
		}
		lastKeyWasCut = cutting
	}
}

// writeOut asks for a file name and saves the buffer.
func writeOut() {
	name, ok := prompt("File Name to Write", fileName)
	if !ok || name == "" {
		statusMessage = "Cancelled"
		return
	}
	message, _ := saveFile(name)
	statusMessage = message
}

// exitEditor offers to save a modified buffer and reports whether to exit.
func exitEditor() bool {
	if !modified {
		return true
	}
	answer := askYesNo("Save modified buffer?")
	if answer == "n" {
		return true
	}
	if answer == "" {
		statusMessage = "Cancelled"
		return false
	}
	name, ok := prompt("File Name to Write", fileName)
	if !ok || name == "" {
		statusMessage = "Cancelled"
		return false
	}
	message, saved := saveFile(name)
	statusMessage = message
	return saved
}

// whereIs searches forward for a string, offering the previous search.
func whereIs() {
	label := "Search"
	if lastSearch != "" {
		label = "Search [" + lastSearch + "]"
	}
	query, ok := prompt(label, "")
	if !ok {
		statusMessage = "Cancelled"
		return
	}
	if query == "" {
		query = lastSearch
	}
	if query == "" {
		statusMessage = "Cancelled"
		return
	}
	lastSearch = query
	startRow := cursorRow
	startByte := cursorByte
	found, wrapped := searchForward(query)
	if !found {
		statusMessage = "\"" + query + "\" not found"
	} else if cursorRow == startRow && cursorByte == startByte {
		statusMessage = "This is the only occurrence"
	} else if wrapped {
		statusMessage = "Search Wrapped"
	}
}

// location describes the cursor like nano's ^C.
func location() string {
	line := currentLine()
	column := charCount(line[:cursorByte]) + 1
	width := charCount(line) + 1
	return "line " + itoa(cursorRow+1) + "/" + itoa(len(lines)) + ", col " + itoa(column) + "/" + itoa(width)
}
