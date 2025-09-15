package bootstrap

import (
	"github.com/gookit/validate"
	"github.com/gookit/validate/locales/ruru"
	"github.com/gookit/validate/locales/zhcn"
	"github.com/gookit/validate/locales/zhtw"
	"github.com/knadh/koanf/v2"
	"gorm.io/gorm"

	"github.com/coder-lulu/nb-agent/internal/http/rule"
)

// NewValidator just for register global rules
func NewValidator(conf *koanf.Koanf, db *gorm.DB) *validate.Validation {
	if conf.String("app.locale") == "zh_CN" {
		zhcn.RegisterGlobal()
	} else if conf.String("app.locale") == "zh_TW" {
		zhtw.RegisterGlobal()
	} else if conf.String("app.locale") == "ru_RU" {
		ruru.RegisterGlobal()
	}
	validate.Config(func(opt *validate.GlobalOption) {
		opt.StopOnError = false
		opt.SkipOnEmpty = true
	})

	// register global rules
	rule.GlobalRules(db)

	return validate.NewEmpty()
}
