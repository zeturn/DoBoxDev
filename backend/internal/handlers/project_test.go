package handlers

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"docode/internal/database"
	"docode/internal/models"

	"github.com/gofiber/fiber/v2"
)

func TestSandboxImageAllowsOnlyDefaultImage(t *testing.T) {
	image, err := sandboxImage("")
	if err != nil {
		t.Fatalf("sandboxImage returned error for empty image: %v", err)
	}
	if image != defaultSandboxImage {
		t.Fatalf("expected default image %q, got %q", defaultSandboxImage, image)
	}

	image, err = sandboxImage(defaultSandboxImage)
	if err != nil {
		t.Fatalf("sandboxImage returned error for default image: %v", err)
	}
	if image != defaultSandboxImage {
		t.Fatalf("expected default image %q, got %q", defaultSandboxImage, image)
	}

	if _, err := sandboxImage("ubuntu:24.04"); err == nil {
		t.Fatal("expected non-default project sandbox image to be rejected")
	}
}

func TestSandboxResourceLimitsAreBackendCapped(t *testing.T) {
	if got := sandboxCPULimit(0); got != defaultCPULimit {
		t.Fatalf("expected default cpu limit, got %.2f", got)
	}
	if got := sandboxCPULimit(defaultCPULimit * 2); got != defaultCPULimit {
		t.Fatalf("expected capped cpu limit, got %.2f", got)
	}
	if got := sandboxCPULimit(defaultCPULimit / 2); got != defaultCPULimit/2 {
		t.Fatalf("expected lower requested cpu limit, got %.2f", got)
	}

	if got := sandboxMemoryLimit(0); got != defaultMemoryLimit {
		t.Fatalf("expected default memory limit, got %d", got)
	}
	if got := sandboxMemoryLimit(defaultMemoryLimit * 2); got != defaultMemoryLimit {
		t.Fatalf("expected capped memory limit, got %d", got)
	}
	if got := sandboxMemoryLimit(defaultMemoryLimit / 2); got != defaultMemoryLimit/2 {
		t.Fatalf("expected lower requested memory limit, got %d", got)
	}
}

func TestSandboxNetworkModeIsPolicyOnly(t *testing.T) {
	onlineModes := []string{"", "project", "bridge", " PROJECT "}
	for _, mode := range onlineModes {
		internal, err := sandboxNetworkInternal(mode)
		if err != nil {
			t.Fatalf("sandboxNetworkInternal(%q) returned error: %v", mode, err)
		}
		if internal {
			t.Fatalf("sandboxNetworkInternal(%q) should create a normal project network", mode)
		}
	}

	offlineModes := []string{"no_internet", "no-internet", "internal", "offline", " INTERNAL "}
	for _, mode := range offlineModes {
		internal, err := sandboxNetworkInternal(mode)
		if err != nil {
			t.Fatalf("sandboxNetworkInternal(%q) returned error: %v", mode, err)
		}
		if !internal {
			t.Fatalf("sandboxNetworkInternal(%q) should create an internal no-internet network", mode)
		}
	}

	for _, mode := range []string{"host", "none", "container:abc", "dobox_default"} {
		if _, err := sandboxNetworkInternal(mode); err == nil {
			t.Fatalf("expected raw Docker network mode %q to be rejected", mode)
		}
	}
}

func TestProjectSandboxWorkspaceIsFixedToDefault(t *testing.T) {
	allowed := []string{"", "workspace", "/workspace", "/workspace/."}
	for _, workspace := range allowed {
		got, err := sandboxWorkspace(workspace)
		if err != nil {
			t.Fatalf("sandboxWorkspace(%q) returned error: %v", workspace, err)
		}
		if got != defaultWorkspacePath {
			t.Fatalf("sandboxWorkspace(%q) = %q, want %q", workspace, got, defaultWorkspacePath)
		}
	}

	for _, workspace := range []string{"/", "/tmp", "../workspace", "/workspace/..", "/workspace-other"} {
		if _, err := sandboxWorkspace(workspace); err == nil {
			t.Fatalf("expected workspace %q to be rejected", workspace)
		}
	}
}

func TestAuditInputJSONRedactsContentBase64AndEnv(t *testing.T) {
	var input = struct {
		Path          string   `json:"path"`
		Content       string   `json:"content"`
		ContentBase64 string   `json:"content_base64"`
		Env           []string `json:"env"`
	}{
		Path:          "README.md",
		Content:       strings.Repeat("secret-", 120),
		ContentBase64: "c2VjcmV0",
		Env:           []string{"API_KEY=secret-token", "DEBUG=1"},
	}

	text := auditInputJSON(input)

	if !strings.Contains(text, `"path":"README.md"`) {
		t.Fatalf("audit input should preserve path context: %s", text)
	}
	if !strings.Contains(text, `"bytes":840`) {
		t.Fatalf("audit input should record content size: %s", text)
	}
	if !strings.Contains(text, `"base64_bytes":8`) {
		t.Fatalf("audit input should record base64 size: %s", text)
	}
	if !strings.Contains(text, `"redacted":true`) {
		t.Fatalf("audit input should mark redacted fields: %s", text)
	}
	if strings.Contains(text, "secret-") || strings.Contains(text, "API_KEY=secret-token") || strings.Contains(text, "c2VjcmV0") {
		t.Fatalf("audit input leaked sensitive content: %s", text)
	}
}

func TestAuditListLimitIsBounded(t *testing.T) {
	if got := auditListLimit(""); got != 100 {
		t.Fatalf("expected default limit 100, got %d", got)
	}
	if got := auditListLimit("25"); got != 25 {
		t.Fatalf("expected requested limit 25, got %d", got)
	}
	if got := auditListLimit("5000"); got != 500 {
		t.Fatalf("expected max limit 500, got %d", got)
	}
}

func TestPreviewDescriptorDoesNotExposeDockerHandles(t *testing.T) {
	payload, err := json.Marshal(previewDescriptor(1, 2, 3000))
	if err != nil {
		t.Fatalf("failed to marshal preview descriptor: %v", err)
	}
	text := string(payload)
	for _, forbidden := range []string{"container_id", "network", "internal_url", "dobox-p1-sandbox", "dobox_project_1"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("preview descriptor leaked %q: %s", forbidden, text)
		}
	}
	for _, expected := range []string{`"project_id":1`, `"sandbox_id":2`, `"port":3000`} {
		if !strings.Contains(text, expected) {
			t.Fatalf("preview descriptor missing %q: %s", expected, text)
		}
	}
}

func TestPublicProjectResponseDoesNotExposeDockerHandles(t *testing.T) {
	now := time.Now()
	response := publicProjectResponse(
		&models.Project{
			ID:        1,
			UserID:    7,
			Name:      "agent task",
			RepoURL:   "https://example.com/repo.git",
			Branch:    "main",
			Workspace: "/workspace",
			SandboxID: 2,
			CreatedAt: now,
			UpdatedAt: now,
		},
		&models.Sandbox{
			ID:            2,
			UserID:        7,
			ProjectID:     1,
			ContainerID:   "docker-container-secret",
			Name:          "dobox-p1-sandbox",
			Image:         defaultSandboxImage,
			Status:        "running",
			WorkspacePath: "/workspace",
			VolumeName:    "dobox_project_1",
			NetworkName:   "dobox_project_1",
			CPULimit:      defaultCPULimit,
			MemoryLimit:   defaultMemoryLimit,
			CreatedAt:     now,
			UpdatedAt:     now,
		},
	)
	payload, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal public response: %v", err)
	}
	text := string(payload)
	for _, forbidden := range []string{"container_id", "docker-container-secret", "volume_name", "network_name"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("public response leaked %q: %s", forbidden, text)
		}
	}
	if !strings.Contains(text, `"sandbox_id":2`) {
		t.Fatalf("public response should include the project sandbox id: %s", text)
	}
}

func TestValidateToolSessionRequiresOwnedProjectSession(t *testing.T) {
	setupProjectHandlerTestDB(t)
	if err := database.DB.Create(&models.AgentSession{ID: 10, UserID: 7, ProjectID: 1, Name: "same-project", Status: "active"}).Error; err != nil {
		t.Fatalf("failed to create same-project session: %v", err)
	}
	if err := database.DB.Create(&models.AgentSession{ID: 11, UserID: 7, ProjectID: 2, Name: "other-project", Status: "active"}).Error; err != nil {
		t.Fatalf("failed to create other-project session: %v", err)
	}
	if err := database.DB.Create(&models.AgentSession{ID: 12, UserID: 8, ProjectID: 1, Name: "other-user", Status: "active"}).Error; err != nil {
		t.Fatalf("failed to create other-user session: %v", err)
	}

	tests := []struct {
		name       string
		rawID      string
		wantStatus int
		wantBody   string
	}{
		{name: "missing session is allowed", rawID: "", wantStatus: fiber.StatusOK, wantBody: `"agent_session_id":0`},
		{name: "same project session is allowed", rawID: "10", wantStatus: fiber.StatusOK, wantBody: `"agent_session_id":10`},
		{name: "malformed session is rejected", rawID: "session-10", wantStatus: fiber.StatusBadRequest, wantBody: "must be a positive integer"},
		{name: "other project session is rejected", rawID: "11", wantStatus: fiber.StatusBadRequest, wantBody: "does not belong to project"},
		{name: "other user session is rejected", rawID: "12", wantStatus: fiber.StatusBadRequest, wantBody: "does not belong to project"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New()
			handler := NewProjectHandler(nil)
			app.Get("/validate", func(c *fiber.Ctx) error {
				sessionID, ok := handler.toolSessionFromQuery(c, 7, 1)
				if !ok {
					return nil
				}
				return c.JSON(fiber.Map{"agent_session_id": sessionID})
			})

			req := httptest.NewRequest("GET", "/validate?agent_session_id="+tc.rawID, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != tc.wantStatus {
				t.Fatalf("expected status %d, got %d: %s", tc.wantStatus, resp.StatusCode, string(body))
			}
			if !strings.Contains(string(body), tc.wantBody) {
				t.Fatalf("expected body to contain %q, got %s", tc.wantBody, string(body))
			}
		})
	}
}

func TestListToolCallsValidatesAgentSessionFilter(t *testing.T) {
	setupProjectHandlerTestDB(t)
	seedOwnedProjectSandbox(t)
	if err := database.DB.Create(&models.AgentSession{ID: 10, UserID: 7, ProjectID: 1, Name: "same-project", Status: "active"}).Error; err != nil {
		t.Fatalf("failed to create same-project session: %v", err)
	}
	if err := database.DB.Create(&models.AgentSession{ID: 11, UserID: 7, ProjectID: 2, Name: "other-project", Status: "active"}).Error; err != nil {
		t.Fatalf("failed to create other-project session: %v", err)
	}
	calls := []models.ToolCall{
		{UserID: 7, ProjectID: 1, AgentSessionID: 10, ToolName: "agent.run_command", Status: "succeeded"},
		{UserID: 7, ProjectID: 1, AgentSessionID: 0, ToolName: "agent.git_status", Status: "succeeded"},
		{UserID: 7, ProjectID: 2, AgentSessionID: 11, ToolName: "agent.read_file", Status: "succeeded"},
	}
	if err := database.DB.Create(&calls).Error; err != nil {
		t.Fatalf("failed to create tool calls: %v", err)
	}

	app := fiber.New()
	handler := NewProjectHandler(nil)
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("userID", uint(7))
		return c.Next()
	})
	app.Get("/projects/:projectId/agent/tool-calls", handler.ListToolCalls)

	validReq := httptest.NewRequest("GET", "/projects/1/agent/tool-calls?agent_session_id=10", nil)
	validResp, err := app.Test(validReq)
	if err != nil {
		t.Fatalf("valid filter request failed: %v", err)
	}
	defer validResp.Body.Close()
	validBody, _ := io.ReadAll(validResp.Body)
	if validResp.StatusCode != fiber.StatusOK {
		t.Fatalf("expected valid filter status %d, got %d: %s", fiber.StatusOK, validResp.StatusCode, string(validBody))
	}
	if !strings.Contains(string(validBody), `"agent_session_id":10`) || strings.Contains(string(validBody), `"agent_session_id":0`) {
		t.Fatalf("expected only session 10 tool calls, got %s", string(validBody))
	}

	for _, tc := range []struct {
		name     string
		query    string
		wantBody string
	}{
		{name: "malformed", query: "session-10", wantBody: "must be a positive integer"},
		{name: "foreign", query: "11", wantBody: "does not belong to project"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/projects/1/agent/tool-calls?agent_session_id="+tc.query, nil)
			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != fiber.StatusBadRequest {
				t.Fatalf("expected status %d, got %d: %s", fiber.StatusBadRequest, resp.StatusCode, string(body))
			}
			if !strings.Contains(string(body), tc.wantBody) {
				t.Fatalf("expected body to contain %q, got %s", tc.wantBody, string(body))
			}
		})
	}
}

func TestRejectedAgentToolRequestsAreAudited(t *testing.T) {
	setupProjectHandlerTestDB(t)
	seedOwnedProjectSandbox(t)

	app := fiber.New()
	handler := NewProjectHandler(nil)
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("userID", uint(7))
		return c.Next()
	})
	app.Post("/projects/:projectId/files/read", handler.ReadFile)
	app.Get("/projects/:projectId/git/diff", handler.GitDiff)

	readReq := httptest.NewRequest("POST", "/projects/1/files/read", strings.NewReader(`{"path":"/etc/passwd"}`))
	readReq.Header.Set("Content-Type", "application/json")
	readResp, err := app.Test(readReq)
	if err != nil {
		t.Fatalf("read request failed: %v", err)
	}
	defer readResp.Body.Close()
	if readResp.StatusCode != fiber.StatusBadRequest {
		body, _ := io.ReadAll(readResp.Body)
		t.Fatalf("expected read status %d, got %d: %s", fiber.StatusBadRequest, readResp.StatusCode, string(body))
	}

	diffReq := httptest.NewRequest("GET", "/projects/1/git/diff?agent_session_id=session-10", nil)
	diffResp, err := app.Test(diffReq)
	if err != nil {
		t.Fatalf("diff request failed: %v", err)
	}
	defer diffResp.Body.Close()
	if diffResp.StatusCode != fiber.StatusBadRequest {
		body, _ := io.ReadAll(diffResp.Body)
		t.Fatalf("expected diff status %d, got %d: %s", fiber.StatusBadRequest, diffResp.StatusCode, string(body))
	}

	var calls []models.ToolCall
	if err := database.DB.Order("id asc").Find(&calls).Error; err != nil {
		t.Fatalf("failed to load tool calls: %v", err)
	}
	if len(calls) != 2 {
		t.Fatalf("expected 2 failed audit rows, got %d", len(calls))
	}

	if calls[0].ToolName != "agent.read_file" || calls[0].Status != "failed" || calls[0].ExitCode != 2 {
		t.Fatalf("unexpected read audit row: %+v", calls[0])
	}
	if !strings.Contains(calls[0].Input, `"/etc/passwd"`) {
		t.Fatalf("read audit should preserve rejected path context: %s", calls[0].Input)
	}
	if !strings.Contains(calls[0].Error, "path must stay inside /workspace") {
		t.Fatalf("read audit should record validation error, got %q", calls[0].Error)
	}

	if calls[1].ToolName != "agent.git_diff" || calls[1].Status != "failed" || calls[1].ExitCode != 2 {
		t.Fatalf("unexpected git diff audit row: %+v", calls[1])
	}
	if !strings.Contains(calls[1].Error, "agent_session_id must be a positive integer") {
		t.Fatalf("git diff audit should record session validation error, got %q", calls[1].Error)
	}
}

func TestProjectExecCanonicalAndAgentAliasRoutes(t *testing.T) {
	setupProjectHandlerTestDB(t)
	seedOwnedProjectSandbox(t)

	app := fiber.New()
	handler := NewProjectHandler(nil)
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("userID", uint(7))
		return c.Next()
	})
	app.Post("/projects/:projectId/exec", handler.RunCommand)
	app.Post("/projects/:projectId/agent/exec", handler.RunCommand)

	for _, route := range []string{"/projects/1/exec", "/projects/1/agent/exec"} {
		req := httptest.NewRequest("POST", route, strings.NewReader(`{}`))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("request to %s failed: %v", route, err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != fiber.StatusBadRequest {
			t.Fatalf("expected status %d from %s, got %d: %s", fiber.StatusBadRequest, route, resp.StatusCode, string(body))
		}
		if !strings.Contains(string(body), "command is required") {
			t.Fatalf("expected command validation from %s, got %s", route, string(body))
		}
	}

	var count int64
	if err := database.DB.Model(&models.ToolCall{}).Where("tool_name = ? AND status = ?", "agent.run_command", "failed").Count(&count).Error; err != nil {
		t.Fatalf("failed to count tool calls: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected both exec routes to create failed audit rows, got %d", count)
	}
}

func TestFirstFileFromTarReaderLimitedReportsTruncation(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	content := []byte("0123456789")
	if err := tw.WriteHeader(&tar.Header{Name: "large.txt", Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatalf("failed to write tar header: %v", err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatalf("failed to write tar body: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("failed to close tar writer: %v", err)
	}

	name, fileBytes, truncated, err := firstFileFromTarReaderLimited(bytes.NewReader(buf.Bytes()), 4)
	if err != nil {
		t.Fatalf("firstFileFromTarReaderLimited returned error: %v", err)
	}
	if name != "large.txt" {
		t.Fatalf("expected large.txt, got %q", name)
	}
	if string(fileBytes) != "0123" {
		t.Fatalf("expected capped content, got %q", string(fileBytes))
	}
	if !truncated {
		t.Fatal("expected truncated=true")
	}
}

func setupProjectHandlerTestDB(t *testing.T) {
	t.Helper()
	if err := database.Connect(filepath.Join(t.TempDir(), "dobox-test.db")); err != nil {
		t.Fatalf("failed to initialize test database: %v", err)
	}
}

func seedOwnedProjectSandbox(t *testing.T) {
	t.Helper()
	project := models.Project{
		ID:        1,
		UserID:    7,
		Name:      "agent task",
		Workspace: defaultWorkspacePath,
		SandboxID: 2,
	}
	if err := database.DB.Create(&project).Error; err != nil {
		t.Fatalf("failed to create project: %v", err)
	}
	sandbox := models.Sandbox{
		ID:            2,
		UserID:        7,
		ProjectID:     1,
		ContainerID:   "container-secret",
		Name:          "dobox-p1-sandbox",
		Image:         defaultSandboxImage,
		Status:        "running",
		WorkspacePath: defaultWorkspacePath,
		VolumeName:    "dobox_project_1",
		NetworkName:   "dobox_project_1",
		CPULimit:      defaultCPULimit,
		MemoryLimit:   defaultMemoryLimit,
	}
	if err := database.DB.Create(&sandbox).Error; err != nil {
		t.Fatalf("failed to create sandbox: %v", err)
	}
}
