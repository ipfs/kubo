package query

// NaiveFilter applies a filter to the results
func NaiveFilter(qr *Results, filter Filter) *Results {
	ch := make(chan Entry)
	go func() {
		defer close(ch)

		for e := range qr.Entries() {
			if filter.Filter(e) {
				ch <- e
			}
		}
	}()
	return ResultsWithEntriesChan(qr.Query, ch)
}

// NaiveLimit truncates the results to a given int limit
func NaiveLimit(qr *Results, limit int) *Results {
	ch := make(chan Entry)
	go func() {
		defer close(ch)

		for l := 0; l < limit; l++ {
			e, more := <-qr.Entries()
			if !more {
				return
			}
			ch <- e
		}
	}()
	return ResultsWithEntriesChan(qr.Query, ch)
}

// NaiveOffset skips a given number of results
func NaiveOffset(qr *Results, offset int) *Results {
	ch := make(chan Entry)
	go func() {
		defer close(ch)

		for l := 0; l < offset; l++ {
			<-qr.Entries() // discard
		}

		for e := range qr.Entries() {
			ch <- e
		}
	}()
	return ResultsWithEntriesChan(qr.Query, ch)
}

// NaiveOrder reorders results according to given Order.
// WARNING: this is the only non-stream friendly operation!
func NaiveOrder(qr *Results, o Order) *Results {
	e := qr.AllEntries()
	o.Sort(e)
	return ResultsWithEntries(qr.Query, e)
}

func (q Query) ApplyTo(qr *Results) *Results {
	if q.Prefix != "" {
		qr = NaiveFilter(qr, FilterKeyPrefix{q.Prefix})
	}
	for _, f := range q.Filters {
		qr = NaiveFilter(qr, f)
	}
	for _, o := range q.Orders {
		qr = NaiveOrder(qr, o)
	}
	if q.Offset != 0 {
		qr = NaiveOffset(qr, q.Offset)
	}
	if q.Limit != 0 {
		qr = NaiveLimit(qr, q.Offset)
	}
	return qr
}

func ResultEntriesFrom(keys []string, vals []interface{}) []Entry {
	re := make([]Entry, len(keys))
	for i, k := range keys {
		re[i] = Entry{Key: k, Value: vals[i]}
	}
	return re
}
