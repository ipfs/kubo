build:
	go build

deps:
	go get ./...

watch:
	-make
	@echo "[watching *.go; for recompilation]"
	# for portability, use watchmedo -- pip install watchmedo
	@watchmedo shell-command --patterns="*.go;" --recursive \
		--command='make' .
