#!/bin/bash
set -e

# Build CSS and JS
npm run build

# Create Vercel static output directory
mkdir -p .vercel/output/static/dist

# Copy built assets to Vercel static directory
cp web/dist/styles.css .vercel/output/static/dist/styles.css
cp web/dist/bundle.js .vercel/output/static/dist/bundle.js

echo "Static files copied to .vercel/output/static/dist/"
ls -la .vercel/output/static/dist/
