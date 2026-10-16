# Deployment Guide

## Render Deployment
1. Set build command: go build -o orchestrator cmd/orchestrator/main.go
2. Set start command: ./orchestrator
3. Environment variables: PORT=8080, MASTER_SECRET_KEY=..., VERCEL_TOKEN=..., VERCEL_PROJECT_ID=...
