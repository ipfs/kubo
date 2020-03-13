package util

import "golang.org/x/sys/windows"

func InsideGUI() bool {
	conhostInfo := &windows.ConsoleScreenBufferInfo{}
	if err := windows.GetConsoleScreenBufferInfo(windows.Stdout, conhostInfo); err != nil {
		return false
	}

	if (conhostInfo.CursorPosition.X | conhostInfo.CursorPosition.Y) == 0 {
		// console cursor has not moved prior to our execution
		// high probability that we're not in a terminal
		return true
	}

	return false
}
