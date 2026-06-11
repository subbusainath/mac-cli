package scaffold

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteK8sBackendAndFrontend(t *testing.T) {
	root := t.TempDir()
	a := Answers{Name: "demo", Backend: "gin", Frontend: "react", Infra: "k8s"}
	if err := writeK8s(root, a); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{
		"k8s/backend-deployment.yaml", "k8s/backend-service.yaml",
		"k8s/frontend-deployment.yaml", "k8s/frontend-service.yaml",
		"k8s/kustomization.yaml",
	} {
		if _, err := os.Stat(filepath.Join(root, f)); err != nil {
			t.Fatalf("missing %s", f)
		}
	}
	kust, _ := os.ReadFile(filepath.Join(root, "k8s", "kustomization.yaml"))
	if !strings.Contains(string(kust), "backend-deployment.yaml") ||
		!strings.Contains(string(kust), "frontend-service.yaml") {
		t.Fatalf("kustomization incomplete:\n%s", kust)
	}
}

func TestWriteK8sBackendOnly(t *testing.T) {
	root := t.TempDir()
	a := Answers{Name: "demo", Backend: "fastapi", Infra: "k8s"}
	if err := writeK8s(root, a); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "k8s", "frontend-deployment.yaml")); !os.IsNotExist(err) {
		t.Fatal("frontend manifests must not exist")
	}
	dep, _ := os.ReadFile(filepath.Join(root, "k8s", "backend-deployment.yaml"))
	if !strings.Contains(string(dep), "demo-backend") {
		t.Fatalf("deployment missing app name:\n%s", dep)
	}
}
