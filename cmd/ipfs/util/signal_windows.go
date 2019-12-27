package util

import (
	"os"

	"golang.org/x/sys/windows"
)

type (
	DWORD    = uint32
	MSVCBOOL = uintptr
)

const (
	processed = iota
	notProcessed
)

func init() {
	nativeNotifier = windowEventHandler
}

// windowEventHandler creates and registers a WINAPI `HandlerRoutine` callback
// which receives window events and relays them as interrupt signals to the provided channel
func windowEventHandler(ih *IntrHandler, sigChan chan os.Signal, sigs ...os.Signal) {
	// we don't want to relay the events `signal` is already listening for
	// otherwise the channel will receive them twice
	var goIsHandlingIntterupts bool
	for _, sig := range sigs {
		if sig == os.Interrupt {
			goIsHandlingIntterupts = true
		}
	}

	fp := func(ctrlType DWORD) MSVCBOOL {
		switch ctrlType {
		case windows.CTRL_CLOSE_EVENT:
			//NOTE: the OS will terminate our process after receiving this event
			// either after `SPI_GETHUNGAPPTIMEOUT` or immediately after we return non-0
			// (whichever is first)
			sigChan <- os.Interrupt
			ih.wg.Wait()
			return processed
		case windows.CTRL_C_EVENT, windows.CTRL_BREAK_EVENT:
			if !goIsHandlingIntterupts {
				sigChan <- os.Interrupt
			}
			return processed

		default:
			// we didn't expect this event
			// send it to the next handler
			return notProcessed
		}
	}

	// register the callback with the OS
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	setConsoleCtrlHandler := kernel32.NewProc("SetConsoleCtrlHandler")

	setConsoleCtrlHandler.Call(windows.NewCallback(fp), 1)
}
