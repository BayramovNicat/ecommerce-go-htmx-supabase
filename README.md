# HTMX E-Commerce Shop

High-performance e-commerce site built with Go, HTMX, and Supabase.

## Tech Stack

- **Backend:** Go (net/http, pgx)
- **Frontend:** HTMX, Alpine.js, Tailwind CSS
- **Database:** Supabase (Postgres)
- **Hosting:** Vercel Serverless Functions

## Setup

1. Install dependencies:

```bash
go mod download
bun install
```

2. Set up Supabase:
   - Create a new Supabase project
   - Run the SQL schema from `supabase_schema.sql`
   - Get your database connection string

3. Configure environment variables in Vercel:

```
SUPABASE_DB_URL=postgresql://...
SUPABASE_JWT_SECRET=your-jwt-secret
SUPABASE_URL=https://your-project.supabase.co
SUPABASE_ANON_KEY=your-anon-key
```

4. Build frontend assets:

```bash
bun run build
```

5. Deploy to Vercel:

```bash
vercel
```

## Project Structure

```
htmxshop/
├── api/
│   └── index.go              # Vercel serverless entry point
├── cmd/
│   └── server/               # Local development server
│       └── main.go
├── internal/
│   ├── database/             # Database layer
│   │   └── db.go            # Connection pool, models & queries
│   ├── middleware/           # HTTP middleware
│   │   └── auth.go          # Authentication & authorization
│   ├── handlers/             # HTTP handlers by domain
│   │   ├── shop/            # Public shop handlers
│   │   │   ├── handlers.go
│   │   │   └── auth_handlers.go
│   │   └── admin/           # Admin dashboard handlers
│   │       └── handlers.go
│   └── services/             # Business logic layer (future)
├── web/
│   ├── static/               # Source assets
│   │   ├── css/
│   │   │   └── styles.css   # Tailwind input
│   │   └── js/
│   │       └── bundle.js    # Frontend JS entrypoint
│   ├── templates/            # HTML templates
│   │   ├── layouts/
│   │   │   └── base.html    # Base layout
│   │   ├── shop/            # Shop templates
│   │   │   ├── home.html
│   │   │   ├── product.html
│   │   │   ├── search.html
│   │   │   ├── login.html
│   │   │   └── oauth-callback.html
│   │   └── admin/           # Admin templates
│   │       ├── dashboard.html
│   │       ├── products.html
│   │       └── orders.html
│   ├── dist/                 # Built assets (gitignored)
│   │   ├── bundle.js
│   │   └── styles.css
│   └── templates.go          # Go embed directive
├── migrations/               # Database migrations
│   └── 001_initial_schema.sql
├── scripts/                  # Build & deployment scripts
├── .env.example              # Environment variables template
├── go.mod                    # Go dependencies
├── package.json              # Node.js dependencies
└── vercel.json               # Vercel configuration
```

## Performance Features

- Keyset pagination (cursor-based, no OFFSET)
- Postgres Full-Text Search with GIN indexes
- Covering indexes for list queries
- Cache-Control headers with stale-while-revalidate
- HTMX infinite scroll with pre-fetch buffer
- Lazy-loaded WebP images
- content-visibility: auto for DOM optimization

## Admin Access

Create an admin user by inserting into the `admin_users` table:

```sql
INSERT INTO admin_users (id, email, is_admin)
VALUES ('user-uuid-from-auth-users', 'admin@example.com', true);
```
