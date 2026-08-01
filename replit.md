# open-coreui backend

A Go re-implementation of the [Open WebUI](https://github.com/open-webui/open-webui) backend. It proxies requests to the Python Open WebUI service while adding a Go-native HTTP layer, SQLite/Postgres persistence, DB migrations, and a SvelteKit static-file server.

## Stack

- **Language:** Go 1.21
- **Entry point:** `cmd/openwebui/main.go`
- **Database:** SQLite (default) or PostgreSQL via `DATABASE_URL`
- **Frontend:** served from `FRONTEND_DIR` (SvelteKit static build)

## Running locally

```bash
go run ./cmd/openwebui
```

The server starts on `:8081` by default. Override with `OPEN_COREUI_GO_ADDR`.

## Key environment variables

| Variable | Default | Description |
|---|---|---|
| `OPEN_COREUI_GO_ADDR` | `:8081` | Listen address |
| `OPEN_COREUI_PYTHON_BASE_URL` | — | URL of the Python Open WebUI backend to proxy |
| `DATABASE_URL` | SQLite in `DATA_DIR` | Postgres connection string (optional) |
| `DATA_DIR` | `data/` | Directory for SQLite DB and uploads |
| `FRONTEND_DIR` | `open-webui/build` | Path to compiled SvelteKit frontend |
| `WEBUI_SECRET_KEY` | — | **Required in production** – JWT signing key |
| `ENABLE_SIGNUP` | `true` | Allow new user registrations |
| `DEFAULT_USER_ROLE` | `pending` | Role assigned to new users |

## Deploy on Render

1. Push the repo to GitHub.
2. In the Render dashboard choose **New → Blueprint** and point it at this repo.  
   Render will detect `render.yaml` and create the service automatically.
3. Set any additional env vars (e.g. `OPEN_COREUI_PYTHON_BASE_URL`) in the Render dashboard.

The `render.yaml` configures:
- Docker build (multi-stage, `Dockerfile`)
- A 10 GB persistent disk mounted at `/app/data` for SQLite & uploads
- Auto-generated `WEBUI_SECRET_KEY`
- `PORT` forwarded via `OPEN_COREUI_GO_ADDR`

## User preferences

<!-- Add any project-specific preferences here -->
