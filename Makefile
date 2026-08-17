.PHONY: run run-api migrate-up migrate-down create-admin test lint docker-build docker-up docker-up-db docker-down docker-logs help

MIGRATE ?= migrate
DB_URL ?= $(shell grep DATABASE_URL .env 2>/dev/null | cut -d= -f2-)
MIGRATIONS_PATH ?= internal/db/migrations

# Cores ANSI
C_RESET  := \033[0m
C_BOLD   := \033[1m
C_DIM    := \033[2m
C_CYAN   := \033[36m
C_GREEN  := \033[32m
C_YELLOW := \033[33m
C_RED    := \033[31m
C_MAGENTA:= \033[35m
C_BLUE   := \033[34m

define say
	printf "$(C_CYAN)$(C_BOLD)▶$(C_RESET) $(C_BOLD)%s$(C_RESET)\n" "$(1)"
endef

define ok
	printf "$(C_GREEN)$(C_BOLD)✔$(C_RESET) %s\n" "$(1)"
endef

define warn
	printf "$(C_YELLOW)$(C_BOLD)⚠$(C_RESET) %s\n" "$(1)"
endef

define fail
	printf "$(C_RED)$(C_BOLD)✖$(C_RESET) %s\n" "$(1)"
endef

# Linux/macOS — make run aplica migrations, garante admin inicial e sobe a API
# Windows: .\dev.ps1  (ou .\dev.ps1 -ApiOnly para pular migrations)

help:
	@printf "\n"
	@printf "$(C_CYAN)$(C_BOLD)  Atlas Knowledge API$(C_RESET) $(C_DIM)— comandos make$(C_RESET)\n"
	@printf "$(C_DIM)  ────────────────────────────────────$(C_RESET)\n"
	@printf "  $(C_GREEN)make run$(C_RESET)           sobe a API\n"
	@printf "  $(C_GREEN)make run-api$(C_RESET)       sobe a API (alias)\n"
	@printf "  $(C_YELLOW)make lint$(C_RESET)          golangci-lint com cores\n"
	@printf "  $(C_YELLOW)make test$(C_RESET)          roda os testes\n"
	@printf "  $(C_BLUE)make migrate-up$(C_RESET)    aplica migrations\n"
	@printf "  $(C_BLUE)make migrate-down$(C_RESET)  reverte 1 migration\n"
	@printf "  $(C_MAGENTA)make create-admin$(C_RESET)  cria admin (EMAIL/PASSWORD/NAME)\n"
	@printf "  $(C_CYAN)make docker-up$(C_RESET)     sobe a API (banco externo via DATABASE_URL)\n"
	@printf "  $(C_CYAN)make docker-up-db$(C_RESET)  sobe API + Postgres interno\n"
	@printf "  $(C_CYAN)make docker-down$(C_RESET)   para os containers\n"
	@printf "  $(C_CYAN)make docker-logs$(C_RESET)   acompanha logs da API\n"
	@printf "\n"

run:
	@$(call say,subindo Atlas Knowledge API…)
	go run ./cmd/api

run-api:
	@$(call say,subindo Atlas Knowledge API…)
	go run ./cmd/api

migrate-up:
	@$(call say,aplicando migrations…)
	go run ./cmd/migrate up
	@$(call ok,migrations aplicadas)

migrate-down:
	@$(call say,revertendo migration…)
	go run ./cmd/migrate down
	@$(call ok,migration revertida)

create-admin:
	@$(call say,criando admin…)
	go run ./cmd/create-admin -email $(EMAIL) -password $(PASSWORD) -name "$(NAME)"
	@$(call ok,admin pronto)

test:
	@$(call say,rodando testes…)
	@go test ./... -count=1 && $(call ok,testes ok) || ( $(call fail,testes falharam); exit 1 )

lint:
	@$(call say,golangci-lint…)
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		$(call warn,golangci-lint não instalado — pulando); \
		printf "  $(C_DIM)instale: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest$(C_RESET)\n"; \
	else \
		printf "$(C_DIM)  analisando ./...$(C_RESET)\n"; \
		if golangci-lint run ./... --color always; then \
			$(call ok,lint limpo — nada a corrigir); \
		else \
			code=$$?; \
			$(call fail,lint encontrou problemas); \
			exit $$code; \
		fi; \
	fi

docker-build:
	@$(call say,construindo imagem Docker…)
	docker compose build

docker-up:
	@$(call say,subindo stack de produção…)
	@if [ ! -f .env ]; then \
		$(call fail,.env ausente — copie .env.production.example para .env e preencha os valores); \
		exit 1; \
	fi
	docker compose up -d --build
	@$(call ok,stack no ar — API em http://localhost:$${PORT:-8080})

docker-up-db:
	@$(call say,subindo API + Postgres interno…)
	@if [ ! -f .env ]; then \
		$(call fail,.env ausente — copie .env.production.example para .env e preencha os valores); \
		exit 1; \
	fi
	docker compose --profile db up -d --build
	@$(call ok,stack no ar — API em http://localhost:$${PORT:-8080})

docker-down:
	@$(call say,parando containers…)
	docker compose down
	@$(call ok,stack parado)

docker-logs:
	docker compose logs -f api
