# -------- Config --------
APP_NAME := xgdb
BIN_DIR  := bin

# Detect all commands inside cmd/
CMDS := $(notdir $(wildcard cmd/*))

# -------- Default --------
.PHONY: all
all: build


GOFILES := $(shell find . -name '*.go')

$(BIN_DIR)/xgdb: $(GOFILES)
	@mkdir -p $(BIN_DIR)
	go build -gcflags="all=-N -l" -o $@ ./cmd/xgdb 

build: $(BIN_DIR)/xgdb
	
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
	go run ./cmd/xgdb ./hello

# -------- Debug Dlv --------
.PHONY: debug
debug:
	dlv debug ./cmd/xgdb --headless --listen=:2345 --api-version=2

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
	GOOS=linux GOARCH=amd64 go build -o $(BIN_DIR)/xgdb-linux ./cmd/xgdb

.PHONY: build-mac
build-mac:
	@mkdir -p $(BIN_DIR)
	GOOS=darwin GOARCH=amd64 go build -o $(BIN_DIR)/xgdb-mac ./cmd/xgdb
