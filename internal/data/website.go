package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/leonelquinteros/gotext"
	"github.com/samber/lo"
	"github.com/spf13/cast"
	"gorm.io/gorm"

	"github.com/coder-lulu/nb-agent/internal/app"
	"github.com/coder-lulu/nb-agent/internal/biz"
	"github.com/coder-lulu/nb-agent/internal/http/request"
	"github.com/coder-lulu/nb-agent/pkg/acme"
	"github.com/coder-lulu/nb-agent/pkg/api"
	"github.com/coder-lulu/nb-agent/pkg/cert"
	"github.com/coder-lulu/nb-agent/pkg/embed"
	"github.com/coder-lulu/nb-agent/pkg/io"
	"github.com/coder-lulu/nb-agent/pkg/nginx"
	"github.com/coder-lulu/nb-agent/pkg/punycode"
	"github.com/coder-lulu/nb-agent/pkg/shell"
	"github.com/coder-lulu/nb-agent/pkg/systemctl"
	"github.com/coder-lulu/nb-agent/pkg/types"
)

type websiteRepo struct {
	t              *gotext.Locale
	db             *gorm.DB
	cache          biz.CacheRepo
	database       biz.DatabaseRepo
	databaseServer biz.DatabaseServerRepo
	databaseUser   biz.DatabaseUserRepo
	cert           biz.CertRepo
	certAccount    biz.CertAccountRepo
}

func NewWebsiteRepo(t *gotext.Locale, db *gorm.DB, cache biz.CacheRepo, database biz.DatabaseRepo, databaseServer biz.DatabaseServerRepo, databaseUser biz.DatabaseUserRepo, cert biz.CertRepo, certAccount biz.CertAccountRepo) biz.WebsiteRepo {
	return &websiteRepo{
		t:              t,
		db:             db,
		cache:          cache,
		database:       database,
		databaseServer: databaseServer,
		databaseUser:   databaseUser,
		cert:           cert,
		certAccount:    certAccount,
	}
}

func (r *websiteRepo) GetRewrites() (map[string]string, error) {
	cached, err := r.cache.Get(biz.CacheKeyRewrites)
	if err != nil {
		return nil, err
	}

	var rewrites api.Rewrites
	if err = json.Unmarshal([]byte(cached), &rewrites); err != nil {
		return nil, err
	}

	rw := make(map[string]string)
	for rewrite := range slices.Values(rewrites) {
		rw[rewrite.Name] = rewrite.Content
	}

	return rw, nil
}

func (r *websiteRepo) UpdateDefaultConfig(req *request.WebsiteDefaultConfig) error {
	if err := io.Write(filepath.Join(app.Root, "server/nginx/html/index.html"), req.Index, 0644); err != nil {
		return err
	}
	if err := io.Write(filepath.Join(app.Root, "server/nginx/html/stop.html"), req.Stop, 0644); err != nil {
		return err
	}

	return systemctl.Reload("nginx")
}

func (r *websiteRepo) Count() (int64, error) {
	var count int64
	if err := r.db.Model(&biz.Website{}).Count(&count).Error; err != nil {
		return 0, err
	}

	return count, nil
}

func (r *websiteRepo) Get(id uint) (*types.WebsiteSetting, error) {
	website := new(biz.Website)
	if err := r.db.Where("id", id).First(website).Error; err != nil {
		return nil, err
	}
	// 解析nginx配置
	config, err := io.Read(filepath.Join(app.Root, "server/vhost", website.Name+".conf"))
	if err != nil {
		return nil, err
	}
	p, err := nginx.NewParser(config)
	if err != nil {
		return nil, err
	}

	setting := new(types.WebsiteSetting)
	setting.ID = website.ID
	setting.Name = website.Name
	setting.Type = website.Type
	setting.Path = website.Path
	setting.HTTPS = website.Https
	setting.PHP = p.GetPHP()
	setting.Raw = config
	// 监听地址
	listens, err := p.GetListen()
	if err != nil {
		return nil, err
	}
	setting.Listens = lo.Map(
		lo.UniqBy(listens, func(listen []string) string {
			if len(listen) == 0 {
				return ""
			}
			return listen[0]
		}),
		func(listen []string, _ int) types.WebsiteListen {
			addr := listen[0]
			grouped := lo.GroupBy(listens, func(listen []string) string {
				if len(listen) == 0 {
					return ""
				}
				return listen[0]
			})[addr]
			return types.WebsiteListen{
				Address: addr,
				HTTPS:   lo.SomeBy(grouped, func(listen []string) bool { return lo.Contains(listen, "ssl") }),
				QUIC:    lo.SomeBy(grouped, func(listen []string) bool { return lo.Contains(listen, "quic") }),
			}
		},
	)
	// 域名
	domains, err := p.GetServerName()
	if err != nil {
		return nil, err
	}
	domains, err = punycode.DecodeDomains(domains)
	if err != nil {
		return nil, err
	}
	setting.Domains = domains
	// 运行目录
	root, _ := p.GetRoot()
	setting.Root = root
	// 默认文档
	index, _ := p.GetIndex()
	setting.Index = index
	// 防跨站
	if io.Exists(filepath.Join(setting.Root, ".user.ini")) {
		userIni, _ := io.Read(filepath.Join(setting.Root, ".user.ini"))
		if strings.Contains(userIni, "open_basedir") {
			setting.OpenBasedir = true
		}
	}
	// HTTPS
	if setting.HTTPS {
		setting.HTTPRedirect = p.GetHTTPSRedirect()
		setting.HSTS = p.GetHSTS()
		setting.OCSP = p.GetOCSP()
	}
	// 证书
	crt, _ := io.Read(filepath.Join(app.Root, "server/vhost/cert", website.Name+".pem"))
	setting.SSLCertificate = crt
	key, _ := io.Read(filepath.Join(app.Root, "server/vhost/cert", website.Name+".key"))
	setting.SSLCertificateKey = key
	// 解析证书信息
	if decode, err := cert.ParseCert(crt); err == nil {
		setting.SSLNotBefore = decode.NotBefore.Format(time.DateTime)
		setting.SSLNotAfter = decode.NotAfter.Format(time.DateTime)
		setting.SSLIssuer = decode.Issuer.CommonName
		setting.SSLOCSPServer = decode.OCSPServer
		setting.SSLDNSNames = decode.DNSNames
	}
	// 伪静态
	rewrite, _ := io.Read(filepath.Join(app.Root, "server/vhost/rewrite", website.Name+".conf"))
	setting.Rewrite = rewrite
	// 访问日志
	if setting.Log, err = p.GetAccessLog(); err != nil {
		setting.Log = fmt.Sprintf("%s/wwwlogs/%s.log", app.Root, website.Name)
	}
	// 错误日志
	if setting.ErrorLog, err = p.GetErrorLog(); err != nil {
		setting.ErrorLog = fmt.Sprintf("%s/wwwlogs/%s.error.log", app.Root, website.Name)
	}

	return setting, err
}

func (r *websiteRepo) GetByName(name string) (*types.WebsiteSetting, error) {
	website := new(biz.Website)
	if err := r.db.Where("name", name).First(website).Error; err != nil {
		return nil, err
	}

	return r.Get(website.ID)

}

func (r *websiteRepo) List(page, limit uint) ([]*biz.Website, int64, error) {
	websites := make([]*biz.Website, 0)
	var total int64

	if err := r.db.Model(&biz.Website{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := r.db.Offset(int((page - 1) * limit)).Limit(int(limit)).Find(&websites).Error; err != nil {
		return nil, 0, err
	}

	// 取证书剩余有效时间
	for _, website := range websites {
		crt, _ := io.Read(filepath.Join(app.Root, "server/vhost/cert", website.Name+".pem"))
		if decode, err := cert.ParseCert(crt); err == nil {
			hours := time.Until(decode.NotAfter).Hours()
			website.CertExpire = fmt.Sprintf("%.2f", hours/24)
		}
	}

	return websites, total, nil
}

func (r *websiteRepo) Create(req *request.WebsiteCreate) (*biz.Website, error) {
	// 初始化nginx配置
	config := nginx.DefaultConf
	if app.Locale == "zh_CN" {
		config = nginx.DefaultConfZh
	}
	p, err := nginx.NewParser(config)
	if err != nil {
		return nil, err
	}
	// 监听地址
	var listens [][]string
	for _, listen := range req.Listens {
		listens = append(listens, []string{listen})
	}
	if err = p.SetListen(listens); err != nil {
		return nil, err
	}
	// 域名
	domains, err := punycode.EncodeDomains(req.Domains)
	if err != nil {
		return nil, err
	}
	if err = p.SetServerName(domains); err != nil {
		return nil, err
	}
	// 运行目录
	if err = p.SetRoot(req.Path); err != nil {
		return nil, err
	}
	// PHP
	if err = p.SetPHP(req.PHP); err != nil {
		return nil, err
	}
	// 伪静态和acme
	includes, comments, err := p.GetIncludes()
	if err != nil {
		return nil, err
	}
	includes = append(includes, filepath.Join(app.Root, "server/vhost/rewrite", req.Name+".conf"))
	includes = append(includes, filepath.Join(app.Root, "server/vhost/acme", req.Name+".conf"))
	comments = append(comments, []string{r.t.Get("# Rewrite rule")})
	comments = append(comments, []string{"# acme http-01"})
	if err = p.SetIncludes(includes, comments); err != nil {
		return nil, err
	}
	// 日志
	if err = p.SetAccessLog(filepath.Join(app.Root, "wwwlogs", req.Name+".log")); err != nil {
		return nil, err
	}
	if err = p.SetErrorLog(filepath.Join(app.Root, "wwwlogs", req.Name+".error.log")); err != nil {
		return nil, err
	}

	// 初始化网站目录
	if err = os.MkdirAll(req.Path, 0755); err != nil {
		return nil, err
	}
	var index []byte
	if app.Locale == "zh_CN" {
		index, err = embed.WebsiteFS.ReadFile(filepath.Join("website", "index_zh.html"))
	} else {
		index, err = embed.WebsiteFS.ReadFile(filepath.Join("website", "index.html"))
	}
	if err != nil {
		return nil, errors.New(r.t.Get("failed to get index template file: %v", err))
	}
	if err = io.Write(filepath.Join(req.Path, "index.html"), string(index), 0644); err != nil {
		return nil, err
	}
	var notFound []byte
	if app.Locale == "zh_CN" {
		notFound, err = embed.WebsiteFS.ReadFile(filepath.Join("website", "404_zh.html"))
	} else {
		notFound, err = embed.WebsiteFS.ReadFile(filepath.Join("website", "404.html"))
	}
	if err != nil {
		return nil, errors.New(r.t.Get("failed to get 404 template file: %v", err))
	}
	if err = io.Write(filepath.Join(req.Path, "404.html"), string(notFound), 0644); err != nil {
		return nil, err
	}

	// 写nginx配置
	if err = io.Write(filepath.Join(app.Root, "server/vhost", req.Name+".conf"), p.Dump(), 0644); err != nil {
		return nil, err
	}
	if err = io.Write(filepath.Join(app.Root, "server/vhost/rewrite", req.Name+".conf"), "", 0644); err != nil {
		return nil, err
	}
	if err = io.Write(filepath.Join(app.Root, "server/vhost/acme", req.Name+".conf"), "", 0644); err != nil {
		return nil, err
	}
	if err = io.Write(filepath.Join(app.Root, "server/vhost/cert", req.Name+".pem"), "", 0644); err != nil {
		return nil, err
	}
	if err = io.Write(filepath.Join(app.Root, "server/vhost/cert", req.Name+".key"), "", 0644); err != nil {
		return nil, err
	}

	// 设置目录权限
	if err = io.Chmod(req.Path, 0755); err != nil {
		return nil, err
	}
	if err = io.Chown(req.Path, "www", "www"); err != nil {
		return nil, err
	}

	// PHP 网站默认开启防跨站
	if req.PHP > 0 {
		userIni := filepath.Join(req.Path, ".user.ini")
		if !io.Exists(userIni) {
			if err = io.Write(userIni, fmt.Sprintf("open_basedir=%s:/tmp/", req.Path), 0644); err != nil {
				return nil, err
			}
		}
		_, _ = shell.Execf(`chattr +i '%s'`, userIni)
	}

	// 创建面板网站
	w := &biz.Website{
		Name:   req.Name,
		Type:   "php", // TODO 支持网站类型
		Status: true,
		Path:   req.Path,
		Https:  false,
		Remark: req.Remark,
	}
	if err = r.db.Create(w).Error; err != nil {
		return nil, err
	}

	if err = systemctl.Reload("nginx"); err != nil {
		_, err = shell.Execf("nginx -t")
		return nil, err
	}

	// 创建数据库
	name := "local_" + req.DBType
	if req.DB {
		server, err := r.databaseServer.GetByName(name)
		if err != nil {
			return nil, errors.New(r.t.Get("can't find %s database server, please add it first", name))
		}
		if err = r.database.Create(&request.DatabaseCreate{
			ServerID:   server.ID,
			Name:       req.DBName,
			CreateUser: true,
			Username:   req.DBUser,
			Password:   req.DBPassword,
			Host:       "localhost",
			Comment:    fmt.Sprintf("website %s", req.Name),
		}); err != nil {
			return nil, err
		}
	}

	return w, nil
}

func (r *websiteRepo) Update(req *request.WebsiteUpdate) error {
	website := new(biz.Website)
	if err := r.db.Where("id", req.ID).First(website).Error; err != nil {
		return err
	}

	// 解析nginx配置
	config, err := io.Read(filepath.Join(app.Root, "server/vhost", website.Name+".conf"))
	if err != nil {
		return err
	}
	// 如果修改了原文，直接写入返回
	if strings.TrimSpace(config) != strings.TrimSpace(req.Raw) {
		if err = io.Write(filepath.Join(app.Root, "server/vhost", website.Name+".conf"), req.Raw, 0644); err != nil {
			return err
		}
		if err = systemctl.Reload("nginx"); err != nil {
			_, err = shell.Execf("nginx -t")
			return err
		}
		return nil
	}

	// 初始化nginx配置
	p, err := nginx.NewParser(config)
	if err != nil {
		return err
	}
	// 监听地址
	var listens [][]string
	quic := false
	for _, listen := range req.Listens {
		if !listen.HTTPS && !listen.QUIC {
			listens = append(listens, []string{listen.Address})
		}
		if listen.HTTPS {
			listens = append(listens, []string{listen.Address, "ssl"})
		}
		if listen.QUIC {
			quic = true
			listens = append(listens, []string{listen.Address, "quic"})
		}
	}
	if err = p.SetListen(listens); err != nil {
		return err
	}
	// 域名
	domains, err := punycode.EncodeDomains(req.Domains)
	if err != nil {
		return err
	}
	if err = p.SetServerName(domains); err != nil {
		return err
	}
	// 首页文件
	if err = p.SetIndex(req.Index); err != nil {
		return err
	}
	// 运行目录
	if !io.Exists(req.Root) {
		return errors.New(r.t.Get("runtime directory does not exist"))
	}
	if err = p.SetRoot(req.Root); err != nil {
		return err
	}
	// 运行目录
	if !io.Exists(req.Path) {
		return errors.New(r.t.Get("website directory does not exist"))
	}
	website.Path = req.Path
	// PHP
	if err = p.SetPHP(req.PHP); err != nil {
		return err
	}
	// HTTPS
	certPath := filepath.Join(app.Root, "server/vhost/cert", website.Name+".pem")
	keyPath := filepath.Join(app.Root, "server/vhost/cert", website.Name+".key")
	if err = io.Write(certPath, req.SSLCertificate, 0644); err != nil {
		return err
	}
	if err = io.Write(keyPath, req.SSLCertificateKey, 0644); err != nil {
		return err
	}
	website.Https = req.HTTPS
	if req.HTTPS {
		if _, err = cert.ParseCert(req.SSLCertificate); err != nil {
			return errors.New(r.t.Get("failed to parse certificate: %v", err))
		}
		if _, err = cert.ParseKey(req.SSLCertificateKey); err != nil {
			return errors.New(r.t.Get("failed to parse private key: %v", err))
		}
		if err = p.SetHTTPS(certPath, keyPath); err != nil {
			return err
		}
		if err = p.SetHTTPRedirect(req.HTTPRedirect); err != nil {
			return err
		}
		if err = p.SetHSTS(req.HSTS); err != nil {
			return err
		}
		if err = p.SetOCSP(req.OCSP); err != nil {
			return err
		}
	} else {
		if err = p.ClearSetHTTPS(); err != nil {
			return err
		}
		if err = p.SetHTTPRedirect(false); err != nil {
			return err
		}
		if err = p.SetHSTS(false); err != nil {
			return err
		}
		if err = p.SetOCSP(false); err != nil {
			return err
		}
	}
	if quic {
		if err = p.SetAltSvc(`'h3=":$server_port"; ma=2592000'`); err != nil {
			return err
		}
	} else {
		if err = p.SetAltSvc(``); err != nil {
			return err
		}
	}
	// 防跨站
	if !strings.HasSuffix(req.Root, "/") {
		req.Root += "/"
	}
	userIni := filepath.Join(req.Root, ".user.ini")
	if req.OpenBasedir {
		if !io.Exists(userIni) {
			if err = io.Write(userIni, fmt.Sprintf("open_basedir=%s:/tmp/", req.Path), 0644); err != nil {
				return err
			}
		}
		_, _ = shell.Execf(`chattr +i '%s'`, userIni)
	} else {
		if io.Exists(userIni) {
			if err = io.Remove(userIni); err != nil {
				return err
			}
		}
	}

	if err = io.Write(filepath.Join(app.Root, "server/vhost", website.Name+".conf"), p.Dump(), 0644); err != nil {
		return err
	}
	if err = io.Write(filepath.Join(app.Root, "server/vhost/rewrite", website.Name+".conf"), req.Rewrite, 0644); err != nil {
		return err
	}

	if err = r.db.Save(website).Error; err != nil {
		return err
	}

	if err = systemctl.Reload("nginx"); err != nil {
		_, err = shell.Execf("nginx -t")
		return err
	}

	return nil
}

func (r *websiteRepo) Delete(req *request.WebsiteDelete) error {
	website := new(biz.Website)
	if err := r.db.Preload("Cert").Where("id", req.ID).First(website).Error; err != nil {
		return err
	}
	if website.Cert != nil {
		return errors.New(r.t.Get("website %s has bound certificates, please delete the certificate first", website.Name))
	}

	_ = io.Remove(filepath.Join(app.Root, "server/vhost", website.Name+".conf"))
	_ = io.Remove(filepath.Join(app.Root, "server/vhost/rewrite", website.Name+".conf"))
	_ = io.Remove(filepath.Join(app.Root, "server/vhost/acme", website.Name+".conf"))
	_ = io.Remove(filepath.Join(app.Root, "server/vhost/cert", website.Name+".pem"))
	_ = io.Remove(filepath.Join(app.Root, "server/vhost/cert", website.Name+".key"))
	_ = io.Remove(filepath.Join(app.Root, "wwwlogs", website.Name+".log"))
	_ = io.Remove(filepath.Join(app.Root, "wwwlogs", website.Name+".error.log"))

	if req.Path {
		_ = io.Remove(website.Path)
	}
	if req.DB {
		if mysql, err := r.databaseServer.GetByName("local_mysql"); err == nil {
			_ = r.databaseUser.DeleteByNames(mysql.ID, []string{website.Name})
			_ = r.database.Delete(mysql.ID, website.Name)
		}
		if postgres, err := r.databaseServer.GetByName("local_postgresql"); err == nil {
			_ = r.databaseUser.DeleteByNames(postgres.ID, []string{website.Name})
			_ = r.database.Delete(postgres.ID, website.Name)
		}
	}

	if err := r.db.Delete(website).Error; err != nil {
		return err
	}

	if err := systemctl.Reload("nginx"); err != nil {
		_, err = shell.Execf("nginx -t")
		return err
	}

	return nil
}

func (r *websiteRepo) ClearLog(id uint) error {
	website := new(biz.Website)
	if err := r.db.Where("id", id).First(website).Error; err != nil {
		return err
	}

	_, err := shell.Execf(`cat /dev/null > %s/wwwlogs/%s.log`, app.Root, website.Name)
	return err
}

func (r *websiteRepo) UpdateRemark(id uint, remark string) error {
	website := new(biz.Website)
	if err := r.db.Where("id", id).First(website).Error; err != nil {
		return err
	}

	website.Remark = remark
	return r.db.Save(website).Error
}

func (r *websiteRepo) ResetConfig(id uint) error {
	website := new(biz.Website)
	if err := r.db.Where("id", id).First(&website).Error; err != nil {
		return err
	}

	// 初始化nginx配置
	config := nginx.DefaultConf
	if app.Locale == "zh_CN" {
		config = nginx.DefaultConfZh
	}
	p, err := nginx.NewParser(config)
	if err != nil {
		return err
	}
	// 运行目录
	if err = p.SetRoot(website.Path); err != nil {
		return err
	}
	// 伪静态
	includes, comments, err := p.GetIncludes()
	if err != nil {
		return err
	}
	includes = append(includes, filepath.Join(app.Root, "server/vhost/rewrite", website.Name+".conf"))
	includes = append(includes, filepath.Join(app.Root, "server/vhost/acme", website.Name+".conf"))
	comments = append(comments, []string{r.t.Get("# Rewrite rule")})
	comments = append(comments, []string{"# acme http-01"})
	if err = p.SetIncludes(includes, comments); err != nil {
		return err
	}
	// 日志
	if err = p.SetAccessLog(filepath.Join(app.Root, "wwwlogs", website.Name+".log")); err != nil {
		return err
	}
	if err = p.SetErrorLog(filepath.Join(app.Root, "wwwlogs", website.Name+".error.log")); err != nil {
		return err
	}

	if err = io.Write(filepath.Join(app.Root, "server/vhost", website.Name+".conf"), p.Dump(), 0644); err != nil {
		return nil
	}
	if err = io.Write(filepath.Join(app.Root, "server/vhost/rewrite", website.Name+".conf"), "", 0644); err != nil {
		return nil
	}
	if err = io.Write(filepath.Join(app.Root, "server/vhost/acme", website.Name+".conf"), "", 0644); err != nil {
		return err
	}

	website.Status = true
	website.Https = false
	if err = r.db.Save(website).Error; err != nil {
		return err
	}

	if err = systemctl.Reload("nginx"); err != nil {
		_, err = shell.Execf("nginx -t")
		return err
	}

	return nil
}

func (r *websiteRepo) UpdateStatus(id uint, status bool) error {
	website := new(biz.Website)
	if err := r.db.Where("id", id).First(&website).Error; err != nil {
		return err
	}

	// 解析nginx配置
	config, err := io.Read(filepath.Join(app.Root, "server/vhost", website.Name+".conf"))
	if err != nil {
		return err
	}
	p, err := nginx.NewParser(config)
	if err != nil {
		return err
	}

	// 取运行目录和默认文档
	root, rootComment, err := p.GetRootWithComment()
	if err != nil {
		return err
	}
	index, indexComment, err := p.GetIndexWithComment()
	if err != nil {
		return err
	}
	indexStr := strings.Join(index, " ")

	if status {
		if len(rootComment) == 0 {
			return errors.New(r.t.Get("runtime directory comment not found"))
		}
		if len(rootComment) != 1 {
			return errors.New(r.t.Get("runtime directory comment count is incorrect, expected 1, actual %d", len(rootComment)))
		}
		rootComment[0] = strings.TrimPrefix(rootComment[0], "# ")
		if !io.Exists(rootComment[0]) {
			return errors.New(r.t.Get("runtime directory does not exist"))
		}
		if err = p.SetRoot(rootComment[0]); err != nil {
			return err
		}
		if len(indexComment) == 0 {
			return errors.New(r.t.Get("default document comment not found"))
		}
		if len(indexComment) != 1 {
			return errors.New(r.t.Get("default document comment count is incorrect, expected 1, actual %d", len(indexComment)))
		}
		indexComment[0] = strings.TrimPrefix(indexComment[0], "# ")
		if err = p.SetIndex(strings.Fields(indexComment[0])); err != nil {
			return err
		}
	} else {
		if err = p.SetRootWithComment(filepath.Join(app.Root, "server/nginx/html"), []string{"# " + root}); err != nil {
			return err
		}
		if err = p.SetIndexWithComment([]string{"stop.html"}, []string{"# " + indexStr}); err != nil {
			return err
		}
	}

	if err = io.Write(filepath.Join(app.Root, "server/vhost", website.Name+".conf"), p.Dump(), 0644); err != nil {
		return err
	}

	website.Status = status
	if err = r.db.Save(website).Error; err != nil {
		return err
	}

	if err = systemctl.Reload("nginx"); err != nil {
		_, err = shell.Execf("nginx -t")
		return err
	}

	return nil
}

func (r *websiteRepo) UpdateCert(req *request.WebsiteUpdateCert) error {
	website := new(biz.Website)
	if err := r.db.Where("name", req.Name).First(&website).Error; err != nil {
		return err
	}

	if _, err := cert.ParseCert(req.Cert); err != nil {
		return errors.New(r.t.Get("failed to parse certificate: %v", err))
	}
	if _, err := cert.ParseKey(req.Key); err != nil {
		return errors.New(r.t.Get("failed to parse private key: %v", err))
	}

	certPath := filepath.Join(app.Root, "server/vhost/cert", website.Name+".pem")
	keyPath := filepath.Join(app.Root, "server/vhost/cert", website.Name+".key")
	if err := io.Write(certPath, req.Cert, 0644); err != nil {
		return err
	}
	if err := io.Write(keyPath, req.Key, 0644); err != nil {
		return err
	}

	if website.Https {
		if err := systemctl.Reload("nginx"); err != nil {
			_, err = shell.Execf("nginx -t")
			return err
		}
	}

	return nil
}

func (r *websiteRepo) ObtainCert(ctx context.Context, id uint) error {
	website, err := r.Get(id)
	if err != nil {
		return err
	}
	if slices.Contains(website.Domains, "*") {
		return errors.New(r.t.Get("not support one-key obtain wildcard certificate, please use Cert menu to obtain it with DNS method"))
	}

	account, err := r.certAccount.GetDefault(cast.ToUint(ctx.Value("user_id")))
	if err != nil {
		return err
	}

	newCert, err := r.cert.GetByWebsite(website.ID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			newCert, err = r.cert.Create(&request.CertCreate{
				Type:      string(acme.KeyEC256),
				Domains:   website.Domains,
				AutoRenew: true,
				AccountID: account.ID,
				WebsiteID: website.ID,
			})
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}
	newCert.Domains = website.Domains
	if err = r.db.Save(newCert).Error; err != nil {
		return err
	}

	_, err = r.cert.ObtainAuto(newCert.ID)
	if err != nil {
		return err
	}

	return r.cert.Deploy(newCert.ID, website.ID)
}
