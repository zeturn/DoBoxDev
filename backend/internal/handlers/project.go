package handlers

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"docode/internal/database"
	"docode/internal/docker"
	"docode/internal/models"

	"github.com/gofiber/fiber/v2"
)

const (
	defaultProjectImage = "dobox/code-sandbox:latest"
	workspaceMountPath  = "/workspace"
)

type ProjectHandler struct {
	dockerService *docker.DockerService
	dataDir       string
}

func NewProjectHandler(dockerService *docker.DockerService, dataDir string) *ProjectHandler {
	if strings.TrimSpace(dataDir) == "" {
		dataDir = "./data"
	}
	return &ProjectHandler{dockerService: dockerService, dataDir: dataDir}
}

type CreateProjectRequest struct {
	Name        string `json:"name"`
	RepoURL     string `json:"repo_url"`
	Branch      string `json:"branch"`
	Image       string `json:"image"`
	NetworkMode string `json:"network_mode"`
}

type CreateAgentSessionRequest struct {
	Name string `json:"name"`
}

type ExecProjectRequest struct {
	Command        any    `json:"command"`
	WorkingDir     string `json:"working_dir"`
	TimeoutSec     int    `json:"timeout_sec"`
	OutputLimit    int    `json:"output_limit"`
	AgentSessionID *uint  `json:"agent_session_id"`
}

type FilePathRequest struct {
	Path           string `json:"path"`
	AgentSessionID *uint  `json:"agent_session_id"`
}

type FileWriteRequest struct {
	Path           string `json:"path"`
	Content        string `json:"content"`
	AgentSessionID *uint  `json:"agent_session_id"`
}

type SearchRequest struct {
	Query          string `json:"query"`
	Path           string `json:"path"`
	AgentSessionID *uint  `json:"agent_session_id"`
}

type CommitRequest struct {
	Message        string `json:"message"`
	AgentSessionID *uint  `json:"agent_session_id"`
}

func (h *ProjectHandler) CreateProject(c *fiber.Ctx) error {
	var req CreateProjectRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	if strings.TrimSpace(req.Name) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name is required"})
	}

	projectID := randomID(12)
	image := strings.TrimSpace(req.Image)
	if image == "" {
		image = defaultProjectImage
	}
	networkMode, err := projectNetworkMode(req.NetworkMode)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	workspace := filepath.Join(h.dataDir, "projects", projectID, "workspace")
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if err := h.prepareWorkspace(workspace, req.RepoURL, req.Branch); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	containerID, err := h.dockerService.CreateContainer(ctx, docker.CreateContainerOptions{
		Name:          "dobox-project-" + projectID,
		Image:         image,
		Volumes:       []string{hostVolume(workspace) + ":" + workspaceMountPath},
		Command:       []string{"sh", "-lc", "while true; do sleep 3600; done"},
		WorkingDir:    workspaceMountPath,
		RestartPolicy: "no",
		NetworkMode:   networkMode,
		CPULimit:      2,
		MemoryLimit:   2 * 1024 * 1024 * 1024,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create sandbox: " + err.Error()})
	}
	if err := h.dockerService.StartContainer(ctx, containerID); err != nil {
		_ = h.dockerService.RemoveContainer(context.Background(), containerID)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to start sandbox: " + err.Error()})
	}

	project := models.Project{
		ID:          projectID,
		Name:        req.Name,
		RepoURL:     req.RepoURL,
		Branch:      req.Branch,
		Image:       image,
		NetworkMode: networkMode,
		Workspace:   workspace,
		ContainerID: containerID,
		Status:      "running",
	}
	if err := database.DB.Create(&project).Error; err != nil {
		_ = h.dockerService.RemoveContainer(context.Background(), containerID)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to persist project: " + err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"project": project,
		"sandbox": fiber.Map{"id": containerID},
	})
}

func (h *ProjectHandler) GetProject(c *fiber.Ctx) error {
	project, err := h.getProject(c.Params("id"))
	if err != nil {
		return notFound(c, "project not found")
	}
	if status, err := h.dockerService.GetContainerStatus(context.Background(), project.ContainerID); err == nil {
		project.Status = status
		_ = database.DB.Save(project).Error
	}
	return c.JSON(project)
}

func (h *ProjectHandler) DeleteProject(c *fiber.Ctx) error {
	project, err := h.getProject(c.Params("id"))
	if err != nil {
		return notFound(c, "project not found")
	}
	_ = h.dockerService.RemoveContainer(context.Background(), project.ContainerID)
	_ = os.RemoveAll(filepath.Dir(project.Workspace))
	_ = database.DB.Where("project_id = ?", project.ID).Delete(&models.AgentSession{}).Error
	_ = database.DB.Delete(project).Error
	return c.JSON(fiber.Map{"message": "Project deleted"})
}

func (h *ProjectHandler) CreateAgentSession(c *fiber.Ctx) error {
	project, err := h.getProject(c.Params("id"))
	if err != nil {
		return notFound(c, "project not found")
	}
	var req CreateAgentSessionRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	if strings.TrimSpace(req.Name) == "" {
		req.Name = "agent"
	}
	session := models.AgentSession{ProjectID: project.ID, Name: req.Name}
	if err := database.DB.Create(&session).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(fiber.StatusCreated).JSON(session)
}

func (h *ProjectHandler) Exec(c *fiber.Ctx) error {
	project, err := h.getProject(c.Params("id"))
	if err != nil {
		return notFound(c, "project not found")
	}
	var req ExecProjectRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	command, err := normalizeCommand(req.Command)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	workingDir := strings.TrimSpace(req.WorkingDir)
	if workingDir == "" {
		workingDir = workspaceMountPath
	}
	if !containerWorkspacePathOK(workingDir) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "working_dir must stay under /workspace"})
	}
	timeoutSec := req.TimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = 120
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
	defer cancel()
	output, exitCode, err := h.dockerService.ExecInContainer(ctx, project.ContainerID, command, workingDir, nil)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error(), "output": output, "exit_code": exitCode})
	}
	limit := req.OutputLimit
	if limit <= 0 {
		limit = 1_000_000
	}
	truncated := false
	if len([]byte(output)) > limit {
		output = string([]byte(output)[:limit])
		truncated = true
	}
	return c.JSON(fiber.Map{"output": output, "exit_code": exitCode, "truncated": truncated})
}

func (h *ProjectHandler) ReadFile(c *fiber.Ctx) error {
	project, err := h.getProject(c.Params("id"))
	if err != nil {
		return notFound(c, "project not found")
	}
	var req FilePathRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	path, err := resolveWorkspacePath(project.Workspace, req.Path)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"path": path, "file_name": filepath.Base(path), "bytes": len(content), "content": string(content), "truncated": false})
}

func (h *ProjectHandler) WriteFile(c *fiber.Ctx) error {
	project, err := h.getProject(c.Params("id"))
	if err != nil {
		return notFound(c, "project not found")
	}
	var req FileWriteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	path, err := resolveWorkspacePath(project.Workspace, req.Path)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if err := os.WriteFile(path, []byte(req.Content), 0o644); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "File written"})
}

func (h *ProjectHandler) ListFiles(c *fiber.Ctx) error {
	project, err := h.getProject(c.Params("id"))
	if err != nil {
		return notFound(c, "project not found")
	}
	var req FilePathRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	path, err := resolveWorkspacePath(project.Workspace, defaultString(req.Path, "."))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	rows := []string{}
	if err := filepath.WalkDir(path, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if p == path {
			return nil
		}
		rel, _ := filepath.Rel(project.Workspace, p)
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			rel += "/"
		}
		rows = append(rows, rel)
		return nil
	}); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"output": strings.Join(rows, "\n"), "exit_code": 0, "truncated": false})
}

func (h *ProjectHandler) SearchFiles(c *fiber.Ctx) error {
	project, err := h.getProject(c.Params("id"))
	if err != nil {
		return notFound(c, "project not found")
	}
	var req SearchRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	root, err := resolveWorkspacePath(project.Workspace, defaultString(req.Path, "."))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	rows := []string{}
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			if d != nil && d.IsDir() && d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		content, readErr := os.ReadFile(p)
		if readErr != nil || bytes.Contains(content, []byte{0}) {
			return nil
		}
		for i, line := range strings.Split(string(content), "\n") {
			if strings.Contains(line, req.Query) {
				rel, _ := filepath.Rel(project.Workspace, p)
				rows = append(rows, fmt.Sprintf("%s:%d:%s", filepath.ToSlash(rel), i+1, line))
			}
		}
		return nil
	})
	return c.JSON(fiber.Map{"output": strings.Join(rows, "\n"), "exit_code": 0, "truncated": false})
}

func (h *ProjectHandler) GitStatus(c *fiber.Ctx) error {
	project, err := h.getProject(c.Params("id"))
	if err != nil {
		return notFound(c, "project not found")
	}
	out, code := runGit(project.Workspace, "status", "--short")
	return c.JSON(fiber.Map{"status": out, "exit_code": code, "truncated": false})
}

func (h *ProjectHandler) GitDiff(c *fiber.Ctx) error {
	project, err := h.getProject(c.Params("id"))
	if err != nil {
		return notFound(c, "project not found")
	}
	out, code := runGit(project.Workspace, "diff")
	return c.JSON(fiber.Map{"diff": out, "exit_code": code, "truncated": false})
}

func (h *ProjectHandler) GitCommit(c *fiber.Ctx) error {
	project, err := h.getProject(c.Params("id"))
	if err != nil {
		return notFound(c, "project not found")
	}
	var req CommitRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	if strings.TrimSpace(req.Message) == "" {
		req.Message = "DoCode changes"
	}
	runGit(project.Workspace, "add", ".")
	out, code := runGit(project.Workspace, "commit", "-m", req.Message)
	return c.JSON(fiber.Map{"output": out, "exit_code": code, "truncated": false})
}

func (h *ProjectHandler) Preview(c *fiber.Ctx) error {
	var req struct {
		Port int `json:"port"`
	}
	_ = c.BodyParser(&req)
	return c.JSON(fiber.Map{"status": "preview_descriptor", "port": req.Port, "message": "Preview proxy is not configured"})
}

func (h *ProjectHandler) Logs(c *fiber.Ctx) error {
	project, err := h.getProject(c.Params("id"))
	if err != nil {
		return notFound(c, "project not found")
	}
	tail := c.Query("tail", "200")
	logs, err := h.dockerService.GetContainerLogs(context.Background(), project.ContainerID, tail)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"logs": logs})
}

func (h *ProjectHandler) Archive(c *fiber.Ctx) error {
	project, err := h.getProject(c.Params("id"))
	if err != nil {
		return notFound(c, "project not found")
	}
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	err = filepath.WalkDir(project.Workspace, func(p string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(project.Workspace, p)
		if err != nil {
			return err
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		file, err := os.Open(p)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = io.Copy(tw, file)
		return err
	})
	if err != nil {
		_ = tw.Close()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if err := tw.Close(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	c.Set("Content-Type", "application/x-tar")
	return c.Send(buf.Bytes())
}

func (h *ProjectHandler) getProject(id string) (*models.Project, error) {
	var project models.Project
	if err := database.DB.Where("id = ?", id).First(&project).Error; err != nil {
		return nil, err
	}
	return &project, nil
}

func (h *ProjectHandler) prepareWorkspace(workspace, repoURL, branch string) error {
	if strings.TrimSpace(repoURL) == "" {
		return ensureGitRepository(workspace)
	}
	source := strings.TrimSpace(repoURL)
	if stat, err := os.Stat(source); err == nil && stat.IsDir() {
		if err := copyDir(source, workspace); err != nil {
			return err
		}
		return ensureGitRepository(workspace)
	}
	args := []string{"clone", source, workspace}
	if branch = strings.TrimSpace(branch); branch != "" {
		args = []string{"clone", "--branch", branch, "--single-branch", source, workspace}
	}
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %s", string(out))
	}
	return ensureGitConfig(workspace)
}

func ensureGitRepository(workspace string) error {
	if _, err := os.Stat(filepath.Join(workspace, ".git")); err == nil {
		return ensureGitConfig(workspace)
	}
	if out, err := exec.Command("git", "-C", workspace, "init").CombinedOutput(); err != nil {
		return fmt.Errorf("git init failed: %s", string(out))
	}
	if err := ensureGitConfig(workspace); err != nil {
		return err
	}
	if out, err := exec.Command("git", "-C", workspace, "add", ".").CombinedOutput(); err != nil {
		return fmt.Errorf("git add failed: %s", string(out))
	}
	if out, err := exec.Command("git", "-C", workspace, "commit", "-m", "initial").CombinedOutput(); err != nil {
		text := string(out)
		if !strings.Contains(strings.ToLower(text), "nothing to commit") {
			return fmt.Errorf("git commit failed: %s", text)
		}
	}
	return nil
}

func ensureGitConfig(workspace string) error {
	commands := [][]string{
		{"config", "user.email", "dobox@example.local"},
		{"config", "user.name", "DoBox"},
	}
	for _, args := range commands {
		if out, err := exec.Command("git", append([]string{"-C", workspace}, args...)...).CombinedOutput(); err != nil {
			return fmt.Errorf("git %s failed: %s", strings.Join(args, " "), string(out))
		}
	}
	return nil
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Name() == ".git" && d.IsDir() {
			return filepath.SkipDir
		}
		rel, err := filepath.Rel(src, path)
		if err != nil || rel == "." {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}

func runGit(workspace string, args ...string) (string, int) {
	cmd := exec.Command("git", append([]string{"-C", workspace}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return string(out), exit.ExitCode()
		}
		return string(out) + err.Error(), 1
	}
	return string(out), 0
}

func normalizeCommand(value any) ([]string, error) {
	switch command := value.(type) {
	case string:
		if strings.TrimSpace(command) == "" {
			return nil, fmt.Errorf("command is required")
		}
		return []string{"sh", "-lc", command}, nil
	case []any:
		result := make([]string, 0, len(command))
		for _, item := range command {
			text, ok := item.(string)
			if !ok || strings.TrimSpace(text) == "" {
				return nil, fmt.Errorf("command entries must be non-empty strings")
			}
			result = append(result, text)
		}
		if len(result) == 0 {
			return nil, fmt.Errorf("command is required")
		}
		return result, nil
	default:
		return nil, fmt.Errorf("command must be a string or string array")
	}
}

func resolveWorkspacePath(workspace, raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		raw = "."
	}
	clean := filepath.ToSlash(strings.TrimSpace(raw))
	clean = strings.TrimPrefix(clean, workspaceMountPath)
	clean = strings.TrimPrefix(clean, "/")
	target := filepath.Clean(filepath.Join(workspace, filepath.FromSlash(clean)))
	root, err := filepath.Abs(workspace)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	if abs != root && !strings.HasPrefix(abs, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("path must stay under %s", workspaceMountPath)
	}
	return abs, nil
}

func containerWorkspacePathOK(path string) bool {
	clean := filepath.ToSlash(filepath.Clean(path))
	return clean == workspaceMountPath || strings.HasPrefix(clean, workspaceMountPath+"/")
}

func projectNetworkMode(value string) (string, error) {
	mode := strings.TrimSpace(value)
	switch mode {
	case "", "project":
		return "bridge", nil
	case "no_internet":
		return "none", nil
	default:
		return "", fmt.Errorf("unsupported network_mode: %s", mode)
	}
}

func hostVolume(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func notFound(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": message})
}

func randomID(bytesLen int) string {
	buf := make([]byte, bytesLen)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}
