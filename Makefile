# -------- Config --------
APP_NAME := gdbforge
BIN_DIR  := bin

# Detect all commands inside cmd/
CMDS := $(notdir $(wildcard cmd/*))

# -------- Default --------
.PHONY: all
all: build


GOFILES := $(shell find . -name '*.go')

$(BIN_DIR)/gdbforge: $(GOFILES)
	@mkdir -p $(BIN_DIR)
	go build -gcflags="all=-N -l" -o $@ ./cmd/gdbforge 

build: $(BIN_DIR)/gdbforge
	
# -------- Build --------
.PHONY: build1
build1: 
	@mkdir -p $(BIN_DIR)
	@for cmd in $(CMDS); do \
		echo "Building $$cmd..."; \
		go build -o $(BIN_DIR)/$$cmd -gcflags="all=-N -l"  ./cmd/$$cmd; \
	done
	@echo "Done."

# -------- Run --------
.PHONY: run
run:
	go run ./cmd/gdbforge ./hello

# -------- Debug Dlv --------
.PHONY: debug
debug:
	dlv debug ./cmd/gdbforge --headless --listen=:2345 --api-version=2

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
	GOOS=linux GOARCH=amd64 go build -o $(BIN_DIR)/gdbforge-linux ./cmd/gdbforge

.PHONY: build-mac
build-mac:
	@mkdir -p $(BIN_DIR)
	GOOS=darwin GOARCH=amd64 go build -o $(BIN_DIR)/gdbforge-mac ./cmd/gdbforge
