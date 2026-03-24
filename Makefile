BIN_DIR := ./bin
SKILL_VALIDATOR ?= python3 ./scripts/validate_skill.py

.PHONY: build test install-codex install-claude validate-skill

build:
	go build -o $(BIN_DIR)/sshctl ./cmd/sshctl

test:
	go test ./...

install-codex:
	./install/install-codex.sh

install-claude:
	./install/install-claude.sh

validate-skill:
	$(SKILL_VALIDATOR) ./skills/ssh-ops
