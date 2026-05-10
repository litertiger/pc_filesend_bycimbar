# ─── cimbar-recv-pc build targets ─────────────────────────────────────────────
BINARY   := cimbar-recv-pc
OUT_DIR  := release

# Embed version from git tag; fall back to "dev" if not in a git repo.
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X main.version=$(VERSION)

.PHONY: all windows clean

## all: build Windows amd64 release binary (default target)
all: windows

## windows: cross-compile for Windows 10 amd64 → release/cimbar-recv-pc.exe
windows:
	@mkdir -p $(OUT_DIR)
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
		go build -trimpath -ldflags "$(LDFLAGS)" \
		-o $(OUT_DIR)/$(BINARY).exe .
	@ls -lh $(OUT_DIR)/$(BINARY).exe
	@echo "[OK] build complete"

## clean: remove compiled binaries from release/
clean:
	rm -f $(OUT_DIR)/*.exe
	@echo "[OK] release/ cleaned"
