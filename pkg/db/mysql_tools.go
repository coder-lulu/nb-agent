package db

import (
	"fmt"

	"github.com/coder-lulu/nb-agent/pkg/shell"
	"github.com/coder-lulu/nb-agent/pkg/systemctl"
)

// MySQLResetRootPassword 重置 MySQL root密码
func MySQLResetRootPassword(password string) error {
	_ = systemctl.Stop("mysqld")
	if run, err := systemctl.Status("mysqld"); err != nil || run {
		return fmt.Errorf("failed to stop MySQL: %w", err)
	}
	_, _ = shell.Execf(`systemctl set-environment MYSQLD_OPTS="--skip-grant-tables --skip-networking"`)
	if err := systemctl.Start("mysqld"); err != nil {
		return fmt.Errorf("failed to start MySQL in safe mode: %w", err)
	}
	if _, err := shell.Execf(`mysql -uroot -e "FLUSH PRIVILEGES;UPDATE mysql.user SET authentication_string=null WHERE user='root' AND host='localhost';ALTER USER 'root'@'localhost' IDENTIFIED BY '%s';FLUSH PRIVILEGES;"`, password); err != nil {
		return fmt.Errorf("failed to reset MySQL root password: %w", err)
	}
	if err := systemctl.Stop("mysqld"); err != nil {
		return fmt.Errorf("failed to stop MySQL: %w", err)
	}
	_, _ = shell.Execf(`systemctl unset-environment MYSQLD_OPTS`)
	if err := systemctl.Start("mysqld"); err != nil {
		return fmt.Errorf("failed to start MySQL: %w", err)
	}

	return nil
}
