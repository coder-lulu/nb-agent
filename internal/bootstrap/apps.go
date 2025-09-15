package bootstrap

import (
	"github.com/coder-lulu/nb-agent/internal/apps/codeserver"
	"github.com/coder-lulu/nb-agent/internal/apps/docker"
	"github.com/coder-lulu/nb-agent/internal/apps/fail2ban"
	"github.com/coder-lulu/nb-agent/internal/apps/frp"
	"github.com/coder-lulu/nb-agent/internal/apps/gitea"
	"github.com/coder-lulu/nb-agent/internal/apps/memcached"
	"github.com/coder-lulu/nb-agent/internal/apps/minio"
	"github.com/coder-lulu/nb-agent/internal/apps/mysql"
	"github.com/coder-lulu/nb-agent/internal/apps/nginx"
	"github.com/coder-lulu/nb-agent/internal/apps/php74"
	"github.com/coder-lulu/nb-agent/internal/apps/php80"
	"github.com/coder-lulu/nb-agent/internal/apps/php81"
	"github.com/coder-lulu/nb-agent/internal/apps/php82"
	"github.com/coder-lulu/nb-agent/internal/apps/php83"
	"github.com/coder-lulu/nb-agent/internal/apps/php84"
	"github.com/coder-lulu/nb-agent/internal/apps/phpmyadmin"
	"github.com/coder-lulu/nb-agent/internal/apps/podman"
	"github.com/coder-lulu/nb-agent/internal/apps/postgresql"
	"github.com/coder-lulu/nb-agent/internal/apps/pureftpd"
	"github.com/coder-lulu/nb-agent/internal/apps/redis"
	"github.com/coder-lulu/nb-agent/internal/apps/rsync"
	"github.com/coder-lulu/nb-agent/internal/apps/s3fs"
	"github.com/coder-lulu/nb-agent/internal/apps/supervisor"
	"github.com/coder-lulu/nb-agent/pkg/apploader"
)

func NewLoader(
	codeserver *codeserver.App,
	docker *docker.App,
	fail2ban *fail2ban.App,
	frp *frp.App,
	gitea *gitea.App,
	memcached *memcached.App,
	minio *minio.App,
	mysql *mysql.App,
	nginx *nginx.App,
	php74 *php74.App,
	php80 *php80.App,
	php81 *php81.App,
	php82 *php82.App,
	php83 *php83.App,
	php84 *php84.App,
	phpmyadmin *phpmyadmin.App,
	podman *podman.App,
	postgresql *postgresql.App,
	pureftpd *pureftpd.App,
	redis *redis.App,
	rsync *rsync.App,
	s3fs *s3fs.App,
	supervisor *supervisor.App,
) *apploader.Loader {
	loader := new(apploader.Loader)
	loader.Add(codeserver, docker, fail2ban, frp, gitea, memcached, minio, mysql, nginx, php74, php80, php81, php82, php83, php84, phpmyadmin, podman, postgresql, pureftpd, redis, rsync, s3fs, supervisor)
	return loader
}
