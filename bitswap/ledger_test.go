package bitswap

import (
	"sync"
	"testing"
)

func TestRaceConditions(t *testing.T) {
	const numberOfExpectedExchanges = 10000
	l := new(ledger)
	var wg sync.WaitGroup
	for i := 0; i < numberOfExpectedExchanges; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			l.ReceivedBytes(1)
		}()
	}
	wg.Wait()
	if l.ExchangeCount() != numberOfExpectedExchanges {
		t.Fail()
	}
}
