package migration

import (
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"

	"github.com/coder-lulu/nb-agent/internal/biz"
)

func init() {
	Migrations = append(Migrations, &gormigrate.Migration{
		ID: "20240812-init",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(
				&biz.Cache{},
				&biz.Cert{},
				&biz.CertDNS{},
				&biz.CertAccount{},
				&biz.Cron{},
				&biz.Monitor{},
				&biz.App{},
				&biz.Setting{},
				&biz.Task{},
				&biz.User{},
				&biz.Website{},
			)
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(
				&biz.Cert{},
				&biz.CertDNS{},
				&biz.CertAccount{},
				&biz.Cron{},
				&biz.Monitor{},
				&biz.App{},
				&biz.Setting{},
				&biz.Task{},
				&biz.User{},
				&biz.Website{},
			)
		},
	})
	Migrations = append(Migrations, &gormigrate.Migration{
		ID: "20241022-ssh",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(
				&biz.SSH{},
			)
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(
				&biz.SSH{},
			)
		},
	})
	Migrations = append(Migrations, &gormigrate.Migration{
		ID: "20241124-database",
		Migrate: func(tx *gorm.DB) error {
			_ = tx.Migrator().DropTable("databases")
			return tx.AutoMigrate(
				&biz.DatabaseServer{},
				&biz.DatabaseUser{},
			)
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropTable(
				&biz.DatabaseServer{},
				&biz.DatabaseUser{},
			)
		},
	})
	Migrations = append(Migrations, &gormigrate.Migration{
		ID: "20250318-cert-script",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(
				&biz.Cache{},
				&biz.Cert{},
				&biz.CertDNS{},
				&biz.CertAccount{},
				&biz.Cron{},
				&biz.Monitor{},
				&biz.App{},
				&biz.Setting{},
				&biz.Task{},
				&biz.User{},
				&biz.Website{},
				&biz.SSH{},
				&biz.DatabaseServer{},
				&biz.DatabaseUser{},
			)
		},
		Rollback: func(tx *gorm.DB) error {
			return tx.Migrator().DropColumn(&biz.Cert{}, "script")
		},
	})
	Migrations = append(Migrations, &gormigrate.Migration{
		ID: "20250514-user-website",
		Migrate: func(tx *gorm.DB) error {
			return tx.AutoMigrate(
				&biz.User{},
				&biz.Website{},
				&biz.UserToken{},
			)
		},
		Rollback: func(tx *gorm.DB) error {
			if err := tx.Migrator().DropColumn(&biz.User{}, "two_fa"); err != nil {
				return err
			}
			if err := tx.Migrator().DropColumn(&biz.Website{}, "type"); err != nil {
				return err
			}
			return tx.Migrator().DropTable(&biz.UserToken{})
		},
	})
}
