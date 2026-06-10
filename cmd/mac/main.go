package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/subbusainath/mac-cli/internal/db"
	"github.com/subbusainath/mac-cli/internal/tui"
)

const defaultDSN = "postgres://postgres:postgres@localhost:5432/mac_cli?sslmode=disable"

func main() {
	root := &cobra.Command{
		Use:   "mac",
		Short: "Agentic local coding CLI",
		Long: `mac — stateful agentic coding assistant.

Run without arguments to open the project picker / new-project wizard.
Use 'mac code "<task>"' to invoke the LangGraph TDD orchestrator.`,
		RunE:          runRoot,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().String("db", "",
		`PostgreSQL DSN (default: MAC_DB_URL env, else `+defaultDSN+`)`)

	root.AddCommand(codeCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runRoot(cmd *cobra.Command, _ []string) error {
	ctx := context.Background()
	database, err := connectDB(ctx, cmd)
	if err != nil {
		return err
	}
	defer database.Close()
	return tui.Run(ctx, database)
}

func codeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "code <task>",
		Short: "Run agentic TDD coding task via LangGraph",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			database, err := connectDB(ctx, cmd)
			if err != nil {
				return err
			}
			defer database.Close()

			cwd, _ := os.Getwd()
			project, err := database.FindProjectByPath(ctx, cwd)
			if err != nil {
				return fmt.Errorf("check project: %w", err)
			}
			if project == nil {
				return fmt.Errorf("no mac project in %s — run 'mac' first to initialise", cwd)
			}

			// Hand off to the Python LangGraph orchestrator. It owns the
			// session lifecycle, TDD loop, and HIL prompts on this terminal.
			bin := os.Getenv("MAC_ORCHESTRATOR")
			if bin == "" {
				bin = "mac-orchestrator"
			}
			orch := exec.CommandContext(ctx, bin,
				"--project", cwd,
				"--task", args[0],
				"--db", resolveDSN(cmd),
			)
			orch.Stdin = os.Stdin
			orch.Stdout = os.Stdout
			orch.Stderr = os.Stderr
			if err := orch.Run(); err != nil {
				return fmt.Errorf("orchestrator: %w\n\nInstall it with: uv tool install --from ./orchestrator mac-orchestrator", err)
			}
			return nil
		},
	}
}

func resolveDSN(cmd *cobra.Command) string {
	dsn, _ := cmd.Flags().GetString("db")
	if dsn == "" {
		dsn = os.Getenv("MAC_DB_URL")
	}
	if dsn == "" {
		dsn = defaultDSN
	}
	return dsn
}

func connectDB(ctx context.Context, cmd *cobra.Command) (*db.DB, error) {
	database, err := db.Connect(ctx, resolveDSN(cmd))
	if err != nil {
		return nil, fmt.Errorf(
			"cannot connect to PostgreSQL: %w\n\nSet MAC_DB_URL env var or pass --db flag", err)
	}
	return database, nil
}
