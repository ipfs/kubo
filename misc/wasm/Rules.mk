include mk/header.mk

# TODO: target disabling all incompatible plugins

$(d)/ipfs.wasm: deps
	GOOS=js GOARCH=wasm $(GOCC) build -tags="nofuse purego" -ldflags="-X "github.com/ipfs/go-ipfs".CurrentCommit=47e9466ac" -o "$@" "github.com/ipfs/go-ipfs/misc/wasm"

.PHONY: $(d)/ipfs.wasm

$(d)/serve:	$(d)/ipfs.wasm
	go get github.com/shurcooL/goexec
	@echo 'Listening on http://127.0.0.1:8000'
	@(cd $(@D) && goexec 'http.ListenAndServe("127.0.0.1:8000", http.FileServer(http.Dir(".")))')

.PHONY: $(d)/serve

include mk/footer.mk