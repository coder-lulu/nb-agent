package data

import (
	"time"

	"github.com/spf13/cast"
	"gorm.io/gorm"

	"github.com/coder-lulu/nb-agent/internal/biz"
	"github.com/coder-lulu/nb-agent/internal/http/request"
)

type monitorRepo struct {
	db      *gorm.DB
	setting biz.SettingRepo
}

func NewMonitorRepo(db *gorm.DB, setting biz.SettingRepo) biz.MonitorRepo {
	return &monitorRepo{
		db:      db,
		setting: setting,
	}
}

func (r monitorRepo) GetSetting() (*request.MonitorSetting, error) {
	monitor, err := r.setting.Get(biz.SettingKeyMonitor)
	if err != nil {
		return nil, err
	}
	monitorDays, err := r.setting.Get(biz.SettingKeyMonitorDays)
	if err != nil {
		return nil, err
	}

	setting := new(request.MonitorSetting)
	setting.Enabled = cast.ToBool(monitor)
	setting.Days = cast.ToUint(monitorDays)

	return setting, nil
}

func (r monitorRepo) UpdateSetting(setting *request.MonitorSetting) error {
	if err := r.setting.Set(biz.SettingKeyMonitor, cast.ToString(setting.Enabled)); err != nil {
		return err
	}
	if err := r.setting.Set(biz.SettingKeyMonitorDays, cast.ToString(setting.Days)); err != nil {
		return err
	}

	return nil
}

func (r monitorRepo) Clear() error {
	return r.db.Where("1 = 1").Delete(&biz.Monitor{}).Error
}

func (r monitorRepo) List(start, end time.Time) ([]*biz.Monitor, error) {
	monitors := make([]*biz.Monitor, 0)
	if err := r.db.Where("created_at BETWEEN ? AND ?", start, end).Find(&monitors).Error; err != nil {
		return nil, err
	}

	return monitors, nil
}
