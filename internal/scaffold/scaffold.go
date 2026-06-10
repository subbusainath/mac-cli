package scaffold

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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

func gitInit(dir string) error {
	cmd := exec.Command("git", "-C", dir, "init")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git init: %w", err)
	}
	return nil
}
