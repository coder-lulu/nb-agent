package php84

import (
	"github.com/go-chi/chi/v5"
	"github.com/leonelquinteros/gotext"

	"github.com/coder-lulu/nb-agent/internal/apps/php"
	"github.com/coder-lulu/nb-agent/internal/biz"
)

type App struct {
	php *php.App
}

func NewApp(t *gotext.Locale, task biz.TaskRepo) *App {
	return &App{
		php: php.NewApp(t, task),
	}
}

func (s *App) Route(r chi.Router) {
	s.php.Route(84)(r)
}
