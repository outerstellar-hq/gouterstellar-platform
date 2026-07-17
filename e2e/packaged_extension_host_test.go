package e2e

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	agentOwner     = "codex-root"
	agentTask      = "packaged-e2e"
	adminUsername  = "admin"
	adminPassword  = "AdminParity1!"
	containerPort  = "8080"
	databaseName   = "outerstellar"
	databaseUser   = "outerstellar"
	databaseSecret = "outerstellar"
)

var csrfMetaPattern = regexp.MustCompile(`<meta name="csrf-token" content="([^"]+)"`)

func TestPackagedExtensionHostEndToEndParity(t *testing.T) {
	if runtime.GOOS == "js" {
		t.Skip("podman-backed packaged E2E requires a local OS process runner")
	}
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skipf("podman not available: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	image := "localhost/gouterstellar-platform:e2e-" + suffix
	networkName := "gsp-e2e-" + suffix
	databaseContainer := networkName + "-db"
	appContainer := networkName + "-app"
	databaseURL := fmt.Sprintf(
		"postgres://%s:%s@%s:5432/%s?sslmode=disable",
		databaseUser,
		databaseSecret,
		databaseContainer,
		databaseName,
	)

	runPodman(t, ctx, "build", "--jobs=1", "--label", "agent.owner="+agentOwner, "--label", "agent.task="+agentTask, "-t", image, ".")
	defer cleanupPodman(t, context.Background(), "rmi", "-f", image)

	runPodman(t, ctx, "network", "create", "--label", "agent.owner="+agentOwner, "--label", "agent.task="+agentTask, networkName)
	defer cleanupPodman(t, context.Background(), "network", "rm", networkName)

	runPodman(t, ctx,
		"run", "-d",
		"--name", databaseContainer,
		"--network", networkName,
		"--label", "agent.owner="+agentOwner,
		"--label", "agent.task="+agentTask,
		"--cpus=1",
		"--memory=512m",
		"-e", "POSTGRES_DB="+databaseName,
		"-e", "POSTGRES_USER="+databaseUser,
		"-e", "POSTGRES_PASSWORD="+databaseSecret,
		"postgres:16-alpine",
	)
	defer cleanupPodman(t, context.Background(), "rm", "-f", databaseContainer)

	eventually(t, 90*time.Second, func() error {
		return runPodmanQuiet(ctx, "exec", databaseContainer, "pg_isready", "-U", databaseUser, "-d", databaseName)
	})
	eventually(t, 90*time.Second, func() error {
		return runPodmanQuiet(ctx,
			"run", "--rm",
			"--network", networkName,
			"--label", "agent.owner="+agentOwner,
			"--label", "agent.task="+agentTask,
			"--cpus=1",
			"--memory=128m",
			"postgres:16-alpine",
			"pg_isready", "-h", databaseContainer, "-U", databaseUser, "-d", databaseName,
		)
	})

	runPodman(t, ctx,
		"run", "--rm",
		"--name", networkName+"-migrate",
		"--network", networkName,
		"--label", "agent.owner="+agentOwner,
		"--label", "agent.task="+agentTask,
		"--cpus=1",
		"--memory=256m",
		"-e", "DATABASE_URL="+databaseURL,
		"--entrypoint", "/app/migrate",
		image,
	)

	runPodman(t, ctx,
		"run", "--rm",
		"--name", networkName+"-seed",
		"--network", networkName,
		"--label", "agent.owner="+agentOwner,
		"--label", "agent.task="+agentTask,
		"--cpus=1",
		"--memory=256m",
		"-e", "DATABASE_URL="+databaseURL,
		"-e", "ADMIN_USERNAME="+adminUsername,
		"-e", "ADMIN_PASSWORD="+adminPassword,
		"--entrypoint", "/app/seed",
		image,
	)

	seededUsers := strings.TrimSpace(runPodmanOutput(t, ctx, "exec", databaseContainer, "psql", "-U", databaseUser, "-d", databaseName, "-tAc", "select count(*) from plt_users where username = 'admin'"))
	if seededUsers != "1" {
		t.Fatalf("seeded admin user count = %q, want 1", seededUsers)
	}

	runPodman(t, ctx,
		"run", "-d",
		"--name", appContainer,
		"--network", networkName,
		"--label", "agent.owner="+agentOwner,
		"--label", "agent.task="+agentTask,
		"--cpus=2",
		"--memory=512m",
		"--read-only",
		"--tmpfs", "/tmp",
		"-p", "127.0.0.1::"+containerPort,
		"-e", "DATABASE_URL="+databaseURL,
		"-e", "PORT="+containerPort,
		"-e", "APP_BASE_URL=http://127.0.0.1",
		"-e", "TOKEN_PEPPER=outerstellar-e2e-token-pepper",
		"-e", "PLATFORM_MODE=extension-host",
		"-e", "MAX_REQUEST_BODY_BYTES=2097152",
		image,
	)
	defer cleanupPodman(t, context.Background(), "rm", "-f", appContainer)

	baseURL := "http://" + mappedAddress(t, ctx, appContainer)
	client := noRedirectClient(t)
	eventually(t, 90*time.Second, func() error {
		resp, body, err := get(client, baseURL+"/auth")
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("GET /auth status %d: %s", resp.StatusCode, truncate(body))
		}
		if !strings.Contains(body, `name="csrf_token"`) {
			return errors.New("login page did not render a CSRF form token")
		}
		return nil
	})

	loginAsAdmin(t, client, baseURL)
	assertStatus(t, client, baseURL+"/openapi.json", http.StatusOK)
	assertStatus(t, client, baseURL+"/api/openapi.json", http.StatusOK)

	reports := assertStatus(t, client, baseURL+"/reports", http.StatusOK)
	assertContains(t, reports, "Reports")
	assertContains(t, reports, `href="/reports"`)
	assertNotContains(t, reports, `action="/search"`)
	assertNotContains(t, reports, `href="/settings"`)
	assertNotContains(t, reports, `href="/contacts"`)
	assertNotContains(t, reports, `href="/notifications"`)
	assertNotContains(t, reports, `href="/auth/profile"`)
	assertNotContains(t, reports, `href="/admin/users"`)
	assertNotContains(t, reports, `id="notification-bell"`)

	for _, path := range []string{
		"/",
		"/settings",
		"/contacts",
		"/search",
		"/notifications",
		"/messages/trash",
		"/auth/profile",
		"/admin/users",
		"/admin/dev",
		"/admin/extensions",
	} {
		assertStatus(t, client, baseURL+path, http.StatusNotFound)
	}

	css := assertStatus(t, client, baseURL+"/site.css", http.StatusOK)
	assertContains(t, css, ".extension-diagnostics")
	assertContains(t, css, ".shell-search")

	assertCacheRevalidation(t, client, baseURL+"/extensions/reports/assets/site.css")

	runPodman(t, ctx,
		"run", "--rm",
		"--label", "agent.owner="+agentOwner,
		"--label", "agent.task="+agentTask,
		"--entrypoint", "/bin/sh",
		image,
		"-c", "test -x /app/server && test -x /app/migrate && test -x /app/seed && test -f /app/static/css/main.css && test ! -e /app/node_modules && test ! -e /app/package.json",
	)
}

func noRedirectClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("create cookie jar: %v", err)
	}
	return &http.Client{
		Jar: jar,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Timeout: 10 * time.Second,
	}
}

func loginAsAdmin(t *testing.T, client *http.Client, baseURL string) {
	t.Helper()
	resp, body, err := get(client, baseURL+"/auth")
	if err != nil {
		t.Fatalf("GET /auth: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /auth status %d: %s", resp.StatusCode, truncate(body))
	}

	matches := csrfMetaPattern.FindStringSubmatch(body)
	if len(matches) != 2 {
		t.Fatalf("login page missing CSRF meta token")
	}

	form := url.Values{}
	form.Set("username", adminUsername)
	form.Set("password", adminPassword)
	form.Set("csrf_token", matches[1])

	req, err := http.NewRequest(http.MethodPost, baseURL+"/auth/login", strings.NewReader(form.Encode()))
	if err != nil {
		t.Fatalf("build login request: %v", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	loginResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST /auth/login: %v", err)
	}
	defer loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusSeeOther && loginResp.StatusCode != http.StatusFound {
		bodyBytes, _ := io.ReadAll(loginResp.Body)
		t.Fatalf("POST /auth/login status %d: %s", loginResp.StatusCode, truncate(string(bodyBytes)))
	}
}

func assertCacheRevalidation(t *testing.T, client *http.Client, target string) {
	t.Helper()
	resp, body, err := get(client, target)
	if err != nil {
		t.Fatalf("GET %s: %v", target, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status %d: %s", target, resp.StatusCode, truncate(body))
	}
	etag := resp.Header.Get("ETag")
	if etag == "" {
		t.Fatalf("GET %s did not return an ETag", target)
	}

	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		t.Fatalf("build cache request: %v", err)
	}
	req.Header.Set("If-None-Match", etag)
	revalidated, err := client.Do(req)
	if err != nil {
		t.Fatalf("revalidate %s: %v", target, err)
	}
	defer revalidated.Body.Close()
	if revalidated.StatusCode != http.StatusNotModified {
		bodyBytes, _ := io.ReadAll(revalidated.Body)
		t.Fatalf("revalidate %s status %d: %s", target, revalidated.StatusCode, truncate(string(bodyBytes)))
	}
}

func assertStatus(t *testing.T, client *http.Client, target string, status int) string {
	t.Helper()
	resp, body, err := get(client, target)
	if err != nil {
		t.Fatalf("GET %s: %v", target, err)
	}
	resp.Body.Close()
	if resp.StatusCode != status {
		t.Fatalf("GET %s status %d, want %d: %s", target, resp.StatusCode, status, truncate(body))
	}
	return body
}

func get(client *http.Client, target string) (*http.Response, string, error) {
	resp, err := client.Get(target)
	if err != nil {
		return nil, "", err
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		resp.Body.Close()
		return nil, "", err
	}
	resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	return resp, string(bodyBytes), nil
}

func assertContains(t *testing.T, body, expected string) {
	t.Helper()
	if !strings.Contains(body, expected) {
		t.Fatalf("response did not contain %q: %s", expected, truncate(body))
	}
}

func assertNotContains(t *testing.T, body, unexpected string) {
	t.Helper()
	if strings.Contains(body, unexpected) {
		t.Fatalf("response unexpectedly contained %q: %s", unexpected, truncate(body))
	}
}

func eventually(t *testing.T, timeout time.Duration, check func() error) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := check(); err == nil {
			return
		} else {
			lastErr = err
		}
		time.Sleep(500 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s: %v", timeout, lastErr)
}

func mappedAddress(t *testing.T, ctx context.Context, container string) string {
	t.Helper()
	output := strings.TrimSpace(runPodmanOutput(t, ctx, "port", container, containerPort+"/tcp"))
	lines := strings.Split(output, "\n")
	address := strings.TrimSpace(lines[len(lines)-1])
	if address == "" {
		t.Fatalf("podman port returned no address for %s", container)
	}
	return address
}

func runPodman(t *testing.T, ctx context.Context, args ...string) {
	t.Helper()
	if err := runPodmanQuiet(ctx, args...); err != nil {
		t.Fatalf("podman %s: %v", strings.Join(args, " "), err)
	}
}

func runPodmanQuiet(ctx context.Context, args ...string) error {
	cmd := exec.CommandContext(ctx, "podman", args...)
	cmd.Dir = repositoryRoot()
	cmd.Env = append(os.Environ(), "AGENT_OWNER="+agentOwner, "AGENT_TASK="+agentTask, "GOMAXPROCS=4")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w\n%s", err, truncate(string(output)))
	}
	return nil
}

func runPodmanOutput(t *testing.T, ctx context.Context, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(ctx, "podman", args...)
	cmd.Dir = repositoryRoot()
	cmd.Env = append(os.Environ(), "AGENT_OWNER="+agentOwner, "AGENT_TASK="+agentTask, "GOMAXPROCS=4")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("podman %s: %v\n%s", strings.Join(args, " "), err, truncate(string(output)))
	}
	return string(output)
}

func cleanupPodman(t *testing.T, ctx context.Context, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(ctx, "podman", args...)
	cmd.Dir = repositoryRoot()
	cmd.Env = append(os.Environ(), "AGENT_OWNER="+agentOwner, "AGENT_TASK="+agentTask, "GOMAXPROCS=4")
	if output, err := cmd.CombinedOutput(); err != nil && len(output) > 0 {
		t.Logf("cleanup podman %s: %v\n%s", strings.Join(args, " "), err, truncate(string(output)))
	}
}

func truncate(value string) string {
	value = strings.TrimSpace(value)
	if len(value) <= 1000 {
		return value
	}
	return value[:1000] + "...<truncated>"
}

func repositoryRoot() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	if filepath.Base(wd) == "e2e" {
		return filepath.Dir(wd)
	}
	return wd
}
