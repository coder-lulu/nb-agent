package bootstrap

import (
	"github.com/knadh/koanf/v2"
	"github.com/leonelquinteros/gotext"

	"github.com/coder-lulu/nb-agent/pkg/embed"
)

func NewT(conf *koanf.Koanf) (*gotext.Locale, error) {
	locale := conf.String("app.locale")
	l := gotext.NewLocaleFSWithPath(locale, embed.LocalesFS, "locales")
	l.AddDomain("backend")

	return l, nil
}
