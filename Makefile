BINARY := kubectl-neighbours

# Standard user-level completion dirs; override any of them like:
#   make install-completions ZSH_COMPLETIONS_DIR=~/.zfunc
BASH_COMPLETIONS_DIR ?= $(HOME)/.local/share/bash-completion/completions
FISH_COMPLETIONS_DIR ?= $(HOME)/.config/fish/completions
# brew's site-functions is already on fpath for Homebrew zsh setups;
# otherwise fall back to ~/.zsh/completions (must be added to fpath manually).
BREW_PREFIX := $(shell brew --prefix 2>/dev/null)
ifeq ($(BREW_PREFIX),)
ZSH_COMPLETIONS_DIR ?= $(HOME)/.zsh/completions
else
ZSH_COMPLETIONS_DIR ?= $(BREW_PREFIX)/share/zsh/site-functions
endif

.PHONY: build test lint fmt snapshot clean install-completions

build:
	go build -o $(BINARY) .

test:
	go test -race -coverprofile=coverage.out ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .

snapshot:
	goreleaser release --snapshot --clean

clean:
	rm -rf dist/ $(BINARY) coverage.out

install-completions: build
	mkdir -p $(BASH_COMPLETIONS_DIR) $(FISH_COMPLETIONS_DIR) $(ZSH_COMPLETIONS_DIR)
	./$(BINARY) completion bash > $(BASH_COMPLETIONS_DIR)/$(BINARY)
	./$(BINARY) completion fish > $(FISH_COMPLETIONS_DIR)/$(BINARY).fish
	./$(BINARY) completion zsh > $(ZSH_COMPLETIONS_DIR)/_$(BINARY)
	@echo "Completions installed:"
	@echo "  bash: $(BASH_COMPLETIONS_DIR)/$(BINARY)"
	@echo "  fish: $(FISH_COMPLETIONS_DIR)/$(BINARY).fish"
	@echo "  zsh:  $(ZSH_COMPLETIONS_DIR)/_$(BINARY) (ensure this dir is on your fpath)"
	@echo "Restart your shell (or rehash/compinit) to pick them up."
