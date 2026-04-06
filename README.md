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
│   └── index.go              # Main Vercel entry point
├── cmd/server/               # Local dev server
├── dist/                     # Built frontend assets (gitignored)
├── internal/
│   ├── db/                   # Database connection & queries
│   ├── shop/                 # Shop logic
│   └── admin/                # Admin logic
├── ui/
│   ├── bundle.js             # Frontend JS entrypoint
│   ├── styles.css            # Tailwind input
│   ├── shop/                 # Shop templates
│   └── admin/                # Admin templates
├── bun.lock                  # Bun lockfile
├── supabase_schema.sql       # Database schema
├── vercel.json               # Vercel configuration
└── go.mod                    # Go dependencies
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
