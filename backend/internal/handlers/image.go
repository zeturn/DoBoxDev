package handlers

import (
	"context"
	"docode/internal/docker"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

type ImageHandler struct {
	dockerService *docker.DockerService
}

func NewImageHandler(dockerService *docker.DockerService) *ImageHandler {
	return &ImageHandler{dockerService: dockerService}
}

func (h *ImageHandler) PullImage(c *fiber.Ctx) error {
	var req struct {
		Image string `json:"image"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	if strings.TrimSpace(req.Image) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "image is required"})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	logs, err := h.dockerService.PullImage(ctx, req.Image)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "Image pulled", "logs": logs})
}

func (h *ImageHandler) ListImages(c *fiber.Ctx) error {
	ctx := context.Background()
	images, err := h.dockerService.ListImages(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(images)
}

func (h *ImageHandler) DeleteImage(c *fiber.Ctx) error {
	imageRef, err := decodeImageRef(c.Params("ref"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid image ref"})
	}
	if strings.TrimSpace(imageRef) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "image ref is required"})
	}
	force := c.Query("force", "false") == "true"
	ctx := context.Background()
	actions, err := h.dockerService.RemoveImage(ctx, imageRef, force)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "Image removed", "actions": actions})
}

func (h *ImageHandler) InspectImage(c *fiber.Ctx) error {
	imageRef, err := decodeImageRef(c.Params("ref"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid image ref"})
	}
	if strings.TrimSpace(imageRef) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "image ref is required"})
	}
	ctx := context.Background()
	info, err := h.dockerService.InspectImage(ctx, imageRef)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(info)
}

func decodeImageRef(raw string) (string, error) {
	decoded, err := url.PathUnescape(raw)
	if err != nil {
		return "", err
	}
	return decoded, nil
}

func (h *ImageHandler) TagImage(c *fiber.Ctx) error {
	var req struct {
		Source string `json:"source"`
		Target string `json:"target"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	if strings.TrimSpace(req.Source) == "" || strings.TrimSpace(req.Target) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "source and target are required"})
	}
	ctx := context.Background()
	if err := h.dockerService.TagImage(ctx, req.Source, req.Target); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "Image tagged"})
}

func (h *ImageHandler) PushImage(c *fiber.Ctx) error {
	var req struct {
		Image         string `json:"image"`
		Username      string `json:"username"`
		Password      string `json:"password"`
		ServerAddress string `json:"server_address"`
	}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	if strings.TrimSpace(req.Image) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "image is required"})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	logs, err := h.dockerService.PushImage(ctx, req.Image, req.Username, req.Password, req.ServerAddress)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "Image pushed", "logs": logs})
}

func (h *ImageHandler) BuildImage(c *fiber.Ctx) error {
	var req docker.BuildImageOptions
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	logs, err := h.dockerService.BuildImage(ctx, req)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"message": "Image built", "logs": logs})
}
