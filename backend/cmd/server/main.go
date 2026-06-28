package main

import (
	"docode/internal/config"
	"docode/internal/database"
	"docode/internal/docker"
	"docode/internal/handlers"
	"docode/internal/middleware"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
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

	app := fiber.New(fiber.Config{
		AppName: "DoBoxDev API",
	})

	app.Use(recover.New())
	app.Use(logger.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.CORSOrigins,
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Origin,Content-Type,Accept,Authorization",
		AllowCredentials: true,
	}))

	authHandler := handlers.NewAuthHandler(cfg)
	containerHandler := handlers.NewContainerHandler(cfg, dockerService)
	projectHandler := handlers.NewProjectHandler(dockerService)
	networkHandler := handlers.NewNetworkHandler(dockerService)
	volumeHandler := handlers.NewVolumeHandler(dockerService)
	imageHandler := handlers.NewImageHandler(dockerService)
	authRequired := middleware.AuthMiddleware(cfg)

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok"})
	})

	api := app.Group("/api")

	auth := api.Group("/auth")
	auth.Post("/register", authHandler.Register)
	auth.Post("/login", authHandler.Login)
	auth.Get("/me", authRequired, authHandler.Me)

	projects := api.Group("/projects", authRequired)
	projects.Get("/", projectHandler.ListProjects)
	projects.Post("/", projectHandler.CreateProject)
	projects.Get("/:projectId", projectHandler.GetProject)
	projects.Delete("/:projectId", projectHandler.DeleteProject)
	projects.Get("/:projectId/agent/sessions", projectHandler.ListAgentSessions)
	projects.Post("/:projectId/agent/sessions", projectHandler.CreateAgentSession)
	projects.Get("/:projectId/agent/tool-calls", projectHandler.ListToolCalls)
	projects.Post("/:projectId/agent/exec", projectHandler.RunCommand)
	projects.Post("/:projectId/exec", projectHandler.RunCommand)
	projects.Post("/:projectId/files/read", projectHandler.ReadFile)
	projects.Post("/:projectId/files/write", projectHandler.WriteFile)
	projects.Post("/:projectId/files/list", projectHandler.ListFiles)
	projects.Post("/:projectId/search", projectHandler.Search)
	projects.Post("/:projectId/files/search", projectHandler.Search)
	projects.Get("/:projectId/git/diff", projectHandler.GitDiff)
	projects.Get("/:projectId/git/status", projectHandler.GitStatus)
	projects.Post("/:projectId/git/status", projectHandler.GitStatus)
	projects.Post("/:projectId/git/commit", projectHandler.GitCommit)
	projects.Get("/:projectId/artifacts/archive", projectHandler.ArchiveWorkspace)
	projects.Get("/:projectId/logs", projectHandler.GetLogs)
	projects.Post("/:projectId/preview", projectHandler.Preview)

	containers := api.Group("/containers", authRequired)
	containers.Get("/", containerHandler.ListContainers)
	containers.Post("/", containerHandler.CreateContainer)
	containers.Get("/audits/all", containerHandler.ListUserAudits)
	containers.Get("/:id", containerHandler.GetContainer)
	containers.Post("/:id/start", containerHandler.StartContainer)
	containers.Post("/:id/stop", containerHandler.StopContainer)
	containers.Post("/:id/restart", containerHandler.RestartContainer)
	containers.Post("/:id/pause", containerHandler.PauseContainer)
	containers.Post("/:id/unpause", containerHandler.UnpauseContainer)
	containers.Delete("/:id", containerHandler.DeleteContainer)
	containers.Put("/:id/limits", containerHandler.UpdateLimits)
	containers.Get("/:id/logs", containerHandler.GetLogs)
	containers.Get("/:id/stats", containerHandler.GetStats)
	containers.Post("/:id/exec", containerHandler.ExecInContainer)
	containers.Get("/:id/processes", containerHandler.GetProcesses)
	containers.Get("/:id/state", containerHandler.GetState)
	containers.Post("/:id/files/upload", containerHandler.UploadFile)
	containers.Get("/:id/files/download", containerHandler.DownloadFile)
	containers.Get("/:id/audits", containerHandler.ListAudits)

	networks := api.Group("/networks", authRequired)
	networks.Get("/", networkHandler.ListNetworks)
	networks.Post("/", networkHandler.CreateNetwork)
	networks.Get("/:id", networkHandler.InspectNetwork)
	networks.Delete("/:id", networkHandler.DeleteNetwork)
	networks.Post("/:id/connect", networkHandler.ConnectContainer)
	networks.Post("/:id/disconnect", networkHandler.DisconnectContainer)

	volumes := api.Group("/volumes", authRequired)
	volumes.Get("/", volumeHandler.ListVolumes)
	volumes.Post("/", volumeHandler.CreateVolume)
	volumes.Get("/:name", volumeHandler.InspectVolume)
	volumes.Delete("/:name", volumeHandler.DeleteVolume)
	volumes.Get("/:name/relations", volumeHandler.MountRelations)

	images := api.Group("/images", authRequired)
	images.Get("/", imageHandler.ListImages)
	images.Post("/pull", imageHandler.PullImage)
	images.Post("/tag", imageHandler.TagImage)
	images.Post("/push", imageHandler.PushImage)
	images.Post("/build", imageHandler.BuildImage)
	images.Get("/:ref", imageHandler.InspectImage)
	images.Delete("/:ref", imageHandler.DeleteImage)

	ws := api.Group("/containers", authRequired)
	ws.Use("/:id/logs/ws", websocket.New(containerHandler.StreamLogsWS))
	ws.Use("/:id/shell/ws", websocket.New(containerHandler.ShellWS))

	go func() {
		addr := ":" + cfg.Port
		log.Printf("DoBoxDev API listening on http://localhost%s", addr)
		if err := app.Listen(addr); err != nil {
			log.Fatalf("server stopped: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("shutting down server")
	if err := app.Shutdown(); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
}
