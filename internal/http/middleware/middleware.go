package middleware

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/golang-cz/httplog"
	"github.com/google/wire"
	"github.com/knadh/koanf/v2"
	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/sessions"
	sessionmiddleware "github.com/libtnb/sessions/middleware"

	"github.com/coder-lulu/nb-agent/internal/biz"
)

var ProviderSet = wire.NewSet(NewMiddlewares)

type Middlewares struct {
	conf      *koanf.Koanf
	log       *slog.Logger
	session   *sessions.Manager
	app       biz.AppRepo
	userToken biz.UserTokenRepo
}

func NewMiddlewares(conf *koanf.Koanf, log *slog.Logger, session *sessions.Manager, app biz.AppRepo, userToken biz.UserTokenRepo) *Middlewares {
	return &Middlewares{
		conf:      conf,
		log:       log,
		session:   session,
		app:       app,
		userToken: userToken,
	}
}

// Globals is a collection of global middleware that will be applied to every request.
func (r *Middlewares) Globals(t *gotext.Locale, mux *chi.Mux) []func(http.Handler) http.Handler {
	return []func(http.Handler) http.Handler{
		middleware.Recoverer,
		//middleware.SupressNotFound(mux),// bug https://github.com/go-chi/chi/pull/940
		httplog.RequestLogger(r.log, &httplog.Options{
			Level:             slog.LevelInfo,
			LogRequestHeaders: []string{"User-Agent"},
		}),
		middleware.Compress(5),
		sessionmiddleware.StartSession(r.session),
		Status(t),
		Entrance(t, r.conf, r.session),
		MustLogin(t, r.session, r.userToken),
		MustInstall(t, r.app),
	}
}
