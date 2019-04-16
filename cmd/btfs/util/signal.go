// +build !wasm

package util

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// IntrHandler helps set up an interrupt handler that can
// be cleanly shut down through the io.Closer interface.
type IntrHandler struct {
	closing chan struct{}
	wg      sync.WaitGroup
}

func NewIntrHandler() *IntrHandler {
	return &IntrHandler{closing: make(chan struct{})}
}

func (ih *IntrHandler) Close() error {
	close(ih.closing)
	ih.wg.Wait()
	return nil
}

// Handle starts handling the given signals, and will call the handler
// callback function each time a signal is caught. The function is passed
// the number of times the handler has been triggered in total, as
// well as the handler itself, so that the handling logic can use the
// handler's wait group to ensure clean shutdown when Close() is called.
func (ih *IntrHandler) Handle(handler func(count int, ih *IntrHandler), sigs ...os.Signal) {
	notify := make(chan os.Signal, 1)
	signal.Notify(notify, sigs...)
	ih.wg.Add(1)
	go func() {
		defer ih.wg.Done()
		defer signal.Stop(notify)

		count := 0
		for {
			select {
			case <-ih.closing:
				return
			case <-notify:
				count++
				handler(count, ih)
			}
		}
	}()
}

func SetupInterruptHandler(ctx context.Context) (io.Closer, context.Context) {
	intrh := NewIntrHandler()
	ctx, cancelFunc := context.WithCancel(ctx)

	handlerFunc := func(count int, ih *IntrHandler) {
		switch count {
		case 1:
			fmt.Println() // Prevent un-terminated ^C character in terminal

			ih.wg.Add(1)
			go func() {
				defer ih.wg.Done()
				cancelFunc()
			}()

		default:
			fmt.Println("Received another interrupt before graceful shutdown, terminating...")
			os.Exit(-1)
		}
	}

	intrh.Handle(handlerFunc, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

	return intrh, ctx
}
