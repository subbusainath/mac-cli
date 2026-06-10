package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

type Project struct {
	ID        string
	Name      string
	Path      string
	CreatedAt time.Time
}

func (d *DB) FindProjectByPath(ctx context.Context, path string) (*Project, error) {
	row := d.Pool.QueryRow(ctx,
		`SELECT id, name, path, created_at FROM projects WHERE path = $1`, path)
	p := &Project{}
	if err := row.Scan(&p.ID, &p.Name, &p.Path, &p.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return p, nil
}

func (d *DB) ListProjects(ctx context.Context) ([]Project, error) {
	rows, err := d.Pool.Query(ctx,
		`SELECT id, name, path, created_at FROM projects ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Path, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (d *DB) CreateProject(ctx context.Context, name, path string) (*Project, error) {
	row := d.Pool.QueryRow(ctx,
		`INSERT INTO projects (name, path) VALUES ($1, $2)
		 ON CONFLICT (path) DO UPDATE SET name = EXCLUDED.name
		 RETURNING id, name, path, created_at`,
		name, path)
	p := &Project{}
	if err := row.Scan(&p.ID, &p.Name, &p.Path, &p.CreatedAt); err != nil {
		return nil, err
	}
	return p, nil
}

type Session struct {
	ID        string
	ProjectID string
	Prompt    string
	CreatedAt time.Time
}

func (d *DB) CreateSession(ctx context.Context, projectID, prompt string) (*Session, error) {
	row := d.Pool.QueryRow(ctx,
		`INSERT INTO sessions (project_id, user_prompt) VALUES ($1, $2)
		 RETURNING id, project_id, user_prompt, created_at`,
		projectID, prompt)
	s := &Session{}
	if err := row.Scan(&s.ID, &s.ProjectID, &s.Prompt, &s.CreatedAt); err != nil {
		return nil, err
	}
	return s, nil
}

type ErrorFeedback struct {
	ID            string
	ProjectID     string
	ErrorHash     string
	ErrorSig      string
	SuccessfulFix string
	TargetFolder  string
}

func (d *DB) FindErrorFix(ctx context.Context, projectID, hash string) (*ErrorFeedback, error) {
	row := d.Pool.QueryRow(ctx,
		`SELECT id, project_id, error_hash, error_signature, successful_fix, target_folder
		 FROM terminal_errors_feedback
		 WHERE project_id = $1 AND error_hash = $2`,
		projectID, hash)
	e := &ErrorFeedback{}
	if err := row.Scan(&e.ID, &e.ProjectID, &e.ErrorHash, &e.ErrorSig, &e.SuccessfulFix, &e.TargetFolder); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return e, nil
}

func (d *DB) SaveErrorFix(ctx context.Context, projectID, hash, sig, fix, folder string) error {
	_, err := d.Pool.Exec(ctx,
		`INSERT INTO terminal_errors_feedback
		 (project_id, error_hash, error_signature, successful_fix, target_folder)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (project_id, error_hash) DO UPDATE
		 SET successful_fix = EXCLUDED.successful_fix,
		     target_folder  = EXCLUDED.target_folder`,
		projectID, hash, sig, fix, folder)
	return err
}
