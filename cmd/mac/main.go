package main

import (
	"context"
	"fmt"
	"os"

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

			// Phase 2: hand off to LangGraph Python orchestrator.
			fmt.Printf("Project: %s\nTask:    %s\n\nLangGraph orchestrator — Phase 2\n", project.Name, args[0])
			return nil
		},
	}
}

func connectDB(ctx context.Context, cmd *cobra.Command) (*db.DB, error) {
	dsn, _ := cmd.Flags().GetString("db")
	if dsn == "" {
		dsn = os.Getenv("MAC_DB_URL")
	}
	if dsn == "" {
		dsn = defaultDSN
	}
	database, err := db.Connect(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot connect to PostgreSQL: %w\n\nSet MAC_DB_URL env var or pass --db flag", err)
	}
	return database, nil
}
