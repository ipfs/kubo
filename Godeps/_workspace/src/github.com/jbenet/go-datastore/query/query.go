package query

/*
Query represents storage for any key-value pair.

tl;dr:

  queries are supported across datastores.
  Cheap on top of relational dbs, and expensive otherwise.
  Pick the right tool for the job!

In addition to the key-value store get and set semantics, datastore
provides an interface to retrieve multiple records at a time through
the use of queries. The datastore Query model gleans a common set of
operations performed when querying. To avoid pasting here years of
database research, let’s summarize the operations datastore supports.

Query Operations:

  * namespace - scope the query, usually by object type
  * filters - select a subset of values by applying constraints
  * orders - sort the results by applying sort conditions
  * limit - impose a numeric limit on the number of results
  * offset - skip a number of results (for efficient pagination)

datastore combines these operations into a simple Query class that allows
applications to define their constraints in a simple, generic, way without
introducing datastore specific calls, languages, etc.

Of course, different datastores provide relational query support across a
wide spectrum, from full support in traditional databases to none at all in
most key-value stores. Datastore aims to provide a common, simple interface
for the sake of application evolution over time and keeping large code bases
free of tool-specific code. It would be ridiculous to claim to support high-
performance queries on architectures that obviously do not. Instead, datastore
provides the interface, ideally translating queries to their native form
(e.g. into SQL for MySQL).

However, on the wrong datastore, queries can potentially incur the high cost
of performing the aforemantioned query operations on the data set directly in
Go. It is the client’s responsibility to select the right tool for the job:
pick a data storage solution that fits the application’s needs now, and wrap
it with a datastore implementation. As the needs change, swap out datastore
implementations to support your new use cases. Some applications, particularly
in early development stages, can afford to incurr the cost of queries on non-
relational databases (e.g. using a FSDatastore and not worry about a database
at all). When it comes time to switch the tool for performance, updating the
application code can be as simple as swapping the datastore in one place, not
all over the application code base. This gain in engineering time, both at
initial development and during later iterations, can significantly offset the
cost of the layer of abstraction.

*/
type Query struct {
	Prefix   string   // namespaces the query to results whose keys have Prefix
	Filters  []Filter // filter results. apply sequentially
	Orders   []Order  // order results. apply sequentially
	Limit    int      // maximum number of results
	Offset   int      // skip given number of results
	KeysOnly bool     // return only keys.
}

// NotFetched is a special type that signals whether or not the value
// of an Entry has been fetched or not. This is needed because
// datastore implementations get to decide whether Query returns values
// or only keys. nil is not a good signal, as real values may be nil.
var NotFetched = struct{}{}

// Entry is a query result entry.
type Entry struct {
	Key   string // cant be ds.Key because circular imports ...!!!
	Value interface{}
}

// Results is a set of Query results
type Results struct {
	Query Query // the query these Results correspond to

	done chan struct{}
	res  chan Entry
	all  []Entry
}

// ResultsWithEntriesChan returns a Results object from a
// channel of ResultEntries. It's merely an encapsulation
// that provides for AllEntries() functionality.
func ResultsWithEntriesChan(q Query, res <-chan Entry) *Results {
	r := &Results{
		Query: q,
		done:  make(chan struct{}),
		res:   make(chan Entry),
		all:   []Entry{},
	}

	// go consume all the results and add them to the results.
	go func() {
		for e := range res {
			r.all = append(r.all, e)
			r.res <- e
		}
		close(r.res)
		close(r.done)
	}()
	return r
}

// ResultsWithEntries returns a Results object from a
// channel of ResultEntries. It's merely an encapsulation
// that provides for AllEntries() functionality.
func ResultsWithEntries(q Query, res []Entry) *Results {
	r := &Results{
		Query: q,
		done:  make(chan struct{}),
		res:   make(chan Entry),
		all:   res,
	}

	// go add all the results
	go func() {
		for _, e := range res {
			r.res <- e
		}
		close(r.res)
		close(r.done)
	}()
	return r
}

// Entries() returns results through a channel.
// Results may arrive at any time.
// The channel may or may not be buffered.
// The channel may or may not rate limit the query processing.
func (r *Results) Entries() <-chan Entry {
	return r.res
}

// AllEntries returns all the entries in Results.
// It blocks until all the results have come in.
func (r *Results) AllEntries() []Entry {
	for e := range r.res {
		_ = e
	}
	<-r.done
	return r.all
}
