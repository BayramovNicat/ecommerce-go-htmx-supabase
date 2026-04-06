#!/bin/bash
set -e

# Build CSS and JS
npm run build

# Create Vercel Build Output API structure
mkdir -p .vercel/output/static/dist
mkdir -p .vercel/output/functions/api

# Copy built assets to static directory
cp web/dist/styles.css .vercel/output/static/dist/styles.css
cp web/dist/bundle.js .vercel/output/static/dist/bundle.js

# Create config.json for Vercel Build Output API
cat > .vercel/output/config.json << 'EOF'
{
  "version": 3,
  "routes": [
    {
      "src": "^/dist/(.*)$",
      "headers": {
        "cache-control": "public, max-age=31536000, immutable"
      },
      "continue": true
    },
    {
      "handle": "filesystem"
    },
    {
      "src": "/(.*)",
      "dest": "/api/index.go"
    }
  ]
}
EOF

echo "Build output structure created:"
ls -la .vercel/output/
ls -la .vercel/output/static/dist/
