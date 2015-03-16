# datastore interface

datastore is a generic layer of abstraction for data store and database access. It is a simple API with the aim to enable application development in a datastore-agnostic way, allowing datastores to be swapped seamlessly without changing application code. Thus, one can leverage different datastores with different strengths without committing the application to one datastore throughout its lifetime.

In addition, grouped datastores significantly simplify interesting data access patterns (such as caching and sharding).

Based on [datastore.py](https://github.com/datastore/datastore).

### Documentation

https://godoc.org/github.com/jbenet/go-datastore

### License

MIT
