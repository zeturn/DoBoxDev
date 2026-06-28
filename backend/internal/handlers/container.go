package handlers

import (
	"archive/tar"
	"bytes"
	"context"
	"docode/internal/config"
	"docode/internal/database"
	"docode/internal/docker"
	"docode/internal/middleware"
	"docode/internal/models"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
)

type ContainerHandler struct {
	dockerService *docker.DockerService
	config        *config.Config
}

func NewContainerHandler(cfg *config.Config, dockerService *docker.DockerService) *ContainerHandler {
	return &ContainerHandler{
		dockerService: dockerService,
		config:        cfg,
	}
}

type CreateContainerRequest struct {
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	Env           []string          `json:"env"`
	Ports         map[string]string `json:"ports"`
	Volumes       []string          `json:"volumes"`
	Command       []string          `json:"command"`
	WorkingDir    string            `json:"working_dir"`
	RestartPolicy string            `json:"restart_policy"`
	NetworkMode   string            `json:"network_mode"`
	CPULimit      float64           `json:"cpu_limit"`
	MemoryLimit   int64             `json:"memory_limit"`
}

func getOwnedContainer(userID uint, containerDBID string) (*models.Container, error) {
	var container models.Container
	if err := database.DB.Where("id = ? AND user_id = ?", containerDBID, userID).First(&container).Error; err != nil {
		return nil, err
	}
	return &container, nil
}

// ListContainers returns all containers for the authenticated user
func (h *ContainerHandler) ListContainers(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var containers []models.Container
	if err := database.DB.Where("user_id = ?", userID).Find(&containers).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch containers",
		})
	}

	// Update status from Docker
	ctx := context.Background()
	for i := range containers {
		status, err := h.dockerService.GetContainerStatus(ctx, containers[i].ContainerID)
		if err == nil {
			containers[i].Status = status
			database.DB.Model(&containers[i]).Update("status", status)
		}
	}

	return c.JSON(containers)
}

// CreateContainer creates a new container
func (h *ContainerHandler) CreateContainer(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)

	var req CreateContainerRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	// Validate input
	if req.Name == "" || req.Image == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Name and image are required",
		})
	}

	// Create container in Docker
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	containerID, err := h.dockerService.CreateContainer(ctx, docker.CreateContainerOptions{
		Name:          req.Name,
		Image:         req.Image,
		Env:           req.Env,
		Ports:         req.Ports,
		Volumes:       req.Volumes,
		Command:       req.Command,
		WorkingDir:    req.WorkingDir,
		RestartPolicy: req.RestartPolicy,
		NetworkMode:   req.NetworkMode,
		CPULimit:      req.CPULimit,
		MemoryLimit:   req.MemoryLimit,
	})

	if err != nil {
		writeAudit(userID, 0, "create", "failed", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create container: " + err.Error(),
		})
	}

	// Store in database
	portsJSON, _ := json.Marshal(req.Ports)
	envJSON, _ := json.Marshal(req.Env)
	volumesJSON, _ := json.Marshal(req.Volumes)
	commandJSON, _ := json.Marshal(req.Command)

	container := models.Container{
		UserID:        userID,
		ContainerID:   containerID,
		Name:          req.Name,
		Image:         req.Image,
		Status:        "created",
		Ports:         string(portsJSON),
		EnvVars:       string(envJSON),
		Volumes:       string(volumesJSON),
		Command:       string(commandJSON),
		WorkingDir:    req.WorkingDir,
		RestartPolicy: req.RestartPolicy,
		NetworkMode:   req.NetworkMode,
		CPULimit:      req.CPULimit,
		MemoryLimit:   req.MemoryLimit,
	}

	if err := database.DB.Create(&container).Error; err != nil {
		// Rollback: remove container from Docker
		h.dockerService.RemoveContainer(ctx, containerID)
		writeAudit(userID, 0, "create", "failed", "failed to persist container")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to save container",
		})
	}
	writeAudit(userID, container.ID, "create", "success", "container created")

	return c.Status(fiber.StatusCreated).JSON(container)
}

// RestartContainer restarts a container
func (h *ContainerHandler) RestartContainer(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	containerDBID := c.Params("id")

	var container models.Container
	if err := database.DB.Where("id = ? AND user_id = ?", containerDBID, userID).First(&container).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Container not found",
		})
	}

	ctx := context.Background()
	if err := h.dockerService.RestartContainer(ctx, container.ContainerID); err != nil {
		writeAudit(userID, container.ID, "restart", "failed", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to restart container: " + err.Error(),
		})
	}

	container.Status = "running"
	database.DB.Model(&container).Update("status", "running")
	writeAudit(userID, container.ID, "restart", "success", "container restarted")

	return c.JSON(fiber.Map{
		"message":   "Container restarted successfully",
		"container": container,
	})
}

// GetContainer returns a specific container
func (h *ContainerHandler) GetContainer(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	containerDBID := c.Params("id")

	var container models.Container
	if err := database.DB.Where("id = ? AND user_id = ?", containerDBID, userID).First(&container).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Container not found",
		})
	}

	// Update status from Docker
	ctx := context.Background()
	status, err := h.dockerService.GetContainerStatus(ctx, container.ContainerID)
	if err == nil {
		container.Status = status
		database.DB.Model(&container).Update("status", status)
	}

	return c.JSON(container)
}

// StartContainer starts a container
func (h *ContainerHandler) StartContainer(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	containerDBID := c.Params("id")

	var container models.Container
	if err := database.DB.Where("id = ? AND user_id = ?", containerDBID, userID).First(&container).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Container not found",
		})
	}

	ctx := context.Background()
	if err := h.dockerService.StartContainer(ctx, container.ContainerID); err != nil {
		writeAudit(userID, container.ID, "start", "failed", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to start container: " + err.Error(),
		})
	}

	container.Status = "running"
	database.DB.Model(&container).Update("status", "running")
	writeAudit(userID, container.ID, "start", "success", "container started")

	return c.JSON(fiber.Map{
		"message":   "Container started successfully",
		"container": container,
	})
}

// StopContainer stops a container
func (h *ContainerHandler) StopContainer(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	containerDBID := c.Params("id")

	var container models.Container
	if err := database.DB.Where("id = ? AND user_id = ?", containerDBID, userID).First(&container).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Container not found",
		})
	}

	ctx := context.Background()
	if err := h.dockerService.StopContainer(ctx, container.ContainerID); err != nil {
		writeAudit(userID, container.ID, "stop", "failed", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to stop container: " + err.Error(),
		})
	}

	container.Status = "exited"
	database.DB.Model(&container).Update("status", "exited")
	writeAudit(userID, container.ID, "stop", "success", "container stopped")

	return c.JSON(fiber.Map{
		"message":   "Container stopped successfully",
		"container": container,
	})
}

// PauseContainer pauses a container
func (h *ContainerHandler) PauseContainer(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	containerDBID := c.Params("id")

	var container models.Container
	if err := database.DB.Where("id = ? AND user_id = ?", containerDBID, userID).First(&container).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Container not found",
		})
	}

	ctx := context.Background()
	if err := h.dockerService.PauseContainer(ctx, container.ContainerID); err != nil {
		writeAudit(userID, container.ID, "pause", "failed", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to pause container: " + err.Error(),
		})
	}

	container.Status = "paused"
	database.DB.Model(&container).Update("status", "paused")
	writeAudit(userID, container.ID, "pause", "success", "container paused")

	return c.JSON(fiber.Map{
		"message":   "Container paused successfully",
		"container": container,
	})
}

// UnpauseContainer resumes a paused container
func (h *ContainerHandler) UnpauseContainer(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	containerDBID := c.Params("id")

	var container models.Container
	if err := database.DB.Where("id = ? AND user_id = ?", containerDBID, userID).First(&container).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Container not found",
		})
	}

	ctx := context.Background()
	if err := h.dockerService.UnpauseContainer(ctx, container.ContainerID); err != nil {
		writeAudit(userID, container.ID, "unpause", "failed", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to unpause container: " + err.Error(),
		})
	}

	container.Status = "running"
	database.DB.Model(&container).Update("status", "running")
	writeAudit(userID, container.ID, "unpause", "success", "container unpaused")

	return c.JSON(fiber.Map{
		"message":   "Container resumed successfully",
		"container": container,
	})
}

// DeleteContainer removes a container
func (h *ContainerHandler) DeleteContainer(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	containerDBID := c.Params("id")

	var container models.Container
	if err := database.DB.Where("id = ? AND user_id = ?", containerDBID, userID).First(&container).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Container not found",
		})
	}

	ctx := context.Background()
	if err := h.dockerService.RemoveContainer(ctx, container.ContainerID); err != nil {
		writeAudit(userID, container.ID, "delete", "failed", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to remove container: " + err.Error(),
		})
	}

	database.DB.Delete(&container)
	writeAudit(userID, container.ID, "delete", "success", "container deleted")

	return c.JSON(fiber.Map{
		"message": "Container deleted successfully",
	})
}

// UpdateLimits updates container resource limits
func (h *ContainerHandler) UpdateLimits(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	containerDBID := c.Params("id")

	var req struct {
		CPULimit    float64 `json:"cpu_limit"`
		MemoryLimit int64   `json:"memory_limit"`
	}

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	var container models.Container
	if err := database.DB.Where("id = ? AND user_id = ?", containerDBID, userID).First(&container).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Container not found",
		})
	}

	ctx := context.Background()
	if err := h.dockerService.UpdateContainerResources(ctx, container.ContainerID, req.CPULimit, req.MemoryLimit); err != nil {
		writeAudit(userID, container.ID, "limits", "failed", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update limits: " + err.Error(),
		})
	}

	container.CPULimit = req.CPULimit
	container.MemoryLimit = req.MemoryLimit
	database.DB.Save(&container)
	writeAudit(userID, container.ID, "limits", "success", fmt.Sprintf("cpu=%.2f memory=%d", req.CPULimit, req.MemoryLimit))

	return c.JSON(fiber.Map{
		"message":   "Limits updated successfully",
		"container": container,
	})
}

// GetLogs returns container logs
func (h *ContainerHandler) GetLogs(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	containerDBID := c.Params("id")
	tail := c.Query("tail", "100")
	since := c.Query("since", "")
	until := c.Query("until", "")
	follow := c.QueryBool("follow", false)

	var container models.Container
	if err := database.DB.Where("id = ? AND user_id = ?", containerDBID, userID).First(&container).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Container not found",
		})
	}

	ctx := context.Background()
	if follow {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}
	logs, err := h.dockerService.GetContainerLogsWithOptions(ctx, container.ContainerID, docker.LogOptions{
		Tail:   tail,
		Since:  since,
		Until:  until,
		Follow: follow,
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get logs: " + err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"logs": logs,
	})
}

// GetStats returns container statistics
func (h *ContainerHandler) GetStats(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	containerDBID := c.Params("id")

	var container models.Container
	if err := database.DB.Where("id = ? AND user_id = ?", containerDBID, userID).First(&container).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Container not found",
		})
	}

	ctx := context.Background()
	stats, err := h.dockerService.GetContainerStats(ctx, container.ContainerID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get stats: " + err.Error(),
		})
	}

	return c.JSON(stats)
}

func (h *ContainerHandler) ExecInContainer(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	containerDBID := c.Params("id")
	var req struct {
		Command    []string `json:"command"`
		WorkingDir string   `json:"working_dir"`
		Env        []string `json:"env"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	if len(req.Command) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "command is required"})
	}
	var container models.Container
	if err := database.DB.Where("id = ? AND user_id = ?", containerDBID, userID).First(&container).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Container not found"})
	}
	output, exitCode, err := h.dockerService.ExecInContainer(context.Background(), container.ContainerID, req.Command, req.WorkingDir, req.Env)
	if err != nil {
		writeAudit(userID, container.ID, "exec", "failed", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to exec in container: " + err.Error()})
	}
	writeAudit(userID, container.ID, "exec", "success", fmt.Sprintf("cmd=%s exit=%d", strings.Join(req.Command, " "), exitCode))
	return c.JSON(fiber.Map{"output": output, "exit_code": exitCode})
}

func (h *ContainerHandler) GetProcesses(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	containerDBID := c.Params("id")
	var container models.Container
	if err := database.DB.Where("id = ? AND user_id = ?", containerDBID, userID).First(&container).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Container not found"})
	}
	top, err := h.dockerService.GetContainerProcesses(context.Background(), container.ContainerID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get processes: " + err.Error()})
	}
	return c.JSON(fiber.Map{"titles": top.Titles, "processes": top.Processes})
}

func (h *ContainerHandler) GetState(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	containerDBID := c.Params("id")
	var container models.Container
	if err := database.DB.Where("id = ? AND user_id = ?", containerDBID, userID).First(&container).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Container not found"})
	}
	health, exitCode, err := h.dockerService.GetContainerHealthAndExit(context.Background(), container.ContainerID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to get state: " + err.Error()})
	}
	return c.JSON(fiber.Map{"health": health, "exit_code": exitCode})
}

func (h *ContainerHandler) UploadFile(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	containerDBID := c.Params("id")
	var req struct {
		DestinationPath string `json:"destination_path"`
		FileName        string `json:"file_name"`
		ContentBase64   string `json:"content_base64"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	if req.DestinationPath == "" || req.FileName == "" || req.ContentBase64 == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "destination_path, file_name and content_base64 are required"})
	}
	var container models.Container
	if err := database.DB.Where("id = ? AND user_id = ?", containerDBID, userID).First(&container).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Container not found"})
	}
	content, err := base64.StdEncoding.DecodeString(req.ContentBase64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "content_base64 is not valid base64"})
	}
	if err := h.dockerService.UploadFileToContainer(context.Background(), container.ContainerID, req.DestinationPath, req.FileName, content); err != nil {
		writeAudit(userID, container.ID, "upload", "failed", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to upload file: " + err.Error()})
	}
	writeAudit(userID, container.ID, "upload", "success", req.DestinationPath+"/"+req.FileName)
	return c.JSON(fiber.Map{"message": "File uploaded successfully"})
}

func firstFileFromTar(content []byte) (string, []byte, error) {
	name, b, _, err := firstFileFromTarReaderLimited(bytes.NewReader(content), int64(len(content)))
	return name, b, err
}

func firstFileFromTarReaderLimited(reader io.Reader, limitBytes int64) (string, []byte, bool, error) {
	if limitBytes <= 0 {
		limitBytes = 1
	}
	tr := tar.NewReader(reader)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", nil, false, err
		}
		if h.Typeflag == tar.TypeReg {
			b, err := io.ReadAll(io.LimitReader(tr, limitBytes+1))
			if err != nil {
				return "", nil, false, err
			}
			truncated := int64(len(b)) > limitBytes
			if truncated {
				b = b[:limitBytes]
			}
			return h.Name, b, truncated, nil
		}
	}
	return "", nil, false, io.EOF
}

func (h *ContainerHandler) DownloadFile(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	containerDBID := c.Params("id")
	sourcePath := strings.TrimSpace(c.Query("path"))
	if sourcePath == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "path is required"})
	}
	var container models.Container
	if err := database.DB.Where("id = ? AND user_id = ?", containerDBID, userID).First(&container).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Container not found"})
	}
	reader, err := h.dockerService.DownloadFromContainer(context.Background(), container.ContainerID, sourcePath)
	if err != nil {
		writeAudit(userID, container.ID, "download", "failed", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to download file: " + err.Error()})
	}
	defer reader.Close()
	tarBytes, err := io.ReadAll(reader)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to read downloaded file: " + err.Error()})
	}
	name, fileBytes, err := firstFileFromTar(tarBytes)
	if err != nil {
		writeAudit(userID, container.ID, "download", "failed", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to extract file from archive: " + err.Error()})
	}
	writeAudit(userID, container.ID, "download", "success", sourcePath)
	return c.JSON(fiber.Map{
		"file_name":      name,
		"content_base64": base64.StdEncoding.EncodeToString(fileBytes),
	})
}

func (h *ContainerHandler) StreamLogsWS(conn *websocket.Conn) {
	userIDVal := conn.Locals("userID")
	userID, ok := userIDVal.(uint)
	if !ok || userID == 0 {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("unauthorized"))
		_ = conn.Close()
		return
	}
	containerDBID := conn.Params("id")
	container, err := getOwnedContainer(userID, containerDBID)
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("container not found"))
		_ = conn.Close()
		return
	}
	tail := conn.Query("tail", "200")
	since := conn.Query("since", "")
	until := conn.Query("until", "")
	_ = h.dockerService.StreamContainerLogs(context.Background(), container.ContainerID, docker.LogOptions{
		Tail:   tail,
		Since:  since,
		Until:  until,
		Follow: true,
	}, func(chunk []byte) error {
		return conn.WriteMessage(websocket.TextMessage, chunk)
	})
	_ = conn.Close()
}

func (h *ContainerHandler) ShellWS(conn *websocket.Conn) {
	userIDVal := conn.Locals("userID")
	userID, ok := userIDVal.(uint)
	if !ok || userID == 0 {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("unauthorized"))
		_ = conn.Close()
		return
	}
	containerDBID := conn.Params("id")
	container, err := getOwnedContainer(userID, containerDBID)
	if err != nil {
		_ = conn.WriteMessage(websocket.TextMessage, []byte("container not found"))
		_ = conn.Close()
		return
	}
	cmd := []string{"sh"}
	if c := strings.TrimSpace(conn.Query("cmd")); c != "" {
		cmd = strings.Fields(c)
	}
	attachRes, _, err := h.dockerService.OpenContainerShell(context.Background(), container.ContainerID, cmd)
	if err != nil {
		writeAudit(userID, container.ID, "shell_open", "failed", err.Error())
		_ = conn.WriteMessage(websocket.TextMessage, []byte("failed to open shell: "+err.Error()))
		_ = conn.Close()
		return
	}
	writeAudit(userID, container.ID, "shell_open", "success", strings.Join(cmd, " "))
	defer attachRes.Close()

	done := make(chan struct{}, 2)
	go func() {
		defer func() { done <- struct{}{} }()
		buf := make([]byte, 4096)
		for {
			n, rErr := attachRes.Reader.Read(buf)
			if n > 0 {
				if err := conn.WriteMessage(websocket.TextMessage, buf[:n]); err != nil {
					return
				}
			}
			if rErr != nil {
				return
			}
		}
	}()

	go func() {
		defer func() { done <- struct{}{} }()
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				_ = attachRes.CloseWrite()
				return
			}
			if len(msg) == 0 {
				continue
			}
			if _, err := attachRes.Conn.Write(msg); err != nil {
				return
			}
		}
	}()

	<-done
	_ = conn.Close()
}
