package handlers

import (
	"bufio"
	"context"
	"docode/internal/database"
	"docode/internal/docker"
	"docode/internal/middleware"
	"docode/internal/models"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

const (
	defaultSandboxImage          = "dobox/code-sandbox:latest"
	defaultWorkspacePath         = "/workspace"
	defaultCPULimit              = 2.0
	defaultMemoryLimit           = int64(2 * 1024 * 1024 * 1024)
	defaultPidsLimit             = int64(512)
	defaultCommandTimeoutSeconds = 120
	maxCommandTimeoutSeconds     = 300
	defaultOutputLimitBytes      = int64(1_000_000)
)

type ProjectHandler struct {
	dockerService *docker.DockerService

	// archiveDownload fetches the workspace tar stream. It is a field (not a
	// direct dockerService call) so unit tests can substitute a fake stream
	// without a live Docker daemon. Defaults to dockerService.DownloadFromContainer.
	archiveDownload func(ctx context.Context, containerID, sourcePath string) (io.ReadCloser, error)
}

func NewProjectHandler(dockerService *docker.DockerService) *ProjectHandler {
	h := &ProjectHandler{dockerService: dockerService}
	if dockerService != nil {
		h.archiveDownload = dockerService.DownloadFromContainer
	}
	return h
}

type CreateProjectRequest struct {
	Name        string  `json:"name"`
	RepoURL     string  `json:"repo_url"`
	Branch      string  `json:"branch"`
	Image       string  `json:"image"`
	Workspace   string  `json:"workspace"`
	NetworkMode string  `json:"network_mode"`
	CPULimit    float64 `json:"cpu_limit"`
	MemoryLimit int64   `json:"memory_limit"`
}

type ProjectResponse struct {
	Project PublicProject `json:"project"`
	Sandbox PublicSandbox `json:"sandbox"`
}

type PublicProject struct {
	ID        uint      `json:"id"`
	UserID    uint      `json:"user_id"`
	Name      string    `json:"name"`
	RepoURL   string    `json:"repo_url"`
	Branch    string    `json:"branch"`
	Workspace string    `json:"workspace"`
	SandboxID uint      `json:"sandbox_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type PublicSandbox struct {
	ID            uint      `json:"id"`
	UserID        uint      `json:"user_id"`
	ProjectID     uint      `json:"project_id"`
	Name          string    `json:"name"`
	Image         string    `json:"image"`
	Status        string    `json:"status"`
	WorkspacePath string    `json:"workspace_path"`
	CPULimit      float64   `json:"cpu_limit"`
	MemoryLimit   int64     `json:"memory_limit"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (h *ProjectHandler) ListProjects(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var projects []models.Project
	if err := database.DB.Preload("Sandbox").Where("user_id = ?", userID).Find(&projects).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch projects"})
	}

	for i := range projects {
		if projects[i].Sandbox == nil || projects[i].Sandbox.ContainerID == "" {
			continue
		}
		if status, err := h.dockerService.GetContainerStatus(context.Background(), projects[i].Sandbox.ContainerID); err == nil {
			projects[i].Sandbox.Status = status
			_ = database.DB.Model(projects[i].Sandbox).Update("status", status).Error
		}
	}

	responses := make([]ProjectResponse, 0, len(projects))
	for i := range projects {
		if projects[i].Sandbox == nil {
			continue
		}
		responses = append(responses, publicProjectResponse(&projects[i], projects[i].Sandbox))
	}

	return c.JSON(responses)
}

func (h *ProjectHandler) GetProject(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	projectID := c.Params("projectId")

	project, sandbox, err := h.getOwnedProjectSandbox(userID, projectID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Project not found"})
	}
	if status, err := h.dockerService.GetContainerStatus(context.Background(), sandbox.ContainerID); err == nil {
		sandbox.Status = status
		_ = database.DB.Model(sandbox).Update("status", status).Error
	}
	return c.JSON(publicProjectResponse(project, sandbox))
}

func (h *ProjectHandler) CreateProject(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req CreateProjectRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name is required"})
	}

	workspace, err := sandboxWorkspace(req.Workspace)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	image, err := sandboxImage(req.Image)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	networkInternal, err := sandboxNetworkInternal(req.NetworkMode)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	cpuLimit := sandboxCPULimit(req.CPULimit)
	memoryLimit := sandboxMemoryLimit(req.MemoryLimit)

	project := models.Project{
		UserID:    userID,
		Name:      req.Name,
		RepoURL:   strings.TrimSpace(req.RepoURL),
		Branch:    strings.TrimSpace(req.Branch),
		Workspace: workspace,
	}
	if err := database.DB.Create(&project).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create project"})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	volumeName := fmt.Sprintf("dobox_project_%d", project.ID)
	networkName := fmt.Sprintf("dobox_project_%d", project.ID)
	containerName := fmt.Sprintf("dobox-p%d-sandbox", project.ID)

	createdContainerID := ""
	createdNetwork := false
	createdVolume := false
	cleanup := func() {
		if createdContainerID != "" {
			_ = h.dockerService.RemoveContainer(context.Background(), createdContainerID)
		}
		if createdNetwork {
			_ = h.dockerService.RemoveNetwork(context.Background(), networkName)
		}
		if createdVolume {
			_ = h.dockerService.RemoveVolume(context.Background(), volumeName, true)
		}
		_ = database.DB.Where("project_id = ?", project.ID).Delete(&models.ToolCall{}).Error
		_ = database.DB.Where("project_id = ?", project.ID).Delete(&models.AgentSession{}).Error
		_ = database.DB.Where("project_id = ?", project.ID).Delete(&models.Sandbox{}).Error
		_ = database.DB.Delete(&project).Error
	}

	if _, err := h.dockerService.CreateVolume(ctx, volumeName, "local", map[string]string{
		"dobox.project_id": strconv.FormatUint(uint64(project.ID), 10),
		"dobox.user_id":    strconv.FormatUint(uint64(userID), 10),
	}); err != nil {
		cleanup()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create project volume: " + err.Error()})
	}
	createdVolume = true

	if _, err := h.dockerService.CreateNetwork(ctx, networkName, "bridge", false, networkInternal); err != nil {
		cleanup()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create project network: " + err.Error()})
	}
	createdNetwork = true

	containerID, err := h.dockerService.CreateSandboxContainer(ctx, docker.CreateSandboxOptions{
		Name:          containerName,
		Image:         image,
		VolumeName:    volumeName,
		NetworkName:   networkName,
		WorkspacePath: workspace,
		User:          docker.DefaultSandboxUser,
		CPULimit:      cpuLimit,
		MemoryLimit:   memoryLimit,
		PidsLimit:     defaultPidsLimit,
	})
	if err != nil {
		cleanup()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create project sandbox: " + err.Error()})
	}
	createdContainerID = containerID

	if err := h.dockerService.StartContainer(ctx, containerID); err != nil {
		cleanup()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to start project sandbox: " + err.Error()})
	}

	sandbox := models.Sandbox{
		UserID:        userID,
		ProjectID:     project.ID,
		ContainerID:   containerID,
		Name:          containerName,
		Image:         image,
		Status:        "running",
		WorkspacePath: workspace,
		VolumeName:    volumeName,
		NetworkName:   networkName,
		CPULimit:      cpuLimit,
		MemoryLimit:   memoryLimit,
	}
	if err := database.DB.Create(&sandbox).Error; err != nil {
		cleanup()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save project sandbox"})
	}
	project.SandboxID = sandbox.ID
	if err := database.DB.Save(&project).Error; err != nil {
		cleanup()
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to link project sandbox"})
	}

	if project.RepoURL != "" {
		if output, exitCode, err := h.cloneRepo(ctx, sandbox, project.RepoURL, project.Branch); err != nil || exitCode != 0 {
			cleanup()
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to clone repository: " + err.Error()})
			}
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to clone repository", "output": output, "exit_code": exitCode})
		}
	}

	return c.Status(fiber.StatusCreated).JSON(publicProjectResponse(&project, &sandbox))
}

func (h *ProjectHandler) DeleteProject(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	projectID := c.Params("projectId")

	project, sandbox, err := h.getOwnedProjectSandbox(userID, projectID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Project not found"})
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	if sandbox.ContainerID != "" {
		_ = h.dockerService.RemoveContainer(ctx, sandbox.ContainerID)
	}
	if sandbox.NetworkName != "" {
		_ = h.dockerService.RemoveNetwork(ctx, sandbox.NetworkName)
	}
	if sandbox.VolumeName != "" {
		_ = h.dockerService.RemoveVolume(ctx, sandbox.VolumeName, true)
	}
	_ = database.DB.Where("project_id = ?", project.ID).Delete(&models.ToolCall{}).Error
	_ = database.DB.Where("project_id = ?", project.ID).Delete(&models.AgentSession{}).Error
	_ = database.DB.Delete(sandbox).Error
	_ = database.DB.Delete(project).Error

	return c.JSON(fiber.Map{"message": "Project deleted successfully"})
}

func (h *ProjectHandler) CreateAgentSession(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	projectID := c.Params("projectId")

	project, _, err := h.getOwnedProjectSandbox(userID, projectID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Project not found"})
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	session := models.AgentSession{
		UserID:    userID,
		ProjectID: project.ID,
		Name:      strings.TrimSpace(req.Name),
		Status:    "active",
	}
	if err := database.DB.Create(&session).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create agent session"})
	}
	return c.Status(fiber.StatusCreated).JSON(session)
}

func (h *ProjectHandler) ListAgentSessions(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	projectID := c.Params("projectId")

	project, _, err := h.getOwnedProjectSandbox(userID, projectID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Project not found"})
	}

	var sessions []models.AgentSession
	if err := database.DB.Where("user_id = ? AND project_id = ?", userID, project.ID).Order("created_at DESC").Find(&sessions).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch agent sessions"})
	}
	return c.JSON(sessions)
}

func (h *ProjectHandler) ListToolCalls(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	projectID := c.Params("projectId")

	project, _, err := h.getOwnedProjectSandbox(userID, projectID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Project not found"})
	}

	query := database.DB.Where("user_id = ? AND project_id = ?", userID, project.ID)
	sessionID, ok := h.toolSessionFromQuery(c, userID, project.ID)
	if !ok {
		return nil
	}
	if sessionID > 0 {
		query = query.Where("agent_session_id = ?", sessionID)
	}

	var calls []models.ToolCall
	if err := query.Order("created_at DESC").Limit(auditListLimit(c.Query("limit"))).Find(&calls).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch tool calls"})
	}
	return c.JSON(calls)
}

func (h *ProjectHandler) RunCommand(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	project, sandbox, ok := h.loadToolSandbox(c, userID)
	if !ok {
		return nil
	}

	var req struct {
		Command        json.RawMessage `json:"command"`
		CWD            string          `json:"cwd"`
		WorkingDir     string          `json:"working_dir"`
		Env            []string        `json:"env"`
		TimeoutSec     int             `json:"timeout_sec"`
		OutputLimit    int64           `json:"output_limit"`
		AgentSessionID uint            `json:"agent_session_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return h.failToolCall(c, userID, project.ID, 0, "agent.run_command", invalidBodyAuditInput(c), fiber.StatusBadRequest, "Invalid request body")
	}
	sessionID, ok := h.validateToolSessionForTool(c, userID, project.ID, req.AgentSessionID, "agent.run_command", req)
	if !ok {
		return nil
	}

	cmd, err := parseCommand(req.Command)
	if err != nil {
		return h.failToolCall(c, userID, project.ID, sessionID, "agent.run_command", req, fiber.StatusBadRequest, err.Error())
	}
	workDirInput := req.WorkingDir
	if strings.TrimSpace(workDirInput) == "" {
		workDirInput = req.CWD
	}
	workDir, err := resolveSandboxPath(sandbox.WorkspacePath, workDirInput)
	if err != nil {
		return h.failToolCall(c, userID, project.ID, sessionID, "agent.run_command", req, fiber.StatusBadRequest, err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout(req.TimeoutSec))
	defer cancel()
	output, exitCode, truncated, execErr := h.dockerService.ExecInContainerLimited(ctx, sandbox.ContainerID, cmd, workDir, req.Env, outputLimit(req.OutputLimit))
	h.recordToolCall(userID, project.ID, sessionID, "agent.run_command", req, output, exitCode, execErr)
	if execErr != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to run command: " + execErr.Error()})
	}
	return c.JSON(fiber.Map{"output": output, "exit_code": exitCode, "truncated": truncated})
}

func (h *ProjectHandler) ReadFile(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	project, sandbox, ok := h.loadToolSandbox(c, userID)
	if !ok {
		return nil
	}

	var req struct {
		Path           string `json:"path"`
		AgentSessionID uint   `json:"agent_session_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return h.failToolCall(c, userID, project.ID, 0, "agent.read_file", invalidBodyAuditInput(c), fiber.StatusBadRequest, "Invalid request body")
	}
	sessionID, ok := h.validateToolSessionForTool(c, userID, project.ID, req.AgentSessionID, "agent.read_file", req)
	if !ok {
		return nil
	}
	sourcePath, err := resolveSandboxPath(sandbox.WorkspacePath, req.Path)
	if err != nil {
		return h.failToolCall(c, userID, project.ID, sessionID, "agent.read_file", req, fiber.StatusBadRequest, err.Error())
	}

	reader, err := h.dockerService.DownloadFromContainer(context.Background(), sandbox.ContainerID, sourcePath)
	if err != nil {
		h.recordToolCall(userID, project.ID, sessionID, "agent.read_file", req, "", 0, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to read file: " + err.Error()})
	}
	defer reader.Close()

	name, fileBytes, truncated, err := firstFileFromTarReaderLimited(reader, defaultOutputLimitBytes)
	if err != nil {
		h.recordToolCall(userID, project.ID, sessionID, "agent.read_file", req, "", 0, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to extract file: " + err.Error()})
	}
	content := string(fileBytes)
	outputSummary := fmt.Sprintf("read %d bytes", len(fileBytes))
	if truncated {
		outputSummary += " (truncated)"
	}
	h.recordToolCall(userID, project.ID, sessionID, "agent.read_file", req, outputSummary, 0, nil)
	return c.JSON(fiber.Map{
		"file_name":      name,
		"path":           sourcePath,
		"content":        content,
		"content_base64": base64.StdEncoding.EncodeToString(fileBytes),
		"bytes":          len(fileBytes),
		"truncated":      truncated,
	})
}

func (h *ProjectHandler) WriteFile(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	project, sandbox, ok := h.loadToolSandbox(c, userID)
	if !ok {
		return nil
	}

	var req struct {
		Path           string `json:"path"`
		Content        string `json:"content"`
		ContentBase64  string `json:"content_base64"`
		AgentSessionID uint   `json:"agent_session_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return h.failToolCall(c, userID, project.ID, 0, "agent.write_file", invalidBodyAuditInput(c), fiber.StatusBadRequest, "Invalid request body")
	}
	sessionID, ok := h.validateToolSessionForTool(c, userID, project.ID, req.AgentSessionID, "agent.write_file", req)
	if !ok {
		return nil
	}
	targetPath, err := resolveSandboxPath(sandbox.WorkspacePath, req.Path)
	if err != nil {
		return h.failToolCall(c, userID, project.ID, sessionID, "agent.write_file", req, fiber.StatusBadRequest, err.Error())
	}
	if targetPath == sandbox.WorkspacePath || strings.HasSuffix(targetPath, "/") {
		return h.failToolCall(c, userID, project.ID, sessionID, "agent.write_file", req, fiber.StatusBadRequest, "path must identify a file")
	}

	data := []byte(req.Content)
	if req.ContentBase64 != "" {
		data, err = base64.StdEncoding.DecodeString(req.ContentBase64)
		if err != nil {
			return h.failToolCall(c, userID, project.ID, sessionID, "agent.write_file", req, fiber.StatusBadRequest, "content_base64 is not valid base64")
		}
	}

	dir := path.Dir(targetPath)
	fileName := path.Base(targetPath)
	if dir != sandbox.WorkspacePath {
		output, exitCode, err := h.dockerService.ExecInContainer(context.Background(), sandbox.ContainerID, []string{"mkdir", "-p", dir}, sandbox.WorkspacePath, nil)
		if err != nil || exitCode != 0 {
			if err == nil {
				err = fmt.Errorf("mkdir exited with code %d: %s", exitCode, strings.TrimSpace(output))
			}
			h.recordToolCall(userID, project.ID, sessionID, "agent.write_file", req, output, exitCode, err)
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to prepare destination: " + err.Error()})
		}
	}
	if err := h.dockerService.UploadFileToContainer(context.Background(), sandbox.ContainerID, dir, fileName, data); err != nil {
		h.recordToolCall(userID, project.ID, sessionID, "agent.write_file", req, "", 0, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to write file: " + err.Error()})
	}
	h.recordToolCall(userID, project.ID, sessionID, "agent.write_file", req, fmt.Sprintf("wrote %d bytes", len(data)), 0, nil)
	return c.JSON(fiber.Map{"message": "File written successfully", "path": targetPath, "bytes": len(data)})
}

func (h *ProjectHandler) ListFiles(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	project, sandbox, ok := h.loadToolSandbox(c, userID)
	if !ok {
		return nil
	}

	var req struct {
		Path           string `json:"path"`
		AgentSessionID uint   `json:"agent_session_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return h.failToolCall(c, userID, project.ID, 0, "agent.list_files", invalidBodyAuditInput(c), fiber.StatusBadRequest, "Invalid request body")
	}
	sessionID, ok := h.validateToolSessionForTool(c, userID, project.ID, req.AgentSessionID, "agent.list_files", req)
	if !ok {
		return nil
	}
	targetPath, err := resolveSandboxPath(sandbox.WorkspacePath, req.Path)
	if err != nil {
		return h.failToolCall(c, userID, project.ID, sessionID, "agent.list_files", req, fiber.StatusBadRequest, err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout(0))
	defer cancel()
	cmd := []string{"sh", "-c", "ls -la " + shellQuote(targetPath)}
	output, exitCode, truncated, execErr := h.dockerService.ExecInContainerLimited(ctx, sandbox.ContainerID, cmd, sandbox.WorkspacePath, nil, defaultOutputLimitBytes)
	h.recordToolCall(userID, project.ID, sessionID, "agent.list_files", req, output, exitCode, execErr)
	if execErr != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to list files: " + execErr.Error()})
	}
	return c.JSON(fiber.Map{"path": targetPath, "output": output, "exit_code": exitCode, "truncated": truncated})
}

func (h *ProjectHandler) Search(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	project, sandbox, ok := h.loadToolSandbox(c, userID)
	if !ok {
		return nil
	}

	var req struct {
		Query          string `json:"query"`
		Path           string `json:"path"`
		AgentSessionID uint   `json:"agent_session_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return h.failToolCall(c, userID, project.ID, 0, "agent.search", invalidBodyAuditInput(c), fiber.StatusBadRequest, "Invalid request body")
	}
	sessionID, ok := h.validateToolSessionForTool(c, userID, project.ID, req.AgentSessionID, "agent.search", req)
	if !ok {
		return nil
	}
	query := strings.TrimSpace(req.Query)
	if query == "" {
		return h.failToolCall(c, userID, project.ID, sessionID, "agent.search", req, fiber.StatusBadRequest, "query is required")
	}
	targetPath, err := resolveSandboxPath(sandbox.WorkspacePath, req.Path)
	if err != nil {
		return h.failToolCall(c, userID, project.ID, sessionID, "agent.search", req, fiber.StatusBadRequest, err.Error())
	}

	cmd := []string{"sh", "-c", "grep -RIn --exclude-dir=.git -- " + shellQuote(query) + " " + shellQuote(targetPath)}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	output, exitCode, truncated, execErr := h.dockerService.ExecInContainerLimited(ctx, sandbox.ContainerID, cmd, sandbox.WorkspacePath, nil, defaultOutputLimitBytes)
	h.recordToolCall(userID, project.ID, sessionID, "agent.search", req, output, exitCode, execErr)
	if execErr != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to search: " + execErr.Error()})
	}
	return c.JSON(fiber.Map{"output": output, "exit_code": exitCode, "truncated": truncated})
}

func (h *ProjectHandler) GitDiff(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	project, sandbox, ok := h.loadToolSandbox(c, userID)
	if !ok {
		return nil
	}
	sessionID, ok := h.toolSessionFromQueryForTool(c, userID, project.ID, "agent.git_diff", fiber.Map{})
	if !ok {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout(0))
	defer cancel()
	output, exitCode, truncated, err := h.dockerService.ExecInContainerLimited(
		ctx,
		sandbox.ContainerID,
		[]string{"git", "--no-pager", "-C", sandbox.WorkspacePath, "diff", "--"},
		sandbox.WorkspacePath,
		nil,
		defaultOutputLimitBytes,
	)
	h.recordToolCall(userID, project.ID, sessionID, "agent.git_diff", fiber.Map{}, output, exitCode, err)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get git diff: " + err.Error()})
	}
	return c.JSON(fiber.Map{"diff": output, "exit_code": exitCode, "truncated": truncated})
}

func (h *ProjectHandler) GitStatus(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	project, sandbox, ok := h.loadToolSandbox(c, userID)
	if !ok {
		return nil
	}
	sessionID, ok := h.toolSessionFromQueryForTool(c, userID, project.ID, "agent.git_status", fiber.Map{})
	if !ok {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout(0))
	defer cancel()
	output, exitCode, truncated, err := h.dockerService.ExecInContainerLimited(ctx, sandbox.ContainerID, []string{"git", "-C", sandbox.WorkspacePath, "status", "--short"}, sandbox.WorkspacePath, nil, defaultOutputLimitBytes)
	h.recordToolCall(userID, project.ID, sessionID, "agent.git_status", fiber.Map{}, output, exitCode, err)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get git status: " + err.Error()})
	}
	return c.JSON(fiber.Map{"status": output, "exit_code": exitCode, "truncated": truncated})
}

func (h *ProjectHandler) GitCommit(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	project, sandbox, ok := h.loadToolSandbox(c, userID)
	if !ok {
		return nil
	}

	var req struct {
		Message        string `json:"message"`
		AgentSessionID uint   `json:"agent_session_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return h.failToolCall(c, userID, project.ID, 0, "agent.git_commit", invalidBodyAuditInput(c), fiber.StatusBadRequest, "Invalid request body")
	}
	sessionID, ok := h.validateToolSessionForTool(c, userID, project.ID, req.AgentSessionID, "agent.git_commit", req)
	if !ok {
		return nil
	}
	message := strings.TrimSpace(req.Message)
	if message == "" {
		return h.failToolCall(c, userID, project.ID, sessionID, "agent.git_commit", req, fiber.StatusBadRequest, "message is required")
	}

	cmd := []string{"sh", "-c", "git -C " + shellQuote(sandbox.WorkspacePath) + " add -A && git -C " + shellQuote(sandbox.WorkspacePath) + " commit -m " + shellQuote(message)}
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout(0))
	defer cancel()
	output, exitCode, truncated, err := h.dockerService.ExecInContainerLimited(ctx, sandbox.ContainerID, cmd, sandbox.WorkspacePath, nil, defaultOutputLimitBytes)
	h.recordToolCall(userID, project.ID, sessionID, "agent.git_commit", req, output, exitCode, err)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to commit: " + err.Error()})
	}
	return c.JSON(fiber.Map{"output": output, "exit_code": exitCode, "truncated": truncated})
}

func (h *ProjectHandler) Preview(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	project, sandbox, ok := h.loadToolSandbox(c, userID)
	if !ok {
		return nil
	}

	var req struct {
		Port           int  `json:"port"`
		AgentSessionID uint `json:"agent_session_id"`
	}
	if err := c.BodyParser(&req); err != nil {
		return h.failToolCall(c, userID, project.ID, 0, "agent.preview", invalidBodyAuditInput(c), fiber.StatusBadRequest, "Invalid request body")
	}
	sessionID, ok := h.validateToolSessionForTool(c, userID, project.ID, req.AgentSessionID, "agent.preview", req)
	if !ok {
		return nil
	}
	if req.Port < 1 || req.Port > 65535 {
		return h.failToolCall(c, userID, project.ID, sessionID, "agent.preview", req, fiber.StatusBadRequest, "valid port is required")
	}

	result := previewDescriptor(project.ID, sandbox.ID, req.Port)
	h.recordToolCall(userID, project.ID, sessionID, "agent.preview", req, fmt.Sprintf("preview port %d", req.Port), 0, nil)
	return c.JSON(result)
}

func (h *ProjectHandler) GetLogs(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	project, sandbox, ok := h.loadToolSandbox(c, userID)
	if !ok {
		return nil
	}
	tail := c.Query("tail", "200")
	since := c.Query("since", "")
	until := c.Query("until", "")
	sessionID, ok := h.toolSessionFromQueryForTool(c, userID, project.ID, "agent.logs", fiber.Map{"tail": tail, "since": since, "until": until})
	if !ok {
		return nil
	}

	logs, err := h.dockerService.GetContainerLogsWithOptions(context.Background(), sandbox.ContainerID, docker.LogOptions{
		Tail:  tail,
		Since: since,
		Until: until,
	})
	h.recordToolCall(userID, project.ID, sessionID, "agent.logs", fiber.Map{"tail": tail, "since": since, "until": until}, logs, 0, err)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get logs: " + err.Error()})
	}
	return c.JSON(fiber.Map{"logs": logs})
}

func (h *ProjectHandler) ArchiveWorkspace(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	project, sandbox, ok := h.loadToolSandbox(c, userID)
	if !ok {
		return nil
	}
	sessionID, ok := h.toolSessionFromQueryForTool(c, userID, project.ID, "agent.archive", fiber.Map{"path": sandbox.WorkspacePath})
	if !ok {
		return nil
	}

	reader, err := h.archiveDownload(context.Background(), sandbox.ContainerID, sandbox.WorkspacePath)
	if err != nil {
		h.recordToolCall(userID, project.ID, sessionID, "agent.archive", fiber.Map{"path": sandbox.WorkspacePath}, "", 0, err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to archive workspace: " + err.Error()})
	}

	c.Set("Content-Type", "application/x-tar")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"dobox-project-%d-workspace.tar\"", project.ID))

	// Stream the Docker tar directly to the client. The reader must stay open
	// until the copy completes: closing it in a deferred call races with
	// fasthttp's lazy body write and truncates the stream (Case B — client
	// sees RemoteProtocolError / "Server disconnected without sending a
	// response"). The delivery outcome is audited only after the copy and
	// close actually happen, so a disconnect is never recorded as success.
	c.Context().SetBodyStreamWriter(func(w *bufio.Writer) {
		copied, copyErr := io.Copy(w, reader)
		flushErr := w.Flush()
		closeErr := reader.Close()

		deliveryErr := copyErr
		if deliveryErr == nil {
			deliveryErr = flushErr
		}
		if deliveryErr == nil {
			deliveryErr = closeErr
		}
		h.recordToolCall(
			userID,
			project.ID,
			sessionID,
			"agent.archive",
			fiber.Map{
				"path":         sandbox.WorkspacePath,
				"project_id":   project.ID,
				"session_id":   sessionID,
				"bytes_copied": copied,
				"copy_error":   errString(copyErr),
				"flush_error":  errString(flushErr),
				"close_error":  errString(closeErr),
			},
			"", 0, deliveryErr,
		)
	})

	return nil
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (h *ProjectHandler) cloneRepo(ctx context.Context, sandbox models.Sandbox, repoURL, branch string) (string, int, error) {
	cleanupOutput, cleanupExitCode, cleanupErr := h.dockerService.ExecInContainer(
		ctx,
		sandbox.ContainerID,
		[]string{"sh", "-lc", "find . -mindepth 1 -maxdepth 1 -exec rm -rf -- {} +"},
		sandbox.WorkspacePath,
		nil,
	)
	if cleanupErr != nil || cleanupExitCode != 0 {
		return cleanupOutput, cleanupExitCode, cleanupErr
	}
	cmd := []string{"git", "clone", "--depth", "1"}
	if strings.TrimSpace(branch) != "" {
		cmd = append(cmd, "--branch", strings.TrimSpace(branch))
	}
	cmd = append(cmd, repoURL, ".")
	return h.dockerService.ExecInContainer(ctx, sandbox.ContainerID, cmd, sandbox.WorkspacePath, nil)
}

func (h *ProjectHandler) getOwnedProjectSandbox(userID uint, projectID string) (*models.Project, *models.Sandbox, error) {
	var project models.Project
	if err := database.DB.Where("id = ? AND user_id = ?", projectID, userID).First(&project).Error; err != nil {
		return nil, nil, err
	}
	var sandbox models.Sandbox
	if err := database.DB.Where("project_id = ? AND user_id = ?", project.ID, userID).First(&sandbox).Error; err != nil {
		return nil, nil, err
	}
	return &project, &sandbox, nil
}

func (h *ProjectHandler) loadToolSandbox(c *fiber.Ctx, userID uint) (*models.Project, *models.Sandbox, bool) {
	project, sandbox, err := h.getOwnedProjectSandbox(userID, c.Params("projectId"))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			_ = c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Project not found"})
			return nil, nil, false
		}
		_ = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to load project sandbox"})
		return nil, nil, false
	}
	return project, sandbox, true
}

func (h *ProjectHandler) validateToolSession(c *fiber.Ctx, userID, projectID, sessionID uint) (uint, bool) {
	if sessionID == 0 {
		return 0, true
	}
	var session models.AgentSession
	if err := database.DB.Where("id = ? AND user_id = ? AND project_id = ?", sessionID, userID, projectID).First(&session).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "agent_session_id does not belong to project"})
			return 0, false
		}
		_ = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to validate agent session"})
		return 0, false
	}
	return sessionID, true
}

func (h *ProjectHandler) validateToolSessionForTool(c *fiber.Ctx, userID, projectID, sessionID uint, toolName string, input any) (uint, bool) {
	if sessionID == 0 {
		return 0, true
	}
	var session models.AgentSession
	if err := database.DB.Where("id = ? AND user_id = ? AND project_id = ?", sessionID, userID, projectID).First(&session).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			h.recordToolCall(userID, projectID, 0, toolName, input, "", 2, fmt.Errorf("agent_session_id does not belong to project"))
			_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "agent_session_id does not belong to project"})
			return 0, false
		}
		h.recordToolCall(userID, projectID, 0, toolName, input, "", 2, fmt.Errorf("failed to validate agent session: %w", err))
		_ = c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to validate agent session"})
		return 0, false
	}
	return sessionID, true
}

func (h *ProjectHandler) toolSessionFromQuery(c *fiber.Ctx, userID, projectID uint) (uint, bool) {
	raw := strings.TrimSpace(c.Query("agent_session_id"))
	if raw == "" {
		return 0, true
	}
	n, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || n == 0 {
		_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "agent_session_id must be a positive integer"})
		return 0, false
	}
	return h.validateToolSession(c, userID, projectID, uint(n))
}

func (h *ProjectHandler) toolSessionFromQueryForTool(c *fiber.Ctx, userID, projectID uint, toolName string, input any) (uint, bool) {
	raw := strings.TrimSpace(c.Query("agent_session_id"))
	if raw == "" {
		return 0, true
	}
	n, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || n == 0 {
		h.recordToolCall(userID, projectID, 0, toolName, input, "", 2, fmt.Errorf("agent_session_id must be a positive integer"))
		_ = c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "agent_session_id must be a positive integer"})
		return 0, false
	}
	return h.validateToolSessionForTool(c, userID, projectID, uint(n), toolName, input)
}

func (h *ProjectHandler) failToolCall(c *fiber.Ctx, userID, projectID, sessionID uint, toolName string, input any, status int, message string) error {
	h.recordToolCall(userID, projectID, sessionID, toolName, input, "", 2, fmt.Errorf("%s", message))
	return c.Status(status).JSON(fiber.Map{"error": message})
}

func invalidBodyAuditInput(c *fiber.Ctx) fiber.Map {
	return fiber.Map{
		"invalid_body": true,
		"body_bytes":   len(c.Body()),
	}
}

func (h *ProjectHandler) recordToolCall(userID, projectID, sessionID uint, toolName string, input any, output string, exitCode int, callErr error) {
	status := "success"
	errMessage := ""
	if callErr != nil || exitCode != 0 {
		status = "failed"
	}
	if callErr != nil {
		errMessage = callErr.Error()
	}
	if len(output) > 16*1024 {
		output = output[:16*1024]
	}
	_ = database.DB.Create(&models.ToolCall{
		UserID:         userID,
		ProjectID:      projectID,
		AgentSessionID: sessionID,
		ToolName:       toolName,
		Status:         status,
		Input:          auditInputJSON(input),
		Output:         output,
		ExitCode:       exitCode,
		Error:          errMessage,
	}).Error
}

func auditInputJSON(input any) string {
	raw, err := json.Marshal(input)
	if err != nil {
		return "{}"
	}
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return string(raw)
	}
	sanitized := sanitizeAuditValue("", value)
	out, err := json.Marshal(sanitized)
	if err != nil {
		return "{}"
	}
	return string(out)
}

func previewDescriptor(projectID, sandboxID uint, port int) fiber.Map {
	return fiber.Map{
		"project_id": projectID,
		"sandbox_id": sandboxID,
		"port":       port,
		"status":     "preview_descriptor",
		"message":    "Preview proxy routing is not exposed by this endpoint; run the service in the sandbox and use the project preview integration.",
	}
}

func sanitizeAuditValue(key string, value any) any {
	lowerKey := strings.ToLower(key)
	switch typed := value.(type) {
	case map[string]any:
		sanitized := make(map[string]any, len(typed))
		for childKey, childValue := range typed {
			sanitized[childKey] = sanitizeAuditValue(childKey, childValue)
		}
		return sanitized
	case []any:
		if lowerKey == "env" {
			return fiber.Map{"count": len(typed), "redacted": true}
		}
		sanitized := make([]any, 0, len(typed))
		for _, item := range typed {
			sanitized = append(sanitized, sanitizeAuditValue("", item))
		}
		return sanitized
	case string:
		switch lowerKey {
		case "content":
			contentBytes := len([]byte(typed))
			return fiber.Map{
				"bytes":    contentBytes,
				"redacted": true,
			}
		case "content_base64":
			return fiber.Map{"base64_bytes": len(typed), "redacted": true}
		}
		return typed
	default:
		return value
	}
}

func auditListLimit(raw string) int {
	limit, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || limit <= 0 {
		return 100
	}
	if limit > 500 {
		return 500
	}
	return limit
}

func parseCommand(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, fmt.Errorf("command is required")
	}
	var argv []string
	if err := json.Unmarshal(raw, &argv); err == nil {
		if len(argv) == 0 {
			return nil, fmt.Errorf("command is required")
		}
		return argv, nil
	}
	var command string
	if err := json.Unmarshal(raw, &command); err == nil {
		command = strings.TrimSpace(command)
		if command == "" {
			return nil, fmt.Errorf("command is required")
		}
		return []string{"sh", "-lc", command}, nil
	}
	return nil, fmt.Errorf("command must be a string or string array")
}

func commandTimeout(requestedSeconds int) time.Duration {
	if requestedSeconds <= 0 {
		requestedSeconds = defaultCommandTimeoutSeconds
	}
	if requestedSeconds > maxCommandTimeoutSeconds {
		requestedSeconds = maxCommandTimeoutSeconds
	}
	return time.Duration(requestedSeconds) * time.Second
}

func outputLimit(requestedBytes int64) int64 {
	if requestedBytes <= 0 {
		return defaultOutputLimitBytes
	}
	if requestedBytes > defaultOutputLimitBytes {
		return defaultOutputLimitBytes
	}
	return requestedBytes
}

func sandboxImage(requestedImage string) (string, error) {
	image := strings.TrimSpace(requestedImage)
	if image == "" || image == defaultSandboxImage {
		return defaultSandboxImage, nil
	}
	return "", fmt.Errorf("project sandboxes must use %s", defaultSandboxImage)
}

func sandboxNetworkInternal(requestedMode string) (bool, error) {
	mode := strings.ToLower(strings.TrimSpace(requestedMode))
	switch mode {
	case "", "project", "bridge":
		return false, nil
	case "no_internet", "no-internet", "internal", "offline":
		return true, nil
	default:
		return false, fmt.Errorf("project sandboxes only support project or no_internet network modes")
	}
}

func sandboxCPULimit(requestedCPU float64) float64 {
	if requestedCPU <= 0 || requestedCPU > defaultCPULimit {
		return defaultCPULimit
	}
	return requestedCPU
}

func sandboxMemoryLimit(requestedMemory int64) int64 {
	if requestedMemory <= 0 || requestedMemory > defaultMemoryLimit {
		return defaultMemoryLimit
	}
	return requestedMemory
}

func publicProjectResponse(project *models.Project, sandbox *models.Sandbox) ProjectResponse {
	return ProjectResponse{
		Project: PublicProject{
			ID:        project.ID,
			UserID:    project.UserID,
			Name:      project.Name,
			RepoURL:   project.RepoURL,
			Branch:    project.Branch,
			Workspace: project.Workspace,
			SandboxID: project.SandboxID,
			CreatedAt: project.CreatedAt,
			UpdatedAt: project.UpdatedAt,
		},
		Sandbox: PublicSandbox{
			ID:            sandbox.ID,
			UserID:        sandbox.UserID,
			ProjectID:     sandbox.ProjectID,
			Name:          sandbox.Name,
			Image:         sandbox.Image,
			Status:        sandbox.Status,
			WorkspacePath: sandbox.WorkspacePath,
			CPULimit:      sandbox.CPULimit,
			MemoryLimit:   sandbox.MemoryLimit,
			CreatedAt:     sandbox.CreatedAt,
			UpdatedAt:     sandbox.UpdatedAt,
		},
	}
}

func cleanWorkspace(workspace string) string {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		return defaultWorkspacePath
	}
	cleaned := path.Clean("/" + strings.TrimPrefix(workspace, "/"))
	if cleaned == "/" {
		return defaultWorkspacePath
	}
	return cleaned
}

func sandboxWorkspace(workspace string) (string, error) {
	raw := strings.TrimSpace(workspace)
	if raw == "" {
		return defaultWorkspacePath, nil
	}
	for _, segment := range strings.Split(raw, "/") {
		if segment == ".." {
			return "", fmt.Errorf("project sandbox workspace must be %s", defaultWorkspacePath)
		}
	}
	cleaned := path.Clean("/" + strings.TrimPrefix(raw, "/"))
	if cleaned != defaultWorkspacePath {
		return "", fmt.Errorf("project sandbox workspace must be %s", defaultWorkspacePath)
	}
	return defaultWorkspacePath, nil
}

func resolveSandboxPath(workspacePath, requestedPath string) (string, error) {
	workspacePath = cleanWorkspace(workspacePath)
	requestedPath = strings.TrimSpace(requestedPath)
	if requestedPath == "" || requestedPath == "." {
		return workspacePath, nil
	}
	var resolved string
	if strings.HasPrefix(requestedPath, "/") {
		resolved = path.Clean(requestedPath)
	} else {
		resolved = path.Clean(path.Join(workspacePath, requestedPath))
	}
	if resolved != workspacePath && !strings.HasPrefix(resolved, workspacePath+"/") {
		return "", fmt.Errorf("path must stay inside %s", workspacePath)
	}
	return resolved, nil
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
