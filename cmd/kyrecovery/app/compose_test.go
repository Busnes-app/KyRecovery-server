package app

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func readCompose(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "docker-compose.yml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// Settings reach the container from .env, and the base file never pins a runtime
// variable to an interpolated value: environment outranks env_file, so a
// `NAME: ${NAME:-}` entry would blank what an operator's override env_file supplies.
func TestComposeForwardsSettingsFromDotEnv(t *testing.T) {
	compose := readCompose(t)
	if !strings.Contains(compose, "    env_file:\n      - path: .env\n        required: false\n") {
		t.Fatal("docker-compose.yml must load .env into the container (env_file, required: false)")
	}
	if strings.Contains(compose, "environment: []") {
		t.Fatal("docker-compose.yml still forwards no environment")
	}
	read := map[string]bool{"KYRECOVERY_SECRET_KEY": true} // secrets.EnvKey
	getenv := regexp.MustCompile(`os\.(?:Getenv|LookupEnv)\("([A-Z0-9_]+)"\)`)
	root := filepath.Join("..", "..", "..")
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == ".claude") {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		for _, m := range getenv.FindAllSubmatch(src, -1) {
			read[string(m[1])] = true
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	for name := range read {
		if strings.Contains(compose, name+": ${") || strings.Contains(compose, "- "+name+"=${") {
			t.Errorf("docker-compose.yml pins %s to an interpolated value", name)
		}
	}
}

func TestComposePublishesOnLoopbackByDefault(t *testing.T) {
	if !strings.Contains(readCompose(t), `- "${KYRECOVERY_BIND:-127.0.0.1}:8095:8095"`) {
		t.Fatal(`docker-compose.yml must publish "${KYRECOVERY_BIND:-127.0.0.1}:8095:8095"`)
	}
}
