# ContextGroup


- Godoc: https://godoc.org/github.com/jbenet/go-ctxgroup

ContextGroup is an interface for services able to be opened and closed.
It has a parent Context, and Children. But ContextGroup is not a proper
"tree" like the Context tree. It is more like a Context-WaitGroup hybrid.
It models a main object with a few children objects -- and, unlike the
context -- concerns itself with the parent-child closing semantics:

- Can define an optional TeardownFunc (func() error) to be run at Closetime.
- Children call Children().Add(1) to be waited upon
- Children can select on <-Closing() to know when they should shut down.
- Close() will wait until all children call Children().Done()
- <-Closed() signals when the service is completely closed.

ContextGroup can be embedded into the main object itself. In that case,
the teardownFunc (if a member function) has to be set after the struct
is intialized:

```Go
type service struct {
  ContextGroup
  net.Conn
}
func (s *service) close() error {
  return s.Conn.Close()
}
func newService(ctx context.Context, c net.Conn) *service {
  s := &service{c}
  s.ContextGroup = NewContextGroup(ctx, s.close)
  return s
}
```
