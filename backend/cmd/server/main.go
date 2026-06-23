package main

import (
	"log"
	"path/filepath"
	"strings"

	"docode/internal/config"
	"docode/internal/database"
	"docode/internal/docker"
	"docode/internal/handlers"
	"docode/internal/middleware"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	fibercors "github.com/gofiber/fiber/v2/middleware/cors"
	fiberlogger "github.com/gofiber/fiber/v2/middleware/logger"
	fiberrecover "github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	cfg := config.Load()
	if err := database.Connect(cfg.DBPath); err != nil {
		log.Fatalf("database initialization failed: %v", err)
	}

	dockerService, err := docker.NewDockerService()
	if err != nil {
		log.Fatalf("docker initialization failed: %v", err)
	}
	defer dockerService.Close()

	app := fiber.New(fiber.Config{
		AppName:      "DoBoxDev",
		ServerHeader: "DoBoxDev",
	})
	app.Use(fiberrecover.New())
	app.Use(fiberlogger.New())
	app.Use(fibercors.New(fibercors.Config{
		AllowOrigins: cfg.CORSOrigins,
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		AllowMethods: "GET,POST,PUT,PATCH,DELETE,OPTIONS",
	}))

	registerRoutes(app, cfg, dockerService)

	log.Printf("DoBoxDev backend listening on :%s", cfg.Port)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("server stopped: %v", err)
	}
}

func registerRoutes(app *fiber.App, cfg *config.Config, dockerService *docker.DockerService) {
	authHandler := handlers.NewAuthHandler(cfg)
	containerHandler := handlers.NewContainerHandler(cfg, dockerService)
	imageHandler := handlers.NewImageHandler(dockerService)
	networkHandler := handlers.NewNetworkHandler(dockerService)
	volumeHandler := handlers.NewVolumeHandler(dockerService)
	projectHandler := handlers.NewProjectHandler(dockerService, dataDir(cfg.DBPath))

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	api := app.Group("/api")
	auth := api.Group("/auth")
	auth.Post("/register", authHandler.Register)
	auth.Post("/login", authHandler.Login)
	auth.Get("/me", middleware.AuthMiddleware(cfg), authHandler.Me)

	// Project APIs are intentionally project-scoped and never expose raw Docker
	// container IDs to callers. DoCode talks to these routes from a trusted
	// service boundary with its own bearer token configuration.
	projects := api.Group("/projects")
	projects.Post("/", projectHandler.CreateProject)
	projects.Post("", projectHandler.CreateProject)
	projects.Get("/:id", projectHandler.GetProject)
	projects.Delete("/:id", projectHandler.DeleteProject)
	projects.Post("/:id/agent/sessions", projectHandler.CreateAgentSession)
	projects.Post("/:id/exec", projectHandler.Exec)
	projects.Post("/:id/files/read", projectHandler.ReadFile)
	projects.Post("/:id/files/write", projectHandler.WriteFile)
	projects.Post("/:id/files/list", projectHandler.ListFiles)
	projects.Post("/:id/files/search", projectHandler.SearchFiles)
	projects.Get("/:id/git/status", projectHandler.GitStatus)
	projects.Get("/:id/git/diff", projectHandler.GitDiff)
	projects.Post("/:id/git/commit", projectHandler.GitCommit)
	projects.Post("/:id/preview", projectHandler.Preview)
	projects.Get("/:id/logs", projectHandler.Logs)
	projects.Get("/:id/artifacts/archive", projectHandler.Archive)

	protected := api.Group("", middleware.AuthMiddleware(cfg))
	containers := protected.Group("/containers")
	containers.Get("/", containerHandler.ListContainers)
	containers.Post("/", containerHandler.CreateContainer)
	containers.Get("/:id", containerHandler.GetContainer)
	containers.Post("/:id/start", containerHandler.StartContainer)
	containers.Post("/:id/stop", containerHandler.StopContainer)
	containers.Post("/:id/restart", containerHandler.RestartContainer)
	containers.Post("/:id/pause", containerHandler.PauseContainer)
	containers.Post("/:id/unpause", containerHandler.UnpauseContainer)
	containers.Put("/:id/limits", containerHandler.UpdateLimits)
	containers.Delete("/:id", containerHandler.DeleteContainer)
	containers.Get("/:id/logs", containerHandler.GetLogs)
	containers.Get("/:id/stats", containerHandler.GetStats)
	containers.Post("/:id/exec", containerHandler.ExecInContainer)
	containers.Get("/:id/processes", containerHandler.GetProcesses)
	containers.Get("/:id/state", containerHandler.GetState)
	containers.Post("/:id/files/upload", containerHandler.UploadFile)
	containers.Get("/:id/files/download", containerHandler.DownloadFile)
	containers.Get("/:id/audits", containerHandler.ListAudits)

	protected.Get("/audits", containerHandler.ListUserAudits)

	images := protected.Group("/images")
	images.Post("/pull", imageHandler.PullImage)
	images.Get("/", imageHandler.ListImages)
	images.Delete("/:ref", imageHandler.DeleteImage)
	images.Get("/:ref", imageHandler.InspectImage)
	images.Post("/tag", imageHandler.TagImage)
	images.Post("/push", imageHandler.PushImage)
	images.Post("/build", imageHandler.BuildImage)

	networks := protected.Group("/networks")
	networks.Post("/", networkHandler.CreateNetwork)
	networks.Get("/", networkHandler.ListNetworks)
	networks.Get("/:id", networkHandler.InspectNetwork)
	networks.Delete("/:id", networkHandler.DeleteNetwork)
	networks.Post("/:id/connect", networkHandler.ConnectContainer)
	networks.Post("/:id/disconnect", networkHandler.DisconnectContainer)

	volumes := protected.Group("/volumes")
	volumes.Post("/", volumeHandler.CreateVolume)
	volumes.Get("/", volumeHandler.ListVolumes)
	volumes.Get("/:name", volumeHandler.InspectVolume)
	volumes.Delete("/:name", volumeHandler.DeleteVolume)
	volumes.Get("/:name/mounts", volumeHandler.MountRelations)

	app.Get("/ws/containers/:id/logs", websocket.New(containerHandler.StreamLogsWS))
	app.Get("/ws/containers/:id/shell", websocket.New(containerHandler.ShellWS))
}

func dataDir(dbPath string) string {
	dir := filepath.Dir(dbPath)
	if dir == "." || strings.TrimSpace(dir) == "" {
		return "./data"
	}
	return dir
}
