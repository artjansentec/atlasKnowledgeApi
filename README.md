# Atlas Knowledge API

API REST em Go + Echo + PostgreSQL para a wiki corporativa Atlas Knowledge.

## Pré-requisitos

- Go 1.22+
- PostgreSQL 15+ instalado localmente

## Configuração

```powershell
cd backend
copy .env.example .env
```

Ajuste `DATABASE_URL` no `.env` com usuário, senha e porta do seu Postgres local (padrão: `5432`).

### Criar o banco

No `psql` ou pgAdmin:

```sql
CREATE DATABASE atlas_knowledge;
```

### Variáveis de ambiente

| Variável | Descrição | Padrão |
|----------|-----------|--------|
| `PORT` | Porta HTTP | `8080` |
| `DATABASE_URL` | Connection string Postgres local | ver `.env.example` |
| `JWT_SECRET` | Segredo HS256 | `change-me-in-production` |
| `JWT_ACCESS_TTL` | Expiração access token | `15m` |
| `JWT_REFRESH_TTL` | Expiração refresh token | `168h` |
| `STORAGE_PATH` | Pasta de uploads locais | `./storage` |
| `MAX_UPLOAD_BYTES` | Tamanho máximo upload | `20971520` (20MB) |
| `CORS_ORIGINS` | Origens permitidas (vírgula) | `http://localhost:5173` |

**Datas JSON:** campos de data usam formato `YYYY-MM-DD` (ISO 8601, apenas data).

## Comandos

### Windows (PowerShell)

```powershell
cd backend

# 1. Aplicar migrations (cria tabelas)
go run ./cmd/migrate up

# 2. Criar o primeiro admin (somente em banco vazio)
go run ./cmd/create-admin -email seu@email.com -password SUA_SENHA

# 3. Subir a API
go run ./cmd/api

# Ou migrate + API de uma vez:
.\dev.ps1
```

### Linux / macOS

```bash
cd backend
cp .env.example .env
make migrate-up
make create-admin EMAIL=seu@email.com PASSWORD=SUA_SENHA NAME=Administrador
make run
```

## Swagger UI

Com a API rodando, abra no navegador:

**http://localhost:8080/swagger**

1. Teste o health check: `GET /api/v1/health` (não precisa de login).
2. Faça login em `POST /api/v1/auth/login` com o admin criado no passo anterior.
3. Copie o `accessToken`, clique no cadeado **Authorize** e informe: `Bearer SEU_TOKEN`.
4. Explore e teste as demais rotas.

A especificação OpenAPI também está em `GET /openapi.yaml`.

## Rotas (`/api/v1`)

| Método | Rota | Auth |
|--------|------|------|
| POST | `/auth/login` | — |
| POST | `/auth/refresh` | cookie |
| POST | `/auth/logout` | JWT |
| GET | `/auth/me` | JWT |
| GET | `/users` | JWT |
| GET | `/dashboard/summary` | JWT |
| GET | `/search?q=` | JWT |
| GET | `/projects` | JWT |
| GET | `/projects/:slug` | JWT |
| POST | `/projects` | JWT (admin) |
| PATCH | `/projects/:slug` | JWT |
| DELETE | `/projects/:slug` | JWT (admin) |
| PUT | `/projects/:slug/readers` | JWT |
| POST/PATCH/DELETE | `/projects/:slug/sections...` | JWT |
| PUT | `/projects/:slug/sections/reorder` | JWT |
| POST/PATCH/DELETE | `/projects/:slug/lessons...` | JWT |
| POST/DELETE | `/projects/:slug/attachments...` | JWT |
| GET | `/files/:fileId/download` | JWT |

## Exemplos curl

```bash
# Login
curl -c cookies.txt -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"seu@email.com","password":"SUA_SENHA"}'

# Listar projetos
curl http://localhost:8080/api/v1/projects \
  -H "Authorization: Bearer SEU_ACCESS_TOKEN"

# Criar projeto (admin)
curl -X POST http://localhost:8080/api/v1/projects \
  -H "Authorization: Bearer SEU_ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"slug":"meu-projeto","name":"Meu Projeto","description":"Descrição"}'
```

## Banco com dados antigos (Docker/seed)

Se você usou o seed anterior ou o Postgres via Docker, limpe o banco antes de usar só dados reais:

```sql
DROP DATABASE atlas_knowledge;
CREATE DATABASE atlas_knowledge;
```

Depois rode `go run ./cmd/migrate up` e `go run ./cmd/create-admin ...`.
