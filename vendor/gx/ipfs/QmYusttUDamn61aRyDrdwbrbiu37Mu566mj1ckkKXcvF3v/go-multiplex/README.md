# go-multiplex

A super simple stream muxing library compatible with [multiplex](http://github.com/maxogden/multiplex)

## Usage

```go
mplex := multiplex.NewMultiplex(mysocket)

s := mplex.NewStream()
s.Write([]byte("Hello World!")
s.Close()

mplex.Serve(func(s *multiplex.Stream) {
	// echo back everything received
	io.Copy(s, s)
})
```
