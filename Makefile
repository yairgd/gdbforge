# -------- Config --------
APP_NAME := chat
BIN_DIR  := bin

# Detect all commands inside cmd/
CMDS := $(notdir $(wildcard cmd/*))

# -------- Default --------
.PHONY: all
all: build

# -------- Build --------
.PHONY: build
build:
	@mkdir -p $(BIN_DIR)
	@for cmd in $(CMDS); do \
		echo "Building $$cmd..."; \
		go build -o $(BIN_DIR)/$$cmd ./cmd/$$cmd; \
	done
	@echo "Done."

# -------- Run TUI --------
.PHONY: run
run:
	go run ./cmd/tui

# -------- Debug Dlv --------
.PHONY: debug
debug:
	 dlv debug ./cmd/tui --headless --listen=:2345 --api-version=2

# -------- Test --------
.PHONY: test
test:
	go test ./...

# -------- Format --------
.PHONY: fmt
fmt:
	go fmt ./...

# -------- Vet --------
.PHONY: vet
vet:
	go vet ./...

# -------- Clean --------
.PHONY: clean
clean:
	rm -rf $(BIN_DIR)

# -------- Cross Compile Example --------
.PHONY: build-linux
build-linux:
	@mkdir -p $(BIN_DIR)
	GOOS=linux GOARCH=amd64 go build -o $(BIN_DIR)/tui-linux ./cmd/tui

.PHONY: build-mac
build-mac:
	@mkdir -p $(BIN_DIR)
	GOOS=darwin GOARCH=amd64 go build -o $(BIN_DIR)/tui-mac ./cmd/tui
