package phpmyadmin

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix"
	"github.com/spf13/cast"

	"github.com/coder-lulu/nb-agent/internal/app"
	"github.com/coder-lulu/nb-agent/internal/service"
	"github.com/coder-lulu/nb-agent/pkg/firewall"
	"github.com/coder-lulu/nb-agent/pkg/io"
	"github.com/coder-lulu/nb-agent/pkg/shell"
	"github.com/coder-lulu/nb-agent/pkg/systemctl"
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
	r.Get("/info", s.Info)
	r.Post("/port", s.UpdatePort)
	r.Get("/config", s.GetConfig)
	r.Post("/config", s.UpdateConfig)
}

func (s *App) Info(w http.ResponseWriter, r *http.Request) {
	files, err := os.ReadDir(fmt.Sprintf("%s/server/phpmyadmin", app.Root))
	if err != nil {
		service.Error(w, http.StatusInternalServerError, s.t.Get("phpMyAdmin directory not found"))
		return
	}

	var phpmyadmin string
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "phpmyadmin_") {
			phpmyadmin = f.Name()
		}
	}
	if len(phpmyadmin) == 0 {
		service.Error(w, http.StatusInternalServerError, s.t.Get("phpMyAdmin directory not found"))
		return
	}

	conf, err := io.Read(fmt.Sprintf("%s/server/vhost/phpmyadmin.conf", app.Root))
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	match := regexp.MustCompile(`listen\s+(\d+);`).FindStringSubmatch(conf)
	if len(match) == 0 {
		service.Error(w, http.StatusInternalServerError, s.t.Get("phpMyAdmin port not found"))
		return
	}

	service.Success(w, chix.M{
		"path": phpmyadmin,
		"port": cast.ToInt(match[1]),
	})
}

func (s *App) UpdatePort(w http.ResponseWriter, r *http.Request) {
	req, err := service.Bind[UpdatePort](r)
	if err != nil {
		service.Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	conf, err := io.Read(fmt.Sprintf("%s/server/vhost/phpmyadmin.conf", app.Root))
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	conf = regexp.MustCompile(`listen\s+(\d+);`).ReplaceAllString(conf, "listen "+cast.ToString(req.Port)+";")
	if err = io.Write(fmt.Sprintf("%s/server/vhost/phpmyadmin.conf", app.Root), conf, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	fw := firewall.NewFirewall()
	err = fw.Port(firewall.FireInfo{
		Type:      firewall.TypeNormal,
		PortStart: req.Port,
		PortEnd:   req.Port,
		Direction: firewall.DirectionIn,
		Strategy:  firewall.StrategyAccept,
	}, firewall.OperationAdd)
	if err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Reload("nginx"); err != nil {
		_, err = shell.Execf("nginx -t")
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to reload nginx: %v", err))
		return
	}

	service.Success(w, nil)
}

func (s *App) GetConfig(w http.ResponseWriter, r *http.Request) {
	config, err := io.Read(fmt.Sprintf("%s/server/vhost/phpmyadmin.conf", app.Root))
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

	if err = io.Write(fmt.Sprintf("%s/server/vhost/phpmyadmin.conf", app.Root), req.Config, 0644); err != nil {
		service.Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	if err = systemctl.Reload("nginx"); err != nil {
		_, err = shell.Execf("nginx -t")
		service.Error(w, http.StatusInternalServerError, s.t.Get("failed to reload nginx: %v", err))
		return
	}

	service.Success(w, nil)
}
