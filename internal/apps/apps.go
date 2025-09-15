package apps

import (
	"github.com/google/wire"

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
)

var ProviderSet = wire.NewSet(
	codeserver.NewApp,
	docker.NewApp,
	fail2ban.NewApp,
	frp.NewApp,
	gitea.NewApp,
	memcached.NewApp,
	minio.NewApp,
	mysql.NewApp,
	nginx.NewApp,
	php74.NewApp,
	php80.NewApp,
	php81.NewApp,
	php82.NewApp,
	php83.NewApp,
	php84.NewApp,
	phpmyadmin.NewApp,
	podman.NewApp,
	postgresql.NewApp,
	pureftpd.NewApp,
	redis.NewApp,
	rsync.NewApp,
	s3fs.NewApp,
	supervisor.NewApp,
)
