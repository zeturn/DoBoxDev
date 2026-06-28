package docker

import (
	"archive/tar"
	"io"
	"strings"
	"testing"

	containerapi "github.com/moby/moby/api/types/container"
)

func TestReadLimitedMarksTruncatedOutput(t *testing.T) {
	output, truncated, err := readLimited(strings.NewReader("abcdef"), 3)
	if err != nil {
		t.Fatalf("readLimited returned error: %v", err)
	}
	if !truncated {
		t.Fatal("expected truncated output")
	}
	if string(output) != "abc" {
		t.Fatalf("expected clipped output abc, got %q", string(output))
	}
}

func TestReadLimitedLeavesShortOutputUnmarked(t *testing.T) {
	output, truncated, err := readLimited(strings.NewReader("abc"), 3)
	if err != nil {
		t.Fatalf("readLimited returned error: %v", err)
	}
	if truncated {
		t.Fatal("did not expect truncated output")
	}
	if string(output) != "abc" {
		t.Fatalf("expected output abc, got %q", string(output))
	}
}

func TestSandboxContainerOptionsUseNonRootSecurityDefaults(t *testing.T) {
	pidsLimit := int64(128)
	opts, err := sandboxContainerCreateOptions(CreateSandboxOptions{
		Name:          "dobox-p1-sandbox",
		Image:         "dobox/code-sandbox:latest",
		VolumeName:    "dobox_project_1",
		NetworkName:   "dobox_project_1",
		WorkspacePath: "/workspace",
		CPULimit:      1.5,
		MemoryLimit:   512 * 1024 * 1024,
		PidsLimit:     pidsLimit,
	})
	if err != nil {
		t.Fatalf("sandboxContainerCreateOptions returned error: %v", err)
	}

	if opts.Config.User != DefaultSandboxUser {
		t.Fatalf("expected sandbox user %q, got %q", DefaultSandboxUser, opts.Config.User)
	}
	if opts.Config.WorkingDir != "/workspace" {
		t.Fatalf("expected working dir /workspace, got %q", opts.Config.WorkingDir)
	}
	if !containsString(opts.Config.Env, "HOME=/home/docode") {
		t.Fatalf("expected HOME to point at the non-root sandbox home, got %#v", opts.Config.Env)
	}
	if len(opts.HostConfig.Binds) != 1 || opts.HostConfig.Binds[0] != "dobox_project_1:/workspace" {
		t.Fatalf("expected named project volume bind, got %#v", opts.HostConfig.Binds)
	}
	if opts.HostConfig.NetworkMode != containerapi.NetworkMode("dobox_project_1") {
		t.Fatalf("expected project network mode, got %q", opts.HostConfig.NetworkMode)
	}
	if opts.HostConfig.AutoRemove {
		t.Fatal("project sandboxes should not auto-remove before artifacts are collected")
	}
	if !containsString(opts.HostConfig.CapDrop, "ALL") {
		t.Fatalf("expected all Linux capabilities to be dropped, got %#v", opts.HostConfig.CapDrop)
	}
	if !containsString(opts.HostConfig.SecurityOpt, "no-new-privileges:true") {
		t.Fatalf("expected no-new-privileges security option, got %#v", opts.HostConfig.SecurityOpt)
	}
	if opts.HostConfig.Resources.PidsLimit == nil || *opts.HostConfig.Resources.PidsLimit != pidsLimit {
		t.Fatalf("expected pids limit %d, got %#v", pidsLimit, opts.HostConfig.Resources.PidsLimit)
	}
	if opts.HostConfig.Resources.NanoCPUs != int64(1.5*1e9) {
		t.Fatalf("expected cpu limit to be converted to NanoCPUs, got %d", opts.HostConfig.Resources.NanoCPUs)
	}
	if opts.HostConfig.Resources.Memory != 512*1024*1024 {
		t.Fatalf("expected memory limit to be set, got %d", opts.HostConfig.Resources.Memory)
	}
}

func TestSandboxContainerOptionsRejectMissingSandboxScope(t *testing.T) {
	_, err := sandboxContainerCreateOptions(CreateSandboxOptions{
		Name:       "dobox-p1-sandbox",
		Image:      "dobox/code-sandbox:latest",
		VolumeName: "dobox_project_1",
	})
	if err == nil {
		t.Fatal("expected missing project network to be rejected")
	}
	if !strings.Contains(err.Error(), "network") {
		t.Fatalf("expected network validation error, got %v", err)
	}
}

func TestTarSandboxFileUsesNonRootOwner(t *testing.T) {
	reader, err := tarSandboxFile("settings.json", []byte(`{"theme":"dark"}`), 0o644)
	if err != nil {
		t.Fatalf("tarSandboxFile returned error: %v", err)
	}

	tr := tar.NewReader(reader)
	header, err := tr.Next()
	if err != nil {
		t.Fatalf("expected tar header, got error: %v", err)
	}
	if header.Name != "settings.json" {
		t.Fatalf("expected file name settings.json, got %q", header.Name)
	}
	if header.Mode != 0o644 {
		t.Fatalf("expected mode 0644, got %#o", header.Mode)
	}
	if header.Uid != DefaultSandboxUID || header.Gid != DefaultSandboxGID {
		t.Fatalf("expected owner %d:%d, got %d:%d", DefaultSandboxUID, DefaultSandboxGID, header.Uid, header.Gid)
	}
	if header.Uname != "docode" || header.Gname != "docode" {
		t.Fatalf("expected docode owner names, got %q:%q", header.Uname, header.Gname)
	}
	content, err := io.ReadAll(tr)
	if err != nil {
		t.Fatalf("failed to read tar content: %v", err)
	}
	if string(content) != `{"theme":"dark"}` {
		t.Fatalf("unexpected tar content: %q", string(content))
	}
	if _, err := tr.Next(); err != io.EOF {
		t.Fatalf("expected tar EOF, got %v", err)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
