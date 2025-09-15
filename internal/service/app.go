package service

import (
	"net/http"

	"github.com/leonelquinteros/gotext"
	"github.com/libtnb/chix"

	"github.com/coder-lulu/nb-agent/internal/biz"
	"github.com/coder-lulu/nb-agent/internal/http/request"
	"github.com/coder-lulu/nb-agent/pkg/types"
)

type AppService struct {
	t           *gotext.Locale
	appRepo     biz.AppRepo
	cacheRepo   biz.CacheRepo
	settingRepo biz.SettingRepo
}

func NewAppService(t *gotext.Locale, app biz.AppRepo, cache biz.CacheRepo, setting biz.SettingRepo) *AppService {
	return &AppService{
		t:           t,
		appRepo:     app,
		cacheRepo:   cache,
		settingRepo: setting,
	}
}

func (s *AppService) List(w http.ResponseWriter, r *http.Request) {
	all := s.appRepo.All()
	installedApps, err := s.appRepo.Installed()
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}
	installedAppMap := make(map[string]*biz.App)

	for _, p := range installedApps {
		installedAppMap[p.Slug] = p
	}

	var apps []types.AppCenter
	for _, item := range all {
		installed, installedChannel, installedVersion, updateExist, show := false, "", "", false, false
		if _, ok := installedAppMap[item.Slug]; ok {
			installed = true
			installedChannel = installedAppMap[item.Slug].Channel
			installedVersion = installedAppMap[item.Slug].Version
			updateExist = s.appRepo.UpdateExist(item.Slug)
			show = installedAppMap[item.Slug].Show
		}
		apps = append(apps, types.AppCenter{
			Icon:        item.Icon,
			Name:        item.Name,
			Description: item.Description,
			Slug:        item.Slug,
			Channels: []struct {
				Slug      string `json:"slug"`
				Name      string `json:"name"`
				Panel     string `json:"panel"`
				Install   string `json:"-"`
				Uninstall string `json:"-"`
				Update    string `json:"-"`
				Subs      []struct {
					Log     string `json:"log"`
					Version string `json:"version"`
				} `json:"subs"`
			}(item.Channels),
			Installed:        installed,
			InstalledChannel: installedChannel,
			InstalledVersion: installedVersion,
			UpdateExist:      updateExist,
			Show:             show,
		})
	}

	paged, total := Paginate(r, apps)

	Success(w, chix.M{
		"total": total,
		"items": paged,
	})
}

func (s *AppService) Install(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.App](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.appRepo.Install(req.Channel, req.Slug); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *AppService) Uninstall(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.AppSlug](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.appRepo.UnInstall(req.Slug); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *AppService) Update(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.AppSlug](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.appRepo.Update(req.Slug); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *AppService) UpdateShow(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.AppUpdateShow](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	if err = s.appRepo.UpdateShow(req.Slug, req.Show); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}

func (s *AppService) IsInstalled(w http.ResponseWriter, r *http.Request) {
	req, err := Bind[request.AppSlug](r)
	if err != nil {
		Error(w, http.StatusUnprocessableEntity, "%v", err)
		return
	}

	app, err := s.appRepo.Get(req.Slug)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	installed, err := s.appRepo.IsInstalled(req.Slug)
	if err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, chix.M{
		"name":      app.Name,
		"installed": installed,
	})
}

func (s *AppService) UpdateCache(w http.ResponseWriter, r *http.Request) {
	if offline, _ := s.settingRepo.GetBool(biz.SettingKeyOfflineMode); offline {
		Error(w, http.StatusForbidden, s.t.Get("Unable to update app list cache in offline mode"))
		return
	}

	if err := s.cacheRepo.UpdateApps(); err != nil {
		Error(w, http.StatusInternalServerError, "%v", err)
		return
	}

	Success(w, nil)
}
