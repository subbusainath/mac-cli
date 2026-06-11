package scaffold

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/subbusainath/mac-cli/internal/config"
	"github.com/subbusainath/mac-cli/internal/db"
)

// Answers holds the choices collected by the wizard.
type Answers struct {
	Name     string
	Path     string
	Backend  string
	Frontend string
	Cloud    string
	IAC      string
}

// New creates the full project scaffold from wizard answers.
func New(ctx context.Context, database *db.DB, a Answers) error {
	root := filepath.Clean(a.Path)

	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("create project dir: %w", err)
	}
	if err := gitInit(root); err != nil {
		return err
	}
	if err := createHexagonalStructure(root, a.Backend, a.Frontend); err != nil {
		return err
	}
	if err := initProjectFiles(root, a); err != nil {
		return err
	}
	if err := writeDockerfiles(root, a.Backend, a.Frontend); err != nil {
		return err
	}
	if err := writeCloudIaC(root, a.Cloud, a.IAC); err != nil {
		return err
	}
	if err := writeHarness(root, a); err != nil {
		return err
	}

	cfg := config.Default(a.Name, a.Backend, a.Frontend, a.Cloud, a.IAC)
	if err := config.Write(root, cfg); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	if _, err := database.CreateProject(ctx, a.Name, root); err != nil {
		return fmt.Errorf("register project: %w", err)
	}
	return nil
}

func initProjectFiles(root string, a Answers) error {
	if a.Backend != "" {
		switch a.Backend {
		case "gin":
			if err := runCmd(filepath.Join(root, "backend"), "go", "mod", "init", a.Name+"/backend"); err != nil {
				return err
			}
		case "fastapi":
			pyproject := fmt.Sprintf(`[project]
name = "%s-backend"
version = "0.1.0"
description = "%s backend"
requires-python = ">=3.12"
dependencies = [
    "fastapi>=0.115",
    "uvicorn[standard]>=0.32",
]
`, a.Name, a.Name)
			if err := os.WriteFile(filepath.Join(root, "backend", "pyproject.toml"), []byte(pyproject), 0o644); err != nil {
				return fmt.Errorf("write pyproject.toml: %w", err)
			}
			if err := runCmd(filepath.Join(root, "backend"), "uv", "lock"); err != nil {
				return fmt.Errorf("uv lock: %w", err)
			}
		case "express":
			pkg := fmt.Sprintf(`{
  "name": "%s-backend",
  "version": "0.1.0",
  "private": true,
  "scripts": {
    "dev": "tsx watch src/adapters/api/main.ts",
    "build": "tsc",
    "start": "node dist/index.js",
    "test": "vitest run"
  }
}`, a.Name)
			if err := os.WriteFile(filepath.Join(root, "backend", "package.json"), []byte(pkg+"\n"), 0o644); err != nil {
				return fmt.Errorf("write package.json: %w", err)
			}
			tsconfig := `{
  "compilerOptions": {
    "target": "ES2022",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "outDir": "dist",
    "strict": true,
    "esModuleInterop": true
  },
  "include": ["src"]
}`
			if err := os.WriteFile(filepath.Join(root, "backend", "tsconfig.json"), []byte(tsconfig+"\n"), 0o644); err != nil {
				return fmt.Errorf("write tsconfig.json: %w", err)
			}
			if err := runCmd(filepath.Join(root, "backend"), "pnpm", "install"); err != nil {
				return fmt.Errorf("pnpm install: %w", err)
			}
		case "axum":
			cargo := fmt.Sprintf(`[package]
name = "%s-backend"
version = "0.1.0"
edition = "2021"

[dependencies]
tokio = { version = "1", features = ["full"] }
axum = "0.7"
serde = { version = "1", features = ["derive"] }
tower-http = { version = "0.5", features = ["cors"] }
`, strings.ReplaceAll(a.Name, "-", "_"))
			if err := os.WriteFile(filepath.Join(root, "backend", "Cargo.toml"), []byte(cargo), 0o644); err != nil {
				return fmt.Errorf("write Cargo.toml: %w", err)
			}
		case "springboot":
			pom := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 http://maven.apache.org/xsd/maven-4.0.0.xsd"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
    <modelVersion>4.0.0</modelVersion>
    <parent>
        <groupId>org.springframework.boot</groupId>
        <artifactId>spring-boot-starter-parent</artifactId>
        <version>3.3.0</version>
    </parent>
    <groupId>com.%s</groupId>
    <artifactId>backend</artifactId>
    <version>0.1.0</version>
    <dependencies>
        <dependency>
            <groupId>org.springframework.boot</groupId>
            <artifactId>spring-boot-starter-web</artifactId>
        </dependency>
    </dependencies>
</project>
`, strings.ToLower(a.Name))
			if err := os.WriteFile(filepath.Join(root, "backend", "pom.xml"), []byte(pom), 0o644); err != nil {
				return fmt.Errorf("write pom.xml: %w", err)
			}
		}
	}

	if a.Frontend != "" {
		switch a.Frontend {
		case "react":
			pkg := fmt.Sprintf(`{
  "name": "%s-frontend",
  "version": "0.1.0",
  "private": true,
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build",
    "preview": "vite preview"
  }
}`, a.Name)
			if err := os.WriteFile(filepath.Join(root, "frontend", "package.json"), []byte(pkg+"\n"), 0o644); err != nil {
				return fmt.Errorf("write frontend package.json: %w", err)
			}
			if err := runCmd(filepath.Join(root, "frontend"), "pnpm", "install"); err != nil {
				return fmt.Errorf("pnpm install frontend: %w", err)
			}
		case "nextjs":
			pkg := fmt.Sprintf(`{
  "name": "%s-frontend",
  "version": "0.1.0",
  "private": true,
  "scripts": {
    "dev": "next dev",
    "build": "next build",
    "start": "next start"
  }
}`, a.Name)
			if err := os.WriteFile(filepath.Join(root, "frontend", "package.json"), []byte(pkg+"\n"), 0o644); err != nil {
				return fmt.Errorf("write frontend package.json: %w", err)
			}
			if err := runCmd(filepath.Join(root, "frontend"), "pnpm", "install"); err != nil {
				return fmt.Errorf("pnpm install frontend: %w", err)
			}
		case "svelte":
			pkg := fmt.Sprintf(`{
  "name": "%s-frontend",
  "version": "0.1.0",
  "private": true,
  "scripts": {
    "dev": "vite dev",
    "build": "vite build",
    "preview": "vite preview"
  }
}`, a.Name)
			if err := os.WriteFile(filepath.Join(root, "frontend", "package.json"), []byte(pkg+"\n"), 0o644); err != nil {
				return fmt.Errorf("write frontend package.json: %w", err)
			}
			if err := runCmd(filepath.Join(root, "frontend"), "pnpm", "install"); err != nil {
				return fmt.Errorf("pnpm install frontend: %w", err)
			}
		}
	}

	return nil
}

func gitInit(dir string) error {
	cmd := exec.Command("git", "-C", dir, "init")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git init: %w", err)
	}
	return nil
}

func runCmd(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %v: %w", name, args, err)
	}
	return nil
}
