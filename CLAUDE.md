# CLAUDE.md — htmxshop

Go + HTMX e-commerce storefront deployed on Vercel serverless. Go handles all routing and HTML rendering; HTMX drives interactivity without a JS framework build step; Tailwind CSS 4 handles styling.

---

## Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go 1.22, `net/http` (no framework) |
| Database | Supabase (PostgreSQL), pgx/v5 pooling |
| Auth | Supabase JWT (local HMAC-SHA256 verify + API fallback) |
| Frontend | HTMX 2.0.8 + Alpine.js 3.15.11 |
| CSS | Tailwind CSS 4.2.2 (compiled via CLI) |
| JS bundler | esbuild |
| Deployment | Vercel serverless (fra1 region) |

---

## Project Structure

```
htmxshop/
├── api/
│   └── index.go                # Vercel entry point — main router, lazy DB init
├── cmd/
│   ├── server/main.go          # Dev server with WebSocket live reload
│   └── migrate/main.go         # Migration runner
├── internal/
│   ├── database/db.go          # All DB models + query functions
│   ├── middleware/auth.go      # JWT verify, admin check, token cache
│   └── handlers/
│       ├── shop/handlers.go    # Home, products list, search, product detail
│       ├── shop/auth_handlers.go # Login, Google OAuth callback
│       └── admin/handlers.go   # Dashboard, products CRUD, orders
├── web/
│   ├── static/
│   │   ├── css/styles.css      # Tailwind input + CSS custom properties
│   │   └── js/bundle.js        # HTMX + Alpine init, cart utilities
│   ├── templates/
│   │   ├── layouts/base.html   # Shared layout: header, footer, live reload
│   │   ├── shop/               # home, product, search, cart, login templates
│   │   └── admin/              # dashboard, products, orders templates
│   ├── templates.go            # Template parsing and dev/prod switching
│   └── template_cache.go       # sync.Map cache + critical CSS embedding
├── migrations/
│   ├── 001_initial_schema.sql
│   ├── 002_seed_products.sql   # 5 000 fake products
│   └── 003_add_image_columns.sql
├── vercel.json
├── package.json
├── go.mod
└── .env.example
```

---

## Commands

```bash
# Development (Go file watcher + live reload on :8080)
npm run dev

# Build CSS + JS for production
npm run build

# Build individually
npm run build:css   # Tailwind → web/dist/styles.css
npm run build:js    # esbuild → web/dist/bundle.js

# Watch mode (CSS + JS concurrently, no Go restart)
npm run watch

# Run migrations (connects to Supabase, runs SQL files in order)
go run ./cmd/migrate

# Format templates and JS with Prettier
npm run format

# Start without nodemon
npm run start
```

---

## Environment Variables

Copy `.env.example` to `.env` before running locally.

| Variable | Purpose |
|---|---|
| `SUPABASE_DB_URL` | Postgres connection string via Supabase pooler |
| `SUPABASE_JWT_SECRET` | Enables fast local JWT verification (skip Supabase API call) |
| `SUPABASE_URL` | Supabase project API base URL |
| `SUPABASE_ANON_KEY` | Public anon key (used in Supabase Auth UI) |
| `SUPABASE_SERVICE_ROLE_KEY` | Admin key for server-side Supabase client |
| `ENV` | `production` embeds static files; anything else serves from disk |
| `PORT` | Dev server port (default 8080) |

---

## Architecture Decisions

### Routing
`api/index.go` is a single `http.Handler` wired to all routes. In dev (`cmd/server/main.go`) the same handler runs behind a standard `http.Server`. There is no framework — routes are matched with `r.URL.Path` and method checks.

### HTMX rendering pattern
Every handler checks the `HX-Request` header. On a full-page request it renders the base layout. On an HTMX request it renders only the `page_root` fragment or a specific partial. Template target logic lives in each handler — keep it there, not in templates.

### Database access
All DB access goes through `internal/database/db.go`. No ORM. Queries use `pgx/v5` with `$1`-style params. The connection pool (`*pgxpool.Pool`) is a package-level singleton, lazily initialized on first request (important for Vercel cold starts). Max 5 connections — respect this limit for serverless.

### Pagination
Keyset pagination everywhere. Home page cursor = highest `id` seen. Do **not** use OFFSET — the `idx_products_keyset` index is built for cursor-based access only.

### Auth flow
1. Token arrives in `Authorization` header or `sb-access-token` cookie.
2. `VerifySupabaseToken` checks local HMAC-SHA256 if `SUPABASE_JWT_SECRET` is set (fast path).
3. Falls back to Supabase `/auth/v1/user` API call (cached 5 min).
4. Admin routes additionally query `admin_users` table.

### Static assets
In production, `web/dist/` is embedded into the binary via `//go:embed`. Vercel serves `/dist/*` with immutable cache headers (1 year). In dev, files are served from disk so changes are reflected without restart.

### Cart
Cart state lives entirely in `localStorage` under the key `"cart"`. There is no server-side cart — `cart.html` is a client-rendered page driven by Alpine.js reading localStorage.

---

## Key Patterns

### Adding a new shop route
1. Add handler function to `internal/handlers/shop/handlers.go`.
2. Register route in `api/index.go` `shopHandler` switch.
3. Create template in `web/templates/shop/`.
4. Add template cache key in `web/templates.go` if needed.

### Adding a new admin route
Same as above but in `internal/handlers/admin/handlers.go` and `admin/` templates. The admin middleware (`VerifyAdminAccess`) is applied at the router level — no need to re-check inside handlers.

### Template data
Handlers pass `map[string]interface{}` to templates. User data comes from `getUserFromRequest()` which parses the JWT cookie. Keep template data keys consistent — use lowercase snake_case keys.

### Cache invalidation
`internal/handlers/shop/handlers.go` holds an in-memory product cache (`sync.RWMutex` + TTL). After any admin mutation (create/update/delete product), call the cache invalidation helper to clear stale entries.

---

## Database Schema (summary)

```sql
products       -- id, uuid, name, slug, description, price, stock,
               -- image_thumb, image_full, is_active, search_vector (generated)
orders         -- id, uuid, user_id, email, total, status
order_items    -- id, order_id, product_id, product_name, quantity, price
admin_users    -- id (FK auth.users), email, is_admin
```

**Indexes used at runtime:**
- `idx_products_keyset` — `(id DESC) WHERE is_active` — home page pagination
- `idx_products_search` — GIN on `search_vector` — full-text search
- `idx_products_slug` — `(slug) WHERE is_active` — product detail
- `idx_products_list_covering` — covering index for list queries

RLS is enabled. Public reads active products. Only `admin_users` rows can mutate products or view all orders.

---

## Deployment

Vercel config (`vercel.json`):
- Region: `fra1`
- Build: `bash scripts/vercel-build.sh` (runs `npm run build`, then `go build`)
- `/dist/*` → immutable 1-year cache
- Everything else → `api/index.go`

Product pages get `Cache-Control: public, max-age=300, stale-while-revalidate=600` via Vercel headers config.

---

## What to Watch Out For

- **No test suite exists yet.** Any new logic should be accompanied by tests in `_test.go` files.
- **`console.log` in JS** — only in `web/static/js/bundle.js` (dev utility). Remove before shipping new JS code.
- **Template parsing errors** are silent in production (cached templates). If a template change doesn't appear, check for parse errors in dev first.
- **pgx prepared statement mode** is disabled (`prefer_simple_protocol=true`) because Supabase PgBouncer runs in session/transaction mode. Do not switch to extended query protocol without testing against the pooler.
- **`ENV=production`** triggers `//go:embed` — the binary must be built after `npm run build` so `web/dist/` is populated before embedding.
