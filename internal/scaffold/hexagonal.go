package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
)

var backendDirs = map[string][]string{
	"fastapi": {
		"backend/src/domain/entities",
		"backend/src/domain/repositories",
		"backend/src/application/use_cases",
		"backend/src/application/services",
		"backend/src/adapters/api/routers",
		"backend/src/adapters/persistence",
		"backend/src/adapters/external",
		"backend/tests/unit",
		"backend/tests/integration",
	},
	"express": {
		"backend/src/domain/entities",
		"backend/src/domain/repositories",
		"backend/src/application/useCases",
		"backend/src/application/services",
		"backend/src/adapters/api/routes",
		"backend/src/adapters/persistence",
		"backend/src/adapters/external",
		"backend/tests/unit",
		"backend/tests/integration",
	},
	"gin": {
		"backend/internal/domain/entity",
		"backend/internal/domain/repository",
		"backend/internal/application/usecase",
		"backend/internal/application/service",
		"backend/internal/adapter/handler",
		"backend/internal/adapter/repository",
		"backend/internal/adapter/external",
		"backend/cmd/server",
		"backend/tests",
	},
	"axum": {
		"backend/src/domain/entities",
		"backend/src/domain/repositories",
		"backend/src/application/use_cases",
		"backend/src/application/services",
		"backend/src/adapters/http",
		"backend/src/adapters/persistence",
		"backend/src/adapters/external",
		"backend/tests",
	},
	"springboot": {
		"backend/src/main/java/domain/entity",
		"backend/src/main/java/domain/repository",
		"backend/src/main/java/application/usecase",
		"backend/src/main/java/application/service",
		"backend/src/main/java/adapter/controller",
		"backend/src/main/java/adapter/persistence",
		"backend/src/main/java/adapter/external",
		"backend/src/test/java/unit",
		"backend/src/test/java/integration",
	},
}

var frontendDirs = map[string][]string{
	"vanilla": {
		"frontend/src",
		"frontend/public",
	},
	"react": {
		"frontend/src/components",
		"frontend/src/pages",
		"frontend/src/hooks",
		"frontend/src/services",
		"frontend/src/types",
		"frontend/public",
	},
	"nextjs": {
		"frontend/app",
		"frontend/components/ui",
		"frontend/components/shared",
		"frontend/lib",
		"frontend/types",
		"frontend/public",
	},
	"svelte": {
		"frontend/src/routes",
		"frontend/src/lib/components",
		"frontend/src/lib/stores",
		"frontend/src/lib/types",
		"frontend/static",
	},
}

func createHexagonalStructure(root, backend, frontend string) error {
	bdirs, ok := backendDirs[backend]
	if !ok {
		return fmt.Errorf("unknown backend: %s", backend)
	}
	fdirs, ok := frontendDirs[frontend]
	if !ok {
		return fmt.Errorf("unknown frontend: %s", frontend)
	}

	all := append(bdirs, fdirs...)
	all = append(all, "docs", "scripts")

	for _, d := range all {
		full := filepath.Join(root, d)
		if err := os.MkdirAll(full, 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", d, err)
		}
		// .gitkeep keeps empty dirs in git history
		if err := os.WriteFile(filepath.Join(full, ".gitkeep"), []byte{}, 0o644); err != nil {
			return fmt.Errorf("gitkeep %s: %w", d, err)
		}
	}
	return nil
}
