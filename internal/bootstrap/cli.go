package bootstrap

import (
	"strings"

	"github.com/leonelquinteros/gotext"
	"github.com/urfave/cli/v3"

	"github.com/coder-lulu/nb-agent/internal/app"
	"github.com/coder-lulu/nb-agent/internal/route"
)

func NewCli(t *gotext.Locale, cmd *route.Cli) *cli.Command {
	cli.RootCommandHelpTemplate = strings.ReplaceAll(cli.RootCommandHelpTemplate, "NAME", t.Get("NAME"))
	cli.RootCommandHelpTemplate = strings.ReplaceAll(cli.RootCommandHelpTemplate, "USAGE", t.Get("USAGE"))
	cli.RootCommandHelpTemplate = strings.ReplaceAll(cli.RootCommandHelpTemplate, "VERSION", t.Get("VERSION"))
	cli.RootCommandHelpTemplate = strings.ReplaceAll(cli.RootCommandHelpTemplate, "DESCRIPTION", t.Get("DESCRIPTION"))
	cli.RootCommandHelpTemplate = strings.ReplaceAll(cli.RootCommandHelpTemplate, "AUTHOR", t.Get("AUTHOR"))
	cli.RootCommandHelpTemplate = strings.ReplaceAll(cli.RootCommandHelpTemplate, "COMMANDS", t.Get("COMMANDS"))
	cli.RootCommandHelpTemplate = strings.ReplaceAll(cli.RootCommandHelpTemplate, "GLOBAL OPTIONS", t.Get("GLOBAL OPTIONS"))
	cli.RootCommandHelpTemplate = strings.ReplaceAll(cli.RootCommandHelpTemplate, "COPYRIGHT", t.Get("COPYRIGHT"))
	cli.CommandHelpTemplate = strings.ReplaceAll(cli.CommandHelpTemplate, "NAME", t.Get("NAME"))
	cli.CommandHelpTemplate = strings.ReplaceAll(cli.CommandHelpTemplate, "USAGE", t.Get("USAGE"))
	cli.CommandHelpTemplate = strings.ReplaceAll(cli.CommandHelpTemplate, "CATEGORY", t.Get("CATEGORY"))
	cli.CommandHelpTemplate = strings.ReplaceAll(cli.CommandHelpTemplate, "DESCRIPTION", t.Get("DESCRIPTION"))
	cli.CommandHelpTemplate = strings.ReplaceAll(cli.CommandHelpTemplate, "OPTIONS", t.Get("OPTIONS"))
	cli.SubcommandHelpTemplate = strings.ReplaceAll(cli.SubcommandHelpTemplate, "NAME", t.Get("NAME"))
	cli.SubcommandHelpTemplate = strings.ReplaceAll(cli.SubcommandHelpTemplate, "USAGE", t.Get("USAGE"))
	cli.SubcommandHelpTemplate = strings.ReplaceAll(cli.SubcommandHelpTemplate, "DESCRIPTION", t.Get("USAGE"))
	cli.SubcommandHelpTemplate = strings.ReplaceAll(cli.SubcommandHelpTemplate, "COMMANDS", t.Get("COMMANDS"))
	cli.SubcommandHelpTemplate = strings.ReplaceAll(cli.SubcommandHelpTemplate, "OPTIONS", t.Get("OPTIONS"))

	cli.RootCommandHelpTemplate += "\n" + t.Get("Project：https://github.com/coder-lulu/nb-agent")
	cli.RootCommandHelpTemplate += "\n" + t.Get("Technical discussion: Coming soon") + "\n"

	return &cli.Command{
		Name:     "nb-cli",
		Usage:    t.Get("NewBee Agent CLI Tool"),
		Version:  app.Version,
		Commands: cmd.Commands(),
	}
}
