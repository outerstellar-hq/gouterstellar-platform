package platform_test

import (
	"bufio"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
)

var (
	allowedRootEntries = []string{
		".github", ".gitignore", ".golangci-lint.yml",
		"AGENTS.md", "LICENSE", "README.md", "auth", "docs", "durablefile", "go.mod", "go.sum",
		"i18n", "migration", "observability", "repository_boundary_test.go", "ui", "web",
	}
	allowedPackageDirectories = []string{
		"auth", "durablefile", "i18n", "migration", "observability", "ui", "web",
	}
	allowedDirectModules = []string{
		"github.com/alexedwards/argon2id",
		"github.com/alexedwards/scs/v2",
		"github.com/exaring/otelpgx",
		"github.com/golang-jwt/jwt/v5",
		"github.com/golang-migrate/migrate/v4",
		"github.com/gorilla/csrf",
		"github.com/jackc/pgx/v5",
		"github.com/magiconair/properties",
		"github.com/natefinch/atomic",
		"github.com/pquerna/otp",
		"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc",
		"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp",
		"go.opentelemetry.io/otel",
		"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp",
		"go.opentelemetry.io/otel/sdk",
		"go.opentelemetry.io/otel/trace",
		"google.golang.org/grpc",
	}
	hostPathPattern = regexp.MustCompile(`(^|/)(cmd|config|deployments?|extensions?|plugins?|queries|static)(/|$)`)
	artifactPattern = regexp.MustCompile(
		`(^|/)(Dockerfile[^/]*|compose[^/]*\.ya?ml)$|` +
			`(^|/)(deploy|deployment|server|startup)([._-][^/]*)?\.(ya?ml|json|toml|ps1|sh|cmd|bat|service)$|` +
			`(^|/)(plugin|extension|host|server|application|startup|wire)([_-][^/]*)?\.(go|ps1|sh|cmd|bat|ya?ml|json|toml)$|` +
			`(\.pb|_generated|\.gen|_gen)\.go$|\.(sql|toml|exe|dll|so|dylib|jar|class|wasm|a|o|obj)$`,
	)
	consumerAssetPattern = regexp.MustCompile(`\.(html|css|jsx?|tsx?|png|jpe?g|gif|webp|svg|ico|woff2?|ttf|otf)$`)
)

func TestRepositoryBoundary(t *testing.T) {
	root := repositoryRoot(t)
	checkRootEntries(t, root)
	files := repositoryFiles(t, root)
	checkPackages(t, root, files)
	checkDirectModules(t, root)
	checkArtifacts(t, root, files)
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate repository boundary test")
	}
	return filepath.Dir(filename)
}

func checkRootEntries(t *testing.T, root string) {
	t.Helper()
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read repository root: %v", err)
	}
	actual := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.Name() == ".git" || entry.Name() == ".agents" || entry.Name() == ".codex" {
			continue
		}
		actual = append(actual, entry.Name())
	}
	checkExactSet(t, "repository root", actual, allowedRootEntries)
}

func repositoryFiles(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if entry.IsDir() && (relative == ".git" || relative == ".agents" || relative == ".codex") {
			return filepath.SkipDir
		}
		if !entry.IsDir() && relative != "." {
			files = append(files, relative)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk repository: %v", err)
	}
	return files
}

func checkPackages(t *testing.T, root string, files []string) {
	t.Helper()
	packageDirectories := make(map[string]struct{})
	for _, relative := range files {
		if filepath.Ext(relative) != ".go" {
			continue
		}
		parsed, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, filepath.FromSlash(relative)), nil, parser.PackageClauseOnly)
		if err != nil {
			t.Errorf("parse package declaration in %s: %v", relative, err)
			continue
		}
		if parsed.Name.Name == "main" {
			t.Errorf("executable package is forbidden: %s", relative)
		}
		if !strings.HasSuffix(relative, "_test.go") {
			packageDirectories[filepath.ToSlash(filepath.Dir(relative))] = struct{}{}
		}
	}
	actual := make([]string, 0, len(packageDirectories))
	for directory := range packageDirectories {
		actual = append(actual, directory)
	}
	checkExactSet(t, "production package directories", actual, allowedPackageDirectories)
}

func checkDirectModules(t *testing.T, root string) {
	t.Helper()
	file, err := os.Open(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatalf("open go.mod: %v", err)
	}
	defer file.Close()

	var modules []string
	inBlock := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case line == "require (":
			inBlock = true
		case inBlock && line == ")":
			inBlock = false
		case inBlock:
			fields := strings.Fields(line)
			if len(fields) >= 2 && !strings.HasPrefix(line, "//") && !strings.Contains(line, "// indirect") {
				modules = append(modules, fields[0])
			}
		case strings.HasPrefix(line, "require "):
			if fields := strings.Fields(line); len(fields) >= 3 && !strings.Contains(line, "// indirect") {
				modules = append(modules, fields[1])
			}
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	checkExactSet(t, "direct module dependencies", modules, allowedDirectModules)
}

func checkArtifacts(t *testing.T, root string, files []string) {
	t.Helper()
	productPattern := regexp.MustCompile(`(?i)` + ("star" + "forge") + `|` + ("star" + "line"))
	for _, relative := range files {
		switch {
		case hostPathPattern.MatchString(relative):
			t.Errorf("application-host path is forbidden: %s", relative)
		case artifactPattern.MatchString(relative):
			t.Errorf("application or deployment artifact is forbidden: %s", relative)
		case consumerAssetPattern.MatchString(relative) && relative != "ui/templates/shell.html":
			t.Errorf("consumer-owned template or asset is forbidden: %s", relative)
		}
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
		if err != nil {
			t.Errorf("read %s: %v", relative, err)
			continue
		}
		if productPattern.Match(content) {
			t.Errorf("consumer product reference is forbidden: %s", relative)
		}
	}
}

func checkExactSet(t *testing.T, name string, actual, expected []string) {
	t.Helper()
	slices.Sort(actual)
	expected = slices.Clone(expected)
	slices.Sort(expected)
	if !slices.Equal(actual, expected) {
		t.Errorf("%s changed\nactual:   %s\nexpected: %s", name, strings.Join(actual, ", "), strings.Join(expected, ", "))
	}
}
