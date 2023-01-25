# Required Assets for the Gateway

> DAG and Directory HTML for HTTP gateway

## Updating

When making updates to the templates, please note the following:

1. Make your changes to the (human-friendly) source documents in the `src` directory.
2. Before testing or releasing, go to `assets/` and run `go generate .`.

## Testing

1. Make sure you have [Go](https://golang.org/dl/) installed
2. Start the test server, which lives in its own directory:

```bash
> cd test
> go run .
```

This will listen on [`localhost:3000`](http://localhost:3000/) and reload the template every time you refresh the page. Here you have two pages:

- [`localhost:3000/dag`](http://localhost:3000/dag) for the DAG template preview; and
- [`localhost:3000/directory`](http://localhost:3000/directory) for the Directory template preview.

If you get a "no such file or directory" error upon trying `go run .`, make sure you ran `go generate .` to generate the minified artifact that the test is looking for.
