package redis

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"

	"github.com/coder-lulu/nb-agent/internal/app"
	"github.com/coder-lulu/nb-agent/internal/service"
	"github.com/coder-lulu/nb-agent/pkg/io"
	"github.com/coder-lulu/nb-agent/pkg/shell"
	"github.com/coder-lulu/nb-agent/pkg/systemctl"
	"github.com/coder-lulu/nb-agent/pkg/types"
)

type App struct {
	t *gotext.Locale
}

func NewApp(t *gotext.Locale) *App {
	return &App{
		t: t,
	}
}

func (s *App) Route(r chi.Router) {
	r.Get("/load", s.Load)
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.UpdateConfig)
}

func (s *App) Load(w http.ResponseWriter, r *http.Request) {
	status, err := systemctl.Status("redis")
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get redis status: %v", err))
		return
	}
	if !status {
		service.Success(w, []types.NV{})
		return
	}

	// 检查 Redis 密码
	withPassword := ""
	config, err := io.Read(fmt.Sprintf("%s/server/redis/redis.conf", app.Root))
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	re := regexp.MustCompile(`^requirepass\s+(.+)`)
	matches := re.FindStringSubmatch(config)
	if len(matches) == 2 {
		withPassword = " -a " + matches[1]
	}

	raw, err := shell.Execf("redis-cli%s info", withPassword)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to get redis info: %v", err))
		return
	}

	infoLines := strings.Split(raw, "\n")
	dataRaw := make(map[string]string)

	for _, item := range infoLines {
		parts := strings.Split(item, ":")
		if len(parts) == 2 {
			dataRaw[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	data := []types.NV{
		{Name: s.t.Get("TCP Port"), Value: dataRaw["tcp_port"]},
		{Name: s.t.Get("Uptime in Days"), Value: dataRaw["uptime_in_days"]},
		{Name: s.t.Get("Connected Clients"), Value: dataRaw["connected_clients"]},
		{Name: s.t.Get("Total Allocated Memory"), Value: dataRaw["used_memory_human"]},
		{Name: s.t.Get("Total Memory Usage"), Value: dataRaw["used_memory_rss_human"]},
		{Name: s.t.Get("Peak Memory Usage"), Value: dataRaw["used_memory_peak_human"]},
		{Name: s.t.Get("Memory Fragmentation Ratio"), Value: dataRaw["mem_fragmentation_ratio"]},
		{Name: s.t.Get("Total Connections Received"), Value: dataRaw["total_connections_received"]},
		{Name: s.t.Get("Total Commands Processed"), Value: dataRaw["total_commands_processed"]},
		{Name: s.t.Get("Commands Per Second"), Value: dataRaw["instantaneous_ops_per_sec"]},
		{Name: s.t.Get("Keyspace Hits"), Value: dataRaw["keyspace_hits"]},
		{Name: s.t.Get("Keyspace Misses"), Value: dataRaw["keyspace_misses"]},
		{Name: s.t.Get("Latest Fork Time (ms)"), Value: dataRaw["latest_fork_usec"]},
	}

	service.Success(w, data)
}

func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	config, err := io.Read(fmt.Sprintf("%s/server/redis/redis.conf", app.Root))
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, config)
}

func (s *App) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdateConfig](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = io.Write(fmt.Sprintf("%s/server/redis/redis.conf", app.Root), req.Config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Restart("redis"); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	service.Success(w, nil)
}
