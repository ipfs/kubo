# dir-index-html

> Directory listing HTML for `go-ipfs` gateways

![](https://user-images.githubusercontent.com/157609/88379209-ce6f0600-cda2-11ea-9620-20b9237bb441.png)

## Updating

When making updates to the directory listing page template, please note the following:

1. Make your changes to the (human-friendly) source documents in the `src` directory and run `npm run build`
3. Before testing or releasing, go to the top-level `./assets` directory and make sure to run the `go generate .` script to update the bindata version

## Testing

1. Make sure you have [Go](https://golang.org/dl/) installed
2. Start the test server, which lives in its own directory:

```bash
> cd test
> go run .
```
This will listen on [`localhost:3000`](http://localhost:3000/) and reload the template every time you refresh the page.

If you get a "no such file or directory" error upon trying `go run .`, make sure you ran `npm run build` to generate the minified artifact that the test is looking for.

