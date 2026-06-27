package docker

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/go-connections/nat"
	containerapi "github.com/moby/moby/api/types/container"
	networkapi "github.com/moby/moby/api/types/network"
	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/client"
)

type DockerService struct {
	client *client.Client
}

func NewDockerService() (*DockerService, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker SDK client: %w", err)
	}
	if _, err := cli.Ping(context.Background(), client.PingOptions{}); err != nil {
		_ = cli.Close()
		return nil, fmt.Errorf("docker daemon is not reachable via SDK: %w", err)
	}
	return &DockerService{client: cli}, nil
}

type CreateContainerOptions struct {
	Name          string
	Image         string
	Env           []string
	Ports         map[string]string
	Volumes       []string // hostPath:containerPath[:ro|rw]
	Command       []string
	WorkingDir    string
	RestartPolicy string
	NetworkMode   string
	CPULimit      float64
	MemoryLimit   int64
}

func toNetworkPortMap(ports map[string]string) (networkapi.PortSet, networkapi.PortMap) {
	exposed := networkapi.PortSet{}
	bindings := networkapi.PortMap{}

	for containerPort, hostPort := range ports {
		p, err := nat.NewPort("tcp", containerPort)
		if err != nil {
			continue
		}
		np, err := networkapi.ParsePort(p.Port() + "/" + p.Proto())
		if err != nil {
			continue
		}
		exposed[np] = struct{}{}
		bindings[np] = []networkapi.PortBinding{
			{
				HostIP:   netip.MustParseAddr("0.0.0.0"),
				HostPort: hostPort,
			},
		}
	}

	return exposed, bindings
}

func (s *DockerService) CreateContainer(ctx context.Context, opts CreateContainerOptions) (string, error) {
	if _, err := s.client.ImageInspect(ctx, opts.Image); err != nil {
		pullResp, err := s.client.ImagePull(ctx, opts.Image, client.ImagePullOptions{})
		if err != nil {
			return "", fmt.Errorf("failed to pull image %s: %w", opts.Image, err)
		}
		_, _ = io.Copy(io.Discard, pullResp)
		_ = pullResp.Close()
	}

	exposedPorts, portBindings := toNetworkPortMap(opts.Ports)

	resources := containerapi.Resources{}
	if opts.CPULimit > 0 {
		resources.NanoCPUs = int64(opts.CPULimit * 1e9)
	}
	if opts.MemoryLimit > 0 {
		resources.Memory = opts.MemoryLimit
	}

	restartPolicyName := strings.TrimSpace(opts.RestartPolicy)
	if restartPolicyName == "" {
		restartPolicyName = "unless-stopped"
	}
	networkMode := strings.TrimSpace(opts.NetworkMode)
	if networkMode == "" {
		networkMode = "bridge"
	}

	result, err := s.client.ContainerCreate(ctx, client.ContainerCreateOptions{
		Name: opts.Name,
		Config: &containerapi.Config{
			Image:        opts.Image,
			Env:          opts.Env,
			Cmd:          opts.Command,
			WorkingDir:   opts.WorkingDir,
			ExposedPorts: exposedPorts,
		},
		HostConfig: &containerapi.HostConfig{
			Binds:        opts.Volumes,
			PortBindings: portBindings,
			NetworkMode:  containerapi.NetworkMode(networkMode),
			Resources:    resources,
			RestartPolicy: containerapi.RestartPolicy{
				Name: containerapi.RestartPolicyMode(restartPolicyName),
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}
	return result.ID, nil
}

func (s *DockerService) StartContainer(ctx context.Context, containerID string) error {
	_, err := s.client.ContainerStart(ctx, containerID, client.ContainerStartOptions{})
	return err
}

func (s *DockerService) StopContainer(ctx context.Context, containerID string) error {
	timeout := 10
	_, err := s.client.ContainerStop(ctx, containerID, client.ContainerStopOptions{Timeout: &timeout})
	return err
}

func (s *DockerService) RestartContainer(ctx context.Context, containerID string) error {
	timeout := 10
	if err := s.StopContainer(ctx, containerID); err != nil {
		// continue only if already stopped
		if !strings.Contains(strings.ToLower(err.Error()), "is not running") {
			return err
		}
	}
	_ = timeout
	return s.StartContainer(ctx, containerID)
}

func (s *DockerService) PauseContainer(ctx context.Context, containerID string) error {
	_, err := s.client.ContainerPause(ctx, containerID, client.ContainerPauseOptions{})
	return err
}

func (s *DockerService) UnpauseContainer(ctx context.Context, containerID string) error {
	_, err := s.client.ContainerUnpause(ctx, containerID, client.ContainerUnpauseOptions{})
	return err
}

func (s *DockerService) RemoveContainer(ctx context.Context, containerID string) error {
	_, err := s.client.ContainerRemove(ctx, containerID, client.ContainerRemoveOptions{Force: true, RemoveVolumes: true})
	return err
}

func (s *DockerService) GetContainerStatus(ctx context.Context, containerID string) (string, error) {
	info, err := s.client.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})
	if err != nil {
		return "", err
	}
	return string(info.Container.State.Status), nil
}

func (s *DockerService) GetContainerLogs(ctx context.Context, containerID string, tail string) (string, error) {
	reader, err := s.client.ContainerLogs(ctx, containerID, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       tail,
		Timestamps: true,
	})
	if err != nil {
		return "", err
	}
	defer reader.Close()
	b, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type ContainerStats struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryUsage   uint64  `json:"memory_usage"`
	MemoryLimit   uint64  `json:"memory_limit"`
	MemoryPercent float64 `json:"memory_percent"`
	NetworkRx     uint64  `json:"network_rx"`
	NetworkTx     uint64  `json:"network_tx"`
	DiskRead      uint64  `json:"disk_read"`
	DiskWrite     uint64  `json:"disk_write"`
}

func (s *DockerService) GetContainerStats(ctx context.Context, containerID string) (*ContainerStats, error) {
	statsResult, err := s.client.ContainerStats(ctx, containerID, client.ContainerStatsOptions{
		Stream:                false,
		IncludePreviousSample: true,
	})
	if err != nil {
		return nil, err
	}
	defer statsResult.Body.Close()

	var st containerapi.StatsResponse
	if err := json.NewDecoder(statsResult.Body).Decode(&st); err != nil {
		return nil, err
	}

	cpuDelta := float64(st.CPUStats.CPUUsage.TotalUsage - st.PreCPUStats.CPUUsage.TotalUsage)
	systemDelta := float64(st.CPUStats.SystemUsage - st.PreCPUStats.SystemUsage)
	cpuPercent := 0.0
	if systemDelta > 0 && cpuDelta > 0 {
		cpuPercent = (cpuDelta / systemDelta) * float64(len(st.CPUStats.CPUUsage.PercpuUsage)) * 100.0
	}

	memPercent := 0.0
	if st.MemoryStats.Limit > 0 {
		memPercent = (float64(st.MemoryStats.Usage) / float64(st.MemoryStats.Limit)) * 100.0
	}

	var rx uint64
	var tx uint64
	for _, n := range st.Networks {
		rx += n.RxBytes
		tx += n.TxBytes
	}
	var diskRead uint64
	var diskWrite uint64
	for _, e := range st.BlkioStats.IoServiceBytesRecursive {
		op := strings.ToLower(e.Op)
		if op == "read" {
			diskRead += e.Value
		}
		if op == "write" {
			diskWrite += e.Value
		}
	}

	return &ContainerStats{
		CPUPercent:    cpuPercent,
		MemoryUsage:   st.MemoryStats.Usage,
		MemoryLimit:   st.MemoryStats.Limit,
		MemoryPercent: memPercent,
		NetworkRx:     rx,
		NetworkTx:     tx,
		DiskRead:      diskRead,
		DiskWrite:     diskWrite,
	}, nil
}

func (s *DockerService) UpdateContainerResources(ctx context.Context, containerID string, cpuLimit float64, memoryLimit int64) error {
	resources := &containerapi.Resources{}
	if cpuLimit > 0 {
		resources.NanoCPUs = int64(cpuLimit * 1e9)
	}
	if memoryLimit > 0 {
		resources.Memory = memoryLimit
	}
	_, err := s.client.ContainerUpdate(ctx, containerID, client.ContainerUpdateOptions{
		Resources: resources,
	})
	return err
}

func (s *DockerService) Close() error {
	if s.client == nil {
		return nil
	}
	return s.client.Close()
}

type LogOptions struct {
	Tail   string
	Since  string
	Until  string
	Follow bool
}

func (s *DockerService) GetContainerLogsWithOptions(ctx context.Context, containerID string, opts LogOptions) (string, error) {
	reader, err := s.client.ContainerLogs(ctx, containerID, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       opts.Tail,
		Since:      opts.Since,
		Until:      opts.Until,
		Follow:     opts.Follow,
		Timestamps: true,
	})
	if err != nil {
		return "", err
	}
	defer reader.Close()
	b, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (s *DockerService) StreamContainerLogs(ctx context.Context, containerID string, opts LogOptions, onChunk func([]byte) error) error {
	reader, err := s.client.ContainerLogs(ctx, containerID, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       opts.Tail,
		Since:      opts.Since,
		Until:      opts.Until,
		Follow:     true,
		Timestamps: true,
	})
	if err != nil {
		return err
	}
	defer reader.Close()

	buf := make([]byte, 4096)
	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			if err := onChunk(buf[:n]); err != nil {
				return err
			}
		}
		if readErr != nil {
			if readErr == io.EOF || strings.Contains(strings.ToLower(readErr.Error()), "context canceled") {
				return nil
			}
			return readErr
		}
	}
}

func (s *DockerService) ExecInContainer(ctx context.Context, containerID string, cmd []string, workDir string, env []string) (string, int, error) {
	if len(cmd) == 0 {
		return "", 0, fmt.Errorf("exec command is required")
	}
	createRes, err := s.client.ExecCreate(ctx, containerID, client.ExecCreateOptions{
		AttachStdout: true,
		AttachStderr: true,
		TTY:          true,
		Cmd:          cmd,
		WorkingDir:   workDir,
		Env:          env,
	})
	if err != nil {
		return "", 0, err
	}
	startRes, err := s.client.ExecAttach(ctx, createRes.ID, client.ExecAttachOptions{
		TTY: true,
	})
	if err != nil {
		return "", 0, err
	}
	defer startRes.Close()
	out, err := io.ReadAll(startRes.Reader)
	if err != nil {
		return "", 0, err
	}
	inspect, err := s.client.ExecInspect(ctx, createRes.ID, client.ExecInspectOptions{})
	if err != nil {
		return "", 0, err
	}
	return string(out), inspect.ExitCode, nil
}

func (s *DockerService) OpenContainerShell(ctx context.Context, containerID string, cmd []string) (client.ExecAttachResult, string, error) {
	if len(cmd) == 0 {
		cmd = []string{"sh"}
	}
	createRes, err := s.client.ExecCreate(ctx, containerID, client.ExecCreateOptions{
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		TTY:          true,
		Cmd:          cmd,
	})
	if err != nil {
		return client.ExecAttachResult{}, "", err
	}
	attachRes, err := s.client.ExecAttach(ctx, createRes.ID, client.ExecAttachOptions{
		TTY: true,
	})
	if err != nil {
		return client.ExecAttachResult{}, "", err
	}
	return attachRes, createRes.ID, nil
}

func (s *DockerService) GetContainerProcesses(ctx context.Context, containerID string) (client.ContainerTopResult, error) {
	return s.client.ContainerTop(ctx, containerID, client.ContainerTopOptions{})
}

func (s *DockerService) GetContainerHealthAndExit(ctx context.Context, containerID string) (string, int, error) {
	info, err := s.client.ContainerInspect(ctx, containerID, client.ContainerInspectOptions{})
	if err != nil {
		return "", 0, err
	}
	health := "unknown"
	if info.Container.State != nil && info.Container.State.Health != nil {
		health = string(info.Container.State.Health.Status)
	}
	exitCode := 0
	if info.Container.State != nil {
		exitCode = info.Container.State.ExitCode
	}
	return health, exitCode, nil
}

func tarSingleFile(name string, data []byte, mode fs.FileMode) (io.Reader, error) {
	buf := &bytes.Buffer{}
	tw := tar.NewWriter(buf)
	h := &tar.Header{
		Name:    name,
		Mode:    int64(mode.Perm()),
		Size:    int64(len(data)),
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(h); err != nil {
		return nil, err
	}
	if _, err := tw.Write(data); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf, nil
}

func (s *DockerService) UploadFileToContainer(ctx context.Context, containerID, destinationPath, fileName string, data []byte) error {
	tarReader, err := tarSingleFile(fileName, data, 0o644)
	if err != nil {
		return err
	}
	_, err = s.client.CopyToContainer(ctx, containerID, client.CopyToContainerOptions{
		DestinationPath: destinationPath,
		Content:         tarReader,
	})
	return err
}

func (s *DockerService) DownloadFromContainer(ctx context.Context, containerID, sourcePath string) (io.ReadCloser, error) {
	res, err := s.client.CopyFromContainer(ctx, containerID, client.CopyFromContainerOptions{SourcePath: sourcePath})
	if err != nil {
		return nil, err
	}
	return res.Content, nil
}

type ImageSummary struct {
	ID       string   `json:"id"`
	RepoTags []string `json:"repo_tags"`
	Created  int64    `json:"created"`
	Size     int64    `json:"size"`
}

func (s *DockerService) PullImage(ctx context.Context, imageRef string) (string, error) {
	reader, err := s.client.ImagePull(ctx, imageRef, client.ImagePullOptions{})
	if err != nil {
		return "", err
	}
	defer reader.Close()
	b, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (s *DockerService) ListImages(ctx context.Context) ([]ImageSummary, error) {
	result, err := s.client.ImageList(ctx, client.ImageListOptions{All: true})
	if err != nil {
		return nil, err
	}
	images := make([]ImageSummary, 0, len(result.Items))
	for _, item := range result.Items {
		images = append(images, ImageSummary{
			ID:       item.ID,
			RepoTags: item.RepoTags,
			Created:  item.Created,
			Size:     item.Size,
		})
	}
	return images, nil
}

func (s *DockerService) RemoveImage(ctx context.Context, imageRef string, force bool) ([]string, error) {
	result, err := s.client.ImageRemove(ctx, imageRef, client.ImageRemoveOptions{Force: force, PruneChildren: true})
	if err != nil {
		return nil, err
	}
	items := make([]string, 0, len(result.Items))
	for _, it := range result.Items {
		if it.Deleted != "" {
			items = append(items, "deleted:"+it.Deleted)
		}
		if it.Untagged != "" {
			items = append(items, "untagged:"+it.Untagged)
		}
	}
	return items, nil
}

func (s *DockerService) InspectImage(ctx context.Context, imageRef string) (client.ImageInspectResult, error) {
	return s.client.ImageInspect(ctx, imageRef)
}

func (s *DockerService) TagImage(ctx context.Context, sourceRef string, targetRef string) error {
	_, err := s.client.ImageTag(ctx, client.ImageTagOptions{Source: sourceRef, Target: targetRef})
	return err
}

func (s *DockerService) PushImage(ctx context.Context, imageRef, username, password, serverAddress string) (string, error) {
	auth := registry.AuthConfig{
		Username:      username,
		Password:      password,
		ServerAddress: serverAddress,
	}
	authJSON, err := json.Marshal(auth)
	if err != nil {
		return "", err
	}
	reader, err := s.client.ImagePush(ctx, imageRef, client.ImagePushOptions{
		RegistryAuth: base64.StdEncoding.EncodeToString(authJSON),
	})
	if err != nil {
		return "", err
	}
	defer reader.Close()
	b, err := io.ReadAll(reader)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type BuildImageOptions struct {
	ContextPath string            `json:"context_path"`
	Dockerfile  string            `json:"dockerfile"`
	Tag         string            `json:"tag"`
	BuildArgs   map[string]string `json:"build_args"`
	NoCache     bool              `json:"no_cache"`
}

func createBuildContextTar(contextPath string) (io.Reader, error) {
	buf := &bytes.Buffer{}
	tw := tar.NewWriter(buf)

	err := filepath.Walk(contextPath, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(contextPath, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = relPath
		header.ModTime = time.Now()

		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		if _, err := io.Copy(tw, file); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if err := tw.Close(); err != nil {
		return nil, err
	}
	return buf, nil
}

func (s *DockerService) BuildImage(ctx context.Context, opts BuildImageOptions) (string, error) {
	contextPath := strings.TrimSpace(opts.ContextPath)
	if contextPath == "" {
		contextPath = "."
	}
	dockerfile := strings.TrimSpace(opts.Dockerfile)
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}
	if strings.TrimSpace(opts.Tag) == "" {
		return "", fmt.Errorf("tag is required")
	}

	buildContext, err := createBuildContextTar(contextPath)
	if err != nil {
		return "", err
	}

	buildArgs := map[string]*string{}
	for k, v := range opts.BuildArgs {
		val := v
		buildArgs[k] = &val
	}

	res, err := s.client.ImageBuild(ctx, buildContext, client.ImageBuildOptions{
		Tags:       []string{opts.Tag},
		Dockerfile: dockerfile,
		BuildArgs:  buildArgs,
		NoCache:    opts.NoCache,
		Remove:     true,
		PullParent: true,
	})
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	b, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type NetworkSummary struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Driver     string `json:"driver"`
	Scope      string `json:"scope"`
	Containers int    `json:"containers"`
}

func (s *DockerService) CreateNetwork(ctx context.Context, name, driver string, attachable, internal bool) (string, error) {
	if strings.TrimSpace(driver) == "" {
		driver = "bridge"
	}
	result, err := s.client.NetworkCreate(ctx, name, client.NetworkCreateOptions{
		Driver:     driver,
		Attachable: attachable,
		Internal:   internal,
	})
	if err != nil {
		return "", err
	}
	return result.ID, nil
}

func (s *DockerService) RemoveNetwork(ctx context.Context, networkID string) error {
	_, err := s.client.NetworkRemove(ctx, networkID, client.NetworkRemoveOptions{})
	return err
}

func (s *DockerService) ListNetworks(ctx context.Context) ([]NetworkSummary, error) {
	result, err := s.client.NetworkList(ctx, client.NetworkListOptions{})
	if err != nil {
		return nil, err
	}
	items := make([]NetworkSummary, 0, len(result.Items))
	for _, n := range result.Items {
		containerCount := 0
		inspectRes, inspectErr := s.client.NetworkInspect(ctx, n.ID, client.NetworkInspectOptions{})
		if inspectErr == nil {
			containerCount = len(inspectRes.Network.Containers)
		}
		items = append(items, NetworkSummary{
			ID:         n.ID,
			Name:       n.Name,
			Driver:     n.Driver,
			Scope:      n.Scope,
			Containers: containerCount,
		})
	}
	return items, nil
}

func (s *DockerService) InspectNetwork(ctx context.Context, networkID string) (client.NetworkInspectResult, error) {
	return s.client.NetworkInspect(ctx, networkID, client.NetworkInspectOptions{})
}

func (s *DockerService) ConnectContainerToNetwork(ctx context.Context, networkID, containerID string) error {
	_, err := s.client.NetworkConnect(ctx, networkID, client.NetworkConnectOptions{
		Container: containerID,
	})
	return err
}

func (s *DockerService) DisconnectContainerFromNetwork(ctx context.Context, networkID, containerID string, force bool) error {
	_, err := s.client.NetworkDisconnect(ctx, networkID, client.NetworkDisconnectOptions{
		Container: containerID,
		Force:     force,
	})
	return err
}

type VolumeSummary struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver"`
	Scope      string            `json:"scope"`
	Mountpoint string            `json:"mountpoint"`
	CreatedAt  string            `json:"created_at"`
	Labels     map[string]string `json:"labels"`
}

type VolumeMountRelation struct {
	ContainerID   string `json:"container_id"`
	ContainerName string `json:"container_name"`
	Destination   string `json:"destination"`
	ReadWrite     bool   `json:"read_write"`
}

func (s *DockerService) CreateVolume(ctx context.Context, name, driver string, labels map[string]string) (VolumeSummary, error) {
	if strings.TrimSpace(driver) == "" {
		driver = "local"
	}
	result, err := s.client.VolumeCreate(ctx, client.VolumeCreateOptions{
		Name:   name,
		Driver: driver,
		Labels: labels,
	})
	if err != nil {
		return VolumeSummary{}, err
	}
	v := result.Volume
	return VolumeSummary{
		Name:       v.Name,
		Driver:     v.Driver,
		Scope:      v.Scope,
		Mountpoint: v.Mountpoint,
		CreatedAt:  v.CreatedAt,
		Labels:     v.Labels,
	}, nil
}

func (s *DockerService) RemoveVolume(ctx context.Context, volumeName string, force bool) error {
	_, err := s.client.VolumeRemove(ctx, volumeName, client.VolumeRemoveOptions{Force: force})
	return err
}

func (s *DockerService) ListVolumes(ctx context.Context) ([]VolumeSummary, error) {
	result, err := s.client.VolumeList(ctx, client.VolumeListOptions{})
	if err != nil {
		return nil, err
	}
	items := make([]VolumeSummary, 0, len(result.Items))
	for _, v := range result.Items {
		items = append(items, VolumeSummary{
			Name:       v.Name,
			Driver:     v.Driver,
			Scope:      v.Scope,
			Mountpoint: v.Mountpoint,
			CreatedAt:  v.CreatedAt,
			Labels:     v.Labels,
		})
	}
	return items, nil
}

func (s *DockerService) InspectVolume(ctx context.Context, volumeName string) (client.VolumeInspectResult, error) {
	return s.client.VolumeInspect(ctx, volumeName, client.VolumeInspectOptions{})
}

func (s *DockerService) GetVolumeMountRelations(ctx context.Context, volumeName string) ([]VolumeMountRelation, error) {
	containers, err := s.client.ContainerList(ctx, client.ContainerListOptions{All: true})
	if err != nil {
		return nil, err
	}
	relations := make([]VolumeMountRelation, 0)
	for _, c := range containers.Items {
		containerName := ""
		if len(c.Names) > 0 {
			containerName = strings.TrimPrefix(c.Names[0], "/")
		}
		for _, m := range c.Mounts {
			if m.Type == "volume" && m.Name == volumeName {
				relations = append(relations, VolumeMountRelation{
					ContainerID:   c.ID,
					ContainerName: containerName,
					Destination:   m.Destination,
					ReadWrite:     m.RW,
				})
			}
		}
	}
	return relations, nil
}
