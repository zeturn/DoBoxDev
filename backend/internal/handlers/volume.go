package handlers

import (
	"context"
	"docode/internal/docker"
	"docode/internal/middleware"
	"strings"

	"github.com/gofiber/fiber/v2"
)

type VolumeHandler struct {
	dockerService *docker.DockerService
}

func NewVolumeHandler(dockerService *docker.DockerService) *VolumeHandler {
	return &VolumeHandler{dockerService: dockerService}
}

func (h *VolumeHandler) CreateVolume(c *fiber.Ctx) error {
	var req struct {
		Name   string            `json:"name"`
		Driver string            `json:"driver"`
		Labels map[string]string `json:"labels"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	if strings.TrimSpace(req.Name) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name is required"})
	}
	userID := middleware.GetUserID(c)
	v, err := h.dockerService.CreateVolume(context.Background(), req.Name, req.Driver, req.Labels)
	if err != nil {
		writeAudit(userID, 0, "volume_create", "failed", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	writeAudit(userID, 0, "volume_create", "success", req.Name)
	return c.JSON(v)
}

func (h *VolumeHandler) DeleteVolume(c *fiber.Ctx) error {
	name := c.Params("name")
	if strings.TrimSpace(name) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name is required"})
	}
	userID := middleware.GetUserID(c)
	force := c.Query("force", "false") == "true"
	if err := h.dockerService.RemoveVolume(context.Background(), name, force); err != nil {
		writeAudit(userID, 0, "volume_delete", "failed", err.Error())
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	writeAudit(userID, 0, "volume_delete", "success", name)
	return c.JSON(fiber.Map{"message": "Volume removed"})
}

func (h *VolumeHandler) ListVolumes(c *fiber.Ctx) error {
	items, err := h.dockerService.ListVolumes(context.Background())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(items)
}

func (h *VolumeHandler) InspectVolume(c *fiber.Ctx) error {
	name := c.Params("name")
	if strings.TrimSpace(name) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name is required"})
	}
	info, err := h.dockerService.InspectVolume(context.Background(), name)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(info.Volume)
}

func (h *VolumeHandler) MountRelations(c *fiber.Ctx) error {
	name := c.Params("name")
	if strings.TrimSpace(name) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name is required"})
	}
	items, err := h.dockerService.GetVolumeMountRelations(context.Background(), name)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(items)
}
