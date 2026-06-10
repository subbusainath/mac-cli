package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var backendDockerfiles = map[string]string{
	"fastapi": `# syntax=docker/dockerfile:1
FROM python:3.12-slim AS base
WORKDIR /app
RUN pip install uv

FROM base AS deps
COPY pyproject.toml uv.lock ./
RUN uv sync --frozen --no-dev

FROM base AS runner
COPY --from=deps /app/.venv .venv
ENV PATH="/app/.venv/bin:$PATH"
COPY . .
EXPOSE 8000
CMD ["uvicorn", "src.adapters.api.main:app", "--host", "0.0.0.0", "--port", "8000"]
`,
	"express": `# syntax=docker/dockerfile:1
FROM node:22-alpine AS base
RUN corepack enable pnpm
WORKDIR /app

FROM base AS deps
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

FROM base AS build
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN pnpm build

FROM base AS runner
ENV NODE_ENV=production
COPY --from=build /app/dist ./dist
COPY --from=build /app/node_modules ./node_modules
EXPOSE 3000
CMD ["node", "dist/index.js"]
`,
	"gin": `# syntax=docker/dockerfile:1
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server

FROM gcr.io/distroless/static-debian12 AS runner
COPY --from=builder /app/server /server
EXPOSE 8080
ENTRYPOINT ["/server"]
`,
	"axum": `# syntax=docker/dockerfile:1
FROM rust:1.80-alpine AS builder
RUN apk add --no-cache musl-dev
WORKDIR /app
COPY Cargo.toml Cargo.lock ./
RUN mkdir src && echo "fn main() {}" > src/main.rs \
    && cargo build --release \
    && rm src/main.rs
COPY . .
RUN cargo build --release

FROM gcr.io/distroless/static-debian12 AS runner
COPY --from=builder /app/target/release/server /server
EXPOSE 3000
ENTRYPOINT ["/server"]
`,
	"springboot": `# syntax=docker/dockerfile:1
FROM maven:3.9-eclipse-temurin-21 AS builder
WORKDIR /app
COPY pom.xml .
RUN mvn dependency:go-offline -q
COPY src ./src
RUN mvn package -DskipTests

FROM eclipse-temurin:21-jre-alpine AS runner
WORKDIR /app
COPY --from=builder /app/target/*.jar app.jar
EXPOSE 8080
ENTRYPOINT ["java", "-jar", "app.jar"]
`,
}

var frontendDockerfiles = map[string]string{
	"vanilla": `# syntax=docker/dockerfile:1
FROM nginx:alpine
COPY src /usr/share/nginx/html
EXPOSE 80
`,
	"react": `# syntax=docker/dockerfile:1
FROM node:22-alpine AS builder
RUN corepack enable pnpm
WORKDIR /app
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY . .
RUN pnpm build

FROM nginx:alpine AS runner
COPY --from=builder /app/dist /usr/share/nginx/html
EXPOSE 80
`,
	"nextjs": `# syntax=docker/dockerfile:1
FROM node:22-alpine AS base
RUN corepack enable pnpm

FROM base AS deps
WORKDIR /app
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

FROM base AS builder
WORKDIR /app
COPY --from=deps /app/node_modules ./node_modules
COPY . .
RUN pnpm build

FROM base AS runner
ENV NODE_ENV=production
WORKDIR /app
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
COPY --from=builder /app/public ./public
EXPOSE 3000
CMD ["node", "server.js"]
`,
	"svelte": `# syntax=docker/dockerfile:1
FROM node:22-alpine AS builder
RUN corepack enable pnpm
WORKDIR /app
COPY package.json pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY . .
RUN pnpm build

FROM node:22-alpine AS runner
WORKDIR /app
COPY --from=builder /app/build ./build
COPY --from=builder /app/package.json ./
EXPOSE 3000
CMD ["node", "build"]
`,
}

var backendPorts = map[string]string{
	"fastapi": "8000", "express": "3000", "gin": "8080",
	"axum": "3000", "springboot": "8080",
}
var frontendPorts = map[string]string{
	"nextjs": "3000", "react": "80", "svelte": "3000", "vanilla": "80",
}

func dockerCompose(backend, frontend string) string {
	bp := backendPorts[backend]
	fp := frontendPorts[frontend]
	return fmt.Sprintf(`services:
  backend:
    build:
      context: ./backend
      dockerfile: Dockerfile
    ports:
      - "%s:%s"
    environment:
      - DATABASE_URL=postgres://postgres:postgres@db:5432/app
    depends_on:
      db:
        condition: service_healthy

  frontend:
    build:
      context: ./frontend
      dockerfile: Dockerfile
    ports:
      - "%s:%s"
    depends_on:
      - backend

  db:
    image: pgvector/pgvector:pg16
    environment:
      POSTGRES_DB: app
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
    volumes:
      - pg_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres"]
      interval: 5s
      timeout: 5s
      retries: 5

volumes:
  pg_data:
`, bp, bp, fp, fp)
}

var dockerignore = strings.Join([]string{
	".git", ".mac", "*.md", "__pycache__",
	"node_modules", "target", ".venv", "dist", ".next",
}, "\n") + "\n"

func writeDockerfiles(root, backend, frontend string) error {
	bdf, ok := backendDockerfiles[backend]
	if !ok {
		return fmt.Errorf("no Dockerfile for backend: %s", backend)
	}
	fdf, ok := frontendDockerfiles[frontend]
	if !ok {
		fdf = frontendDockerfiles["vanilla"]
	}

	pairs := []struct{ path, content string }{
		{filepath.Join(root, "backend", "Dockerfile"), bdf},
		{filepath.Join(root, "frontend", "Dockerfile"), fdf},
		{filepath.Join(root, "docker-compose.yml"), dockerCompose(backend, frontend)},
		{filepath.Join(root, ".dockerignore"), dockerignore},
	}
	for _, p := range pairs {
		if err := os.MkdirAll(filepath.Dir(p.path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p.path, []byte(p.content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", p.path, err)
		}
	}
	return nil
}
