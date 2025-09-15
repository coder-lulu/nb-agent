package data

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"

	"github.com/knadh/koanf/v2"
	"github.com/leonelquinteros/gotext"
	"github.com/spf13/cast"
	"go.yaml.in/yaml/v3"
	"gorm.io/gorm"

	"github.com/coder-lulu/nb-agent/internal/app"
	"github.com/coder-lulu/nb-agent/internal/biz"
	"github.com/coder-lulu/nb-agent/internal/http/request"
	"github.com/coder-lulu/nb-agent/pkg/cert"
	"github.com/coder-lulu/nb-agent/pkg/firewall"
	"github.com/coder-lulu/nb-agent/pkg/io"
	"github.com/coder-lulu/nb-agent/pkg/os"
	"github.com/coder-lulu/nb-agent/pkg/systemctl"
	"github.com/coder-lulu/nb-agent/pkg/types"
)

type settingRepo struct {
	t     *gotext.Locale
	cache sync.Map
	db    *gorm.DB
	conf  *koanf.Koanf
	task  biz.TaskRepo
}

func NewSettingRepo(t *gotext.Locale, db *gorm.DB, conf *koanf.Koanf, task biz.TaskRepo) biz.SettingRepo {
	return &settingRepo{
		t:    t,
		db:   db,
		conf: conf,
		task: task,
	}
}

func (r *settingRepo) Get(key biz.SettingKey, defaultValue ...string) (string, error) {
	if cache, ok := r.cache.Load(key); ok {
		if v, ok := cache.(string); ok {
			return v, nil
		}
		r.cache.Delete(key)
	}

	setting := new(biz.Setting)
	if err := r.db.Where("key = ?", key).First(setting).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return "", err
		}
	}

	if setting.Value == "" && len(defaultValue) > 0 {
		return defaultValue[0], nil
	}

	return setting.Value, nil
}

func (r *settingRepo) GetBool(key biz.SettingKey, defaultValue ...bool) (bool, error) {
	if cache, ok := r.cache.Load(key); ok {
		if v, ok := cache.(bool); ok {
			return v, nil
		}
		r.cache.Delete(key)
	}

	setting := new(biz.Setting)
	if err := r.db.Where("key = ?", key).First(setting).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return false, err
		}
	}

	if setting.Value == "" && len(defaultValue) > 0 {
		return defaultValue[0], nil
	}

	return cast.ToBool(setting.Value), nil
}

func (r *settingRepo) GetInt(key biz.SettingKey, defaultValue ...int) (int, error) {
	if cache, ok := r.cache.Load(key); ok {
		if v, ok := cache.(int); ok {
			return v, nil
		}
		r.cache.Delete(key)
	}

	setting := new(biz.Setting)
	if err := r.db.Where("key = ?", key).First(setting).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, err
		}
	}

	if setting.Value == "" && len(defaultValue) > 0 {
		return defaultValue[0], nil
	}

	return cast.ToInt(setting.Value), nil
}

func (r *settingRepo) GetSlice(key biz.SettingKey, defaultValue ...[]string) ([]string, error) {
	if cache, ok := r.cache.Load(key); ok {
		if v, ok := cache.([]string); ok {
			return v, nil
		}
		r.cache.Delete(key)
	}

	setting := new(biz.Setting)
	if err := r.db.Where("key = ?", key).First(setting).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}

	// 设置值为空时提前返回
	slice := make([]string, 0)
	if setting.Value == "" {
		if len(defaultValue) > 0 {
			return defaultValue[0], nil
		}
		return slice, nil
	}

	if err := json.Unmarshal([]byte(setting.Value), &slice); err != nil {
		return nil, err
	}
	if len(slice) == 0 && len(defaultValue) > 0 {
		return defaultValue[0], nil
	}

	return slice, nil
}

func (r *settingRepo) Set(key biz.SettingKey, value string) error {
	setting := new(biz.Setting)
	if err := r.db.Where("key = ?", key).First(setting).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}

	setting.Key = key
	setting.Value = value
	if err := r.db.Save(setting).Error; err != nil {
		return err
	}

	r.cache.Store(key, value)

	return nil
}

func (r *settingRepo) SetSlice(key biz.SettingKey, value []string) error {
	setting := new(biz.Setting)
	if err := r.db.Where("key = ?", key).First(setting).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
	}

	setting.Key = key
	if len(value) == 0 {
		setting.Value = "[]"
	} else {
		b, err := json.Marshal(value)
		if err != nil {
			return err
		}
		setting.Value = string(b)
	}

	if err := r.db.Save(setting).Error; err != nil {
		return err
	}

	r.cache.Store(key, value)

	return nil
}

func (r *settingRepo) Delete(key biz.SettingKey) error {
	setting := new(biz.Setting)
	if err := r.db.Where("key = ?", key).Delete(setting).Error; err != nil {
		return err
	}

	r.cache.Delete(key)

	return nil
}

func (r *settingRepo) GetPanel() (*request.SettingPanel, error) {
	name, err := r.Get(biz.SettingKeyName)
	if err != nil {
		return nil, err
	}
	channel, err := r.Get(biz.SettingKeyChannel)
	if err != nil {
		return nil, err
	}
	offlineMode, err := r.GetBool(biz.SettingKeyOfflineMode)
	if err != nil {
		return nil, err
	}
	autoUpdate, err := r.GetBool(biz.SettingKeyAutoUpdate)
	if err != nil {
		return nil, err
	}
	websitePath, err := r.Get(biz.SettingKeyWebsitePath)
	if err != nil {
		return nil, err
	}
	backupPath, err := r.Get(biz.SettingKeyBackupPath)
	if err != nil {
		return nil, err
	}

	crt, err := io.Read(filepath.Join(app.Root, "panel/storage/cert.pem"))
	if err != nil {
		return nil, err
	}
	key, err := io.Read(filepath.Join(app.Root, "panel/storage/cert.key"))
	if err != nil {
		return nil, err
	}

	return &request.SettingPanel{
		Name:        name,
		Channel:     channel,
		Locale:      r.conf.String("app.locale"),
		Entrance:    r.conf.String("http.entrance"),
		OfflineMode: offlineMode,
		AutoUpdate:  autoUpdate,
		Lifetime:    uint(r.conf.Int("session.lifetime")),
		BindDomain:  r.conf.Strings("http.bind_domain"),
		BindIP:      r.conf.Strings("http.bind_ip"),
		BindUA:      r.conf.Strings("http.bind_ua"),
		WebsitePath: websitePath,
		BackupPath:  backupPath,
		Port:        uint(r.conf.Int("http.port")),
		HTTPS:       r.conf.Bool("http.tls"),
		Cert:        crt,
		Key:         key,
	}, nil
}

func (r *settingRepo) UpdatePanel(req *request.SettingPanel) (bool, error) {
	if err := r.Set(biz.SettingKeyName, req.Name); err != nil {
		return false, err
	}
	if err := r.Set(biz.SettingKeyChannel, req.Channel); err != nil {
		return false, err
	}
	if err := r.Set(biz.SettingKeyOfflineMode, cast.ToString(req.OfflineMode)); err != nil {
		return false, err
	}
	if err := r.Set(biz.SettingKeyAutoUpdate, cast.ToString(req.AutoUpdate)); err != nil {
		return false, err
	}
	if err := r.Set(biz.SettingKeyWebsitePath, req.WebsitePath); err != nil {
		return false, err
	}
	if err := r.Set(biz.SettingKeyBackupPath, req.BackupPath); err != nil {
		return false, err
	}

	// 下面是需要需要重启的设置
	// 面板HTTPS
	restartFlag := false
	oldCert, _ := io.Read(filepath.Join(app.Root, "panel/storage/cert.pem"))
	oldKey, _ := io.Read(filepath.Join(app.Root, "panel/storage/cert.key"))
	if oldCert != req.Cert || oldKey != req.Key {
		if r.task.HasRunningTask() {
			return false, errors.New(r.t.Get("background task is running, modifying some settings is prohibited, please try again later"))
		}
		restartFlag = true
	}
	if _, err := cert.ParseCert(req.Cert); err != nil {
		return false, errors.New(r.t.Get("failed to parse certificate: %v", err))
	}
	if _, err := cert.ParseKey(req.Key); err != nil {
		return false, errors.New(r.t.Get("failed to parse private key: %v", err))
	}
	if err := io.Write(filepath.Join(app.Root, "panel/storage/cert.pem"), req.Cert, 0644); err != nil {
		return false, err
	}
	if err := io.Write(filepath.Join(app.Root, "panel/storage/cert.key"), req.Key, 0644); err != nil {
		return false, err
	}

	// 面板主配置
	config := new(types.PanelConfig)
	raw, err := io.Read("/usr/local/etc/panel/config.yml")
	if err != nil {
		return false, err
	}
	if err = yaml.Unmarshal([]byte(raw), config); err != nil {
		return false, err
	}

	if req.Port != config.HTTP.Port {
		if os.TCPPortInUse(req.Port) {
			return false, errors.New(r.t.Get("port is already in use"))
		}
		// 放行端口
		if ok, _ := systemctl.IsEnabled("firewalld"); ok {
			fw := firewall.NewFirewall()
			err = fw.Port(firewall.FireInfo{
				Type:      firewall.TypeNormal,
				PortStart: req.Port,
				PortEnd:   req.Port,
				Direction: firewall.DirectionIn,
				Strategy:  firewall.StrategyAccept,
			}, firewall.OperationAdd)
			if err != nil {
				return false, err
			}
		}
	}

	config.App.Locale = req.Locale
	config.HTTP.Port = req.Port
	config.HTTP.Entrance = req.Entrance
	config.HTTP.TLS = req.HTTPS
	config.HTTP.BindDomain = req.BindDomain
	config.HTTP.BindIP = req.BindIP
	config.HTTP.BindUA = req.BindUA
	config.Session.Lifetime = req.Lifetime

	encoded, err := yaml.Marshal(config)
	if err != nil {
		return false, err
	}
	if raw != string(encoded) {
		if r.task.HasRunningTask() {
			return false, errors.New(r.t.Get("background task is running, modifying some settings is prohibited, please try again later"))
		}
		restartFlag = true
	}
	if err = io.Write("/usr/local/etc/panel/config.yml", string(encoded), 0644); err != nil {
		return false, err
	}

	return restartFlag, nil
}

func (r *settingRepo) UpdateCert(req *request.SettingCert) error {
	if r.task.HasRunningTask() {
		return errors.New(r.t.Get("background task is running, modifying some settings is prohibited, please try again later"))
	}
	if _, err := cert.ParseCert(req.Cert); err != nil {
		return errors.New(r.t.Get("failed to parse certificate: %v", err))
	}
	if _, err := cert.ParseKey(req.Key); err != nil {
		return errors.New(r.t.Get("failed to parse private key: %v", err))
	}

	if err := io.Write(filepath.Join(app.Root, "panel/storage/cert.pem"), req.Cert, 0644); err != nil {
		return err
	}
	if err := io.Write(filepath.Join(app.Root, "panel/storage/cert.key"), req.Key, 0644); err != nil {
		return err
	}

	return nil
}
