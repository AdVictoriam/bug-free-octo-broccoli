.PHONY: build run test check pdf-check browser-check
build:
	mkdir -p bin
	CGO_ENABLED=1 go build -trimpath -o bin/bolty ./cmd/bolty
run: build
	python3 tools/run.py
test:
	go test -race ./...
check:
	go vet ./...
	node --check web/app.js
	python3 -m py_compile tools/*.py tests/*.py
pdf-check:
	python3 tests/pdf_smoke.py
browser-check: build
	python3 tests/browser_smoke.py --binary bin/bolty
