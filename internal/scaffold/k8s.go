package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Backend containers listen on 8000; frontends on 3000. Matches the ports
// already used by writeDockerfiles.
func writeK8s(root string, a Answers) error {
	dir := filepath.Join(root, "k8s")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir k8s: %w", err)
	}

	var resources []string
	write := func(name, content string) error {
		resources = append(resources, name)
		return os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644)
	}

	if a.Backend != "" {
		if err := write("backend-deployment.yaml", k8sDeployment(a.Name, "backend", 8000)); err != nil {
			return err
		}
		if err := write("backend-service.yaml", k8sService(a.Name, "backend", 8000)); err != nil {
			return err
		}
	}
	if a.Frontend != "" {
		if err := write("frontend-deployment.yaml", k8sDeployment(a.Name, "frontend", 3000)); err != nil {
			return err
		}
		if err := write("frontend-service.yaml", k8sService(a.Name, "frontend", 3000)); err != nil {
			return err
		}
	}

	kust := "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n"
	for _, r := range resources {
		kust += "  - " + r + "\n"
	}
	return os.WriteFile(filepath.Join(dir, "kustomization.yaml"), []byte(kust), 0o644)
}

func k8sDeployment(project, component string, port int) string {
	app := fmt.Sprintf("%s-%s", strings.ToLower(project), component)
	return fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %[1]s
  labels:
    app: %[1]s
spec:
  replicas: 1
  selector:
    matchLabels:
      app: %[1]s
  template:
    metadata:
      labels:
        app: %[1]s
    spec:
      containers:
        - name: %[2]s
          image: %[1]s:latest
          imagePullPolicy: IfNotPresent
          ports:
            - containerPort: %[3]d
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 512Mi
`, app, component, port)
}

func k8sService(project, component string, port int) string {
	app := fmt.Sprintf("%s-%s", strings.ToLower(project), component)
	return fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %[1]s
spec:
  selector:
    app: %[1]s
  ports:
    - port: 80
      targetPort: %[2]d
`, app, port)
}
