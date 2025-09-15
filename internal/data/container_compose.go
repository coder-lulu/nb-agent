package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/coder-lulu/nb-agent/internal/app"
	"github.com/coder-lulu/nb-agent/internal/biz"
	"github.com/coder-lulu/nb-agent/pkg/shell"
	"github.com/coder-lulu/nb-agent/pkg/types"
)

type containerComposeRepo struct{}

func NewContainerComposeRepo() biz.ContainerComposeRepo {
	return &containerComposeRepo{}
}

// List 列出所有编排
func (r *containerComposeRepo) List() ([]types.ContainerCompose, error) {
	_ = os.Setenv("PODMAN_COMPOSE_WARNING_LOGS", "false") // 禁用 Podman Compose 的警告日志
	raw, err := shell.Execf("docker compose ls -a --format json")
	_ = os.Unsetenv("PODMAN_COMPOSE_WARNING_LOGS")
	if err != nil {
		return nil, err
	}

	var composeRaws []types.ContainerComposeRaw
	if err = json.Unmarshal([]byte(raw), &composeRaws); err != nil {
		return nil, err
	}

	composeDir := filepath.Join(app.Root, "server", "compose")
	entries, err := os.ReadDir(composeDir)
	if err != nil {
		return nil, err
	}

	var composes []types.ContainerCompose
	index := make(map[string]int)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		path := filepath.Join(composeDir, entry.Name())
		var createdAt time.Time
		if info, err := entry.Info(); err == nil {
			createdAt = info.ModTime()
		}

		composes = append(composes, types.ContainerCompose{
			Name:      entry.Name(),
			Path:      path,
			Status:    "unknown",
			CreatedAt: createdAt,
		})
		index[entry.Name()] = len(composes) - 1
	}

	// 更新状态
	for _, item := range composeRaws {
		if i, ok := index[item.Name]; ok {
			composes[i].Status = item.Status
		}
	}

	return composes, nil
}

// Get 获取编排文件和环境变量内容
func (r *containerComposeRepo) Get(name string) (string, []types.KV, error) {
	content, _ := os.ReadFile(filepath.Join(app.Root, "server", "compose", name, "docker-compose.yml"))
	env, _ := os.ReadFile(filepath.Join(app.Root, "server", "compose", name, ".env"))

	var envs []types.KV
	for _, line := range strings.Split(string(env), "\n") {
		if line == "" {
			continue
		}
		kv := strings.SplitN(line, "=", 2)
		if len(kv) != 2 {
			continue
		}
		envs = append(envs, types.KV{Key: kv[0], Value: kv[1]})
	}

	return string(content), envs, nil // 有意忽略错误，这样可以允许新建文件
}

// Create 创建编排文件
func (r *containerComposeRepo) Create(name, compose string, envs []types.KV) error {
	dir := filepath.Join(app.Root, "server", "compose", name)
	if err := os.MkdirAll(dir, 0644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(compose), 0644); err != nil {
		return err
	}

	var sb strings.Builder
	for _, kv := range envs {
		sb.WriteString(kv.Key)
		sb.WriteString("=")
		sb.WriteString(kv.Value)
		sb.WriteString("\n")
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(sb.String()), 0644); err != nil {
		return err
	}

	return nil
}

// Update 更新编排文件
func (r *containerComposeRepo) Update(name, compose string, envs []types.KV) error {
	dir := filepath.Join(app.Root, "server", "compose", name)
	if err := os.WriteFile(filepath.Join(dir, "docker-compose.yml"), []byte(compose), 0644); err != nil {
		return err
	}

	var sb strings.Builder
	for _, kv := range envs {
		sb.WriteString(kv.Key)
		sb.WriteString("=")
		sb.WriteString(kv.Value)
		sb.WriteString("\n")
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(sb.String()), 0644); err != nil {
		return err
	}

	return nil
}

// Up 启动编排
func (r *containerComposeRepo) Up(name string, force bool) error {
	file := filepath.Join(app.Root, "server", "compose", name, "docker-compose.yml")
	cmd := "docker compose -f %s up -d"
	if force {
		cmd += " --pull always" // 强制拉取镜像
	}
	_ = os.Setenv("PODMAN_COMPOSE_WARNING_LOGS", "false") // 禁用 Podman Compose 的警告日志
	_, err := shell.Execf(cmd, file)
	_ = os.Unsetenv("PODMAN_COMPOSE_WARNING_LOGS")
	return err
}

// Down 停止编排
func (r *containerComposeRepo) Down(name string) error {
	file := filepath.Join(app.Root, "server", "compose", name, "docker-compose.yml")
	_ = os.Setenv("PODMAN_COMPOSE_WARNING_LOGS", "false") // 禁用 Podman Compose 的警告日志
	_, err := shell.Execf("docker compose -f %s down", file)
	_ = os.Unsetenv("PODMAN_COMPOSE_WARNING_LOGS")
	return err
}

// Remove 删除编排
func (r *containerComposeRepo) Remove(name string) error {
	if err := r.Down(name); err != nil {
		return err
	}
	dir := filepath.Join(app.Root, "server", "compose", name)
	return os.RemoveAll(dir)
}
