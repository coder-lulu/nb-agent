package route

import (
	"github.com/leonelquinteros/gotext"
	"github.com/urfave/cli/v3"

	"github.com/coder-lulu/nb-agent/internal/service"
)

type Cli struct {
	t   *gotext.Locale
	cli *service.CliService
}

func NewCli(t *gotext.Locale, cli *service.CliService) *Cli {
	return &Cli{
		t:   t,
		cli: cli,
	}
}

func (route *Cli) Commands() []*cli.Command {
	return []*cli.Command{
		{
			Name:   "restart",
			Usage:  route.t.Get("Restart panel service"),
			Action: route.cli.Restart,
		},
		{
			Name:   "stop",
			Usage:  route.t.Get("Stop panel service"),
			Action: route.cli.Stop,
		},
		{
			Name:   "start",
			Usage:  route.t.Get("Start panel service"),
			Action: route.cli.Start,
		},
		{
			Name:   "update",
			Usage:  route.t.Get("Update panel"),
			Action: route.cli.Update,
		},
		{
			Name:   "sync",
			Usage:  route.t.Get("Sync panel data"),
			Action: route.cli.Sync,
		},
		{
			Name:   "fix",
			Usage:  route.t.Get("Fix panel"),
			Action: route.cli.Fix,
		},
		{
			Name:   "info",
			Usage:  route.t.Get("Output panel basic information and generate new password"),
			Action: route.cli.Info,
		},
		{
			Name:  "user",
			Usage: route.t.Get("Operate panel users"),
			Commands: []*cli.Command{
				{
					Name:   "list",
					Usage:  route.t.Get("List all users"),
					Action: route.cli.UserList,
				},
				{
					Name:   "username",
					Usage:  route.t.Get("Change username"),
					Action: route.cli.UserName,
				},
				{
					Name:   "password",
					Usage:  route.t.Get("Change user password"),
					Action: route.cli.UserPassword,
				},
				{
					Name:   "2fa",
					Usage:  route.t.Get("Change user 2FA"),
					Action: route.cli.UserTwoFA,
				},
			},
		},
		{
			Name:  "https",
			Usage: route.t.Get("Operate panel HTTPS"),
			Commands: []*cli.Command{
				{
					Name:   "on",
					Usage:  route.t.Get("Enable HTTPS"),
					Action: route.cli.HTTPSOn,
				},
				{
					Name:   "off",
					Usage:  route.t.Get("Disable HTTPS"),
					Action: route.cli.HTTPSOff,
				},
				{
					Name:   "generate",
					Usage:  route.t.Get("Generate HTTPS certificate"),
					Action: route.cli.HTTPSGenerate,
				},
			},
		},
		{
			Name:  "entrance",
			Usage: route.t.Get("Operate panel access entrance"),
			Commands: []*cli.Command{
				{
					Name:   "on",
					Usage:  route.t.Get("Enable access entrance"),
					Action: route.cli.EntranceOn,
				},
				{
					Name:   "off",
					Usage:  route.t.Get("Disable access entrance"),
					Action: route.cli.EntranceOff,
				},
			},
		},
		{
			Name:  "bind-domain",
			Usage: route.t.Get("Operate panel domain binding"),
			Commands: []*cli.Command{
				{
					Name:   "off",
					Usage:  route.t.Get("Disable domain binding"),
					Action: route.cli.BindDomainOff,
				},
			},
		},
		{
			Name:  "bind-ip",
			Usage: route.t.Get("Operate panel IP binding"),
			Commands: []*cli.Command{
				{
					Name:   "off",
					Usage:  route.t.Get("Disable IP binding"),
					Action: route.cli.BindIPOff,
				},
			},
		},
		{
			Name:  "bind-ua",
			Usage: route.t.Get("Operate panel UA binding"),
			Commands: []*cli.Command{
				{
					Name:   "off",
					Usage:  route.t.Get("Disable UA binding"),
					Action: route.cli.BindUAOff,
				},
			},
		},
		{
			Name:   "port",
			Usage:  route.t.Get("Change panel port"),
			Action: route.cli.Port,
		},
		{
			Name:  "website",
			Usage: route.t.Get("Website management"),
			Commands: []*cli.Command{
				{
					Name:   "create",
					Usage:  route.t.Get("Create new website"),
					Action: route.cli.WebsiteCreate,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "name",
							Usage:    route.t.Get("Website name"),
							Aliases:  []string{"n"},
							Required: true,
						},
						&cli.StringSliceFlag{
							Name:     "domains",
							Usage:    route.t.Get("List of domains associated with the website"),
							Aliases:  []string{"d"},
							Required: true,
						},
						&cli.StringSliceFlag{
							Name:     "listens",
							Usage:    route.t.Get("List of listening addresses associated with the website"),
							Aliases:  []string{"l"},
							Required: true,
						},
						&cli.StringFlag{
							Name:  "path",
							Usage: route.t.Get("Path where the website is hosted (default path if not filled)"),
						},
						&cli.IntFlag{
							Name:  "php",
							Usage: route.t.Get("PHP version used by the website (not used if not filled)"),
						},
					},
				},
				{
					Name:   "remove",
					Usage:  route.t.Get("Remove website"),
					Action: route.cli.WebsiteRemove,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "name",
							Usage:    route.t.Get("Website name"),
							Aliases:  []string{"n"},
							Required: true,
						},
					},
				},
				{
					Name:   "delete",
					Usage:  route.t.Get("Delete website (including website directory, database with the same name)"),
					Action: route.cli.WebsiteDelete,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "name",
							Usage:    route.t.Get("Website name"),
							Aliases:  []string{"n"},
							Required: true,
						},
					},
				},
				{
					Name:   "write",
					Usage:  route.t.Get("Write website data (use only under guidance)"),
					Hidden: true,
					Action: route.cli.WebsiteWrite,
				},
			},
		},
		{
			Name:  "database",
			Usage: route.t.Get("Database management"),
			Commands: []*cli.Command{
				{
					Name:   "add-server",
					Usage:  route.t.Get("Add database server"),
					Action: route.cli.DatabaseAddServer,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "type",
							Usage:    route.t.Get("Server type"),
							Required: true,
						},
						&cli.StringFlag{
							Name:     "name",
							Usage:    route.t.Get("Server name"),
							Required: true,
						},
						&cli.StringFlag{
							Name:     "host",
							Usage:    route.t.Get("Server address"),
							Required: true,
						},
						&cli.UintFlag{
							Name:     "port",
							Usage:    route.t.Get("Server port"),
							Required: true,
						},
						&cli.StringFlag{
							Name:  "username",
							Usage: route.t.Get("Server username"),
						},
						&cli.StringFlag{
							Name:  "password",
							Usage: route.t.Get("Server password"),
						},
						&cli.StringFlag{
							Name:  "remark",
							Usage: route.t.Get("Server remark"),
						},
					},
				},
				{
					Name:   "delete-server",
					Usage:  route.t.Get("Delete database server"),
					Action: route.cli.DatabaseDeleteServer,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "name",
							Usage:    route.t.Get("Server name"),
							Aliases:  []string{"n"},
							Required: true,
						},
					},
				},
			},
		},
		{
			Name:  "backup",
			Usage: route.t.Get("Data backup"),
			Commands: []*cli.Command{
				{
					Name:   "website",
					Usage:  route.t.Get("Backup website"),
					Action: route.cli.BackupWebsite,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "name",
							Aliases:  []string{"n"},
							Usage:    route.t.Get("Website name"),
							Required: true,
						},
						&cli.StringFlag{
							Name:    "path",
							Aliases: []string{"p"},
							Usage:   route.t.Get("Save directory (default path if not filled)"),
						},
					},
				},
				{
					Name:   "database",
					Usage:  route.t.Get("Backup database"),
					Action: route.cli.BackupDatabase,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "type",
							Aliases:  []string{"t"},
							Usage:    route.t.Get("Database type"),
							Required: true,
						},
						&cli.StringFlag{
							Name:     "name",
							Aliases:  []string{"n"},
							Usage:    route.t.Get("Database name"),
							Required: true,
						},
						&cli.StringFlag{
							Name:    "path",
							Aliases: []string{"p"},
							Usage:   route.t.Get("Save directory (default path if not filled)"),
						},
					},
				},
				{
					Name:   "panel",
					Usage:  route.t.Get("Backup panel"),
					Action: route.cli.BackupPanel,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:    "path",
							Aliases: []string{"p"},
							Usage:   route.t.Get("Save directory (default path if not filled)"),
						},
					},
				},
				{
					Name:   "clear",
					Usage:  route.t.Get("Clear backups"),
					Action: route.cli.BackupClear,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "type",
							Aliases:  []string{"t"},
							Usage:    route.t.Get("Backup type"),
							Required: true,
						},
						&cli.StringFlag{
							Name:     "file",
							Aliases:  []string{"f"},
							Usage:    route.t.Get("Backup file"),
							Required: true,
						},
						&cli.IntFlag{
							Name:     "save",
							Aliases:  []string{"s"},
							Usage:    route.t.Get("Number of backups to keep"),
							Required: true,
						},
						&cli.StringFlag{
							Name:    "path",
							Aliases: []string{"p"},
							Usage:   route.t.Get("Backup directory (default path if not filled)"),
						},
					},
				},
			},
		},
		{
			Name:  "cutoff",
			Usage: route.t.Get("Log rotation"),
			Commands: []*cli.Command{
				{
					Name:   "website",
					Usage:  route.t.Get("Website"),
					Action: route.cli.CutoffWebsite,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "name",
							Aliases:  []string{"n"},
							Usage:    route.t.Get("Website name"),
							Required: true,
						},

						&cli.StringFlag{
							Name:    "path",
							Aliases: []string{"p"},
							Usage:   route.t.Get("Save directory (default path if not filled)"),
						},
					},
				},
				{
					Name:   "clear",
					Usage:  route.t.Get("Clear rotated logs"),
					Action: route.cli.CutoffClear,
					Flags: []cli.Flag{
						&cli.StringFlag{
							Name:     "type",
							Aliases:  []string{"t"},
							Usage:    route.t.Get("Rotation type"),
							Required: true,
						},
						&cli.StringFlag{
							Name:     "file",
							Aliases:  []string{"f"},
							Usage:    route.t.Get("Rotation file"),
							Required: true,
						},
						&cli.IntFlag{
							Name:     "save",
							Aliases:  []string{"s"},
							Usage:    route.t.Get("Number of logs to keep"),
							Required: true,
						},
						&cli.StringFlag{
							Name:    "path",
							Aliases: []string{"p"},
							Usage:   route.t.Get("Rotation directory (default path if not filled)"),
						},
					},
				},
			},
		},
		{
			Name:  "app",
			Usage: route.t.Get("Application management"),
			Commands: []*cli.Command{
				{
					Name:   "install",
					Usage:  route.t.Get("Install application"),
					Action: route.cli.AppInstall,
				},
				{
					Name:   "uninstall",
					Usage:  route.t.Get("Uninstall application"),
					Action: route.cli.AppUnInstall,
				},
				{
					Name:   "update",
					Usage:  route.t.Get("Update application"),
					Action: route.cli.AppUpdate,
				},
				{
					Name:   "write",
					Usage:  route.t.Get("Add panel application mark (use only under guidance)"),
					Hidden: true,
					Action: route.cli.AppWrite,
				},
				{
					Name:   "remove",
					Usage:  route.t.Get("Remove panel application mark (use only under guidance)"),
					Hidden: true,
					Action: route.cli.AppRemove,
				},
			},
		},
		{
			Name:   "setting",
			Usage:  route.t.Get("Setting management"),
			Hidden: true,
			Commands: []*cli.Command{
				{
					Name:   "get",
					Usage:  route.t.Get("Get panel setting (use only under guidance)"),
					Hidden: true,
					Action: route.cli.GetSetting,
				},
				{
					Name:   "write",
					Usage:  route.t.Get("Write panel setting (use only under guidance)"),
					Hidden: true,
					Action: route.cli.WriteSetting,
				},
				{
					Name:   "remove",
					Usage:  route.t.Get("Remove panel setting (use only under guidance)"),
					Hidden: true,
					Action: route.cli.RemoveSetting,
				},
			},
		},
		{
			Name:   "sync-time",
			Usage:  route.t.Get("Sync system time"),
			Action: route.cli.SyncTime,
		},
		{
			Name:   "clear-task",
			Usage:  route.t.Get("Clear panel task queue (use only under guidance)"),
			Hidden: true,
			Action: route.cli.ClearTask,
		},
		{
			Name:   "init",
			Usage:  route.t.Get("Initialize panel (use only under guidance)"),
			Hidden: true,
			Action: route.cli.Init,
		},
	}
}
