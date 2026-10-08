package orm

import (
	"fmt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"time"
)

type SqliteConfig struct {
	MaxIdleConns int
	MaxOpenConns int
}

func NewSqlite(sqliteConf *SqliteConfig, logwriter logger.Writer) *gorm.DB {
	// Nextec: modo WAL (leitores não esperam escritor), espera de até 30 s em vez de 5 s quando o banco
	// está ocupado e transações que já começam como escrita (evita "database is locked" sem saída)
	dsn := "file:./data/rustdeskapi.db?_journal_mode=WAL&_busy_timeout=30000&_synchronous=NORMAL&_txlock=immediate"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger: logger.New(
			logwriter, // io writer
			logger.Config{
				SlowThreshold:             time.Second, // Slow SQL threshold
				LogLevel:                  logger.Warn, // Log level
				IgnoreRecordNotFoundError: true,        // Ignore ErrRecordNotFound error for logger
				ParameterizedQueries:      true,        // Don't include params in the SQL log
				Colorful:                  true,
			},
		),
	})
	if err != nil {
		fmt.Println(err)
	}
	sqlDB, err2 := db.DB()
	if err2 != nil {
		fmt.Println(err2)
	}
	// SetMaxIdleConns 设置空闲连接池中连接的最大数量
	sqlDB.SetMaxIdleConns(sqliteConf.MaxIdleConns)

	// SetMaxOpenConns 设置打开数据库连接的最大数量。
	sqlDB.SetMaxOpenConns(sqliteConf.MaxOpenConns)

	return db
}
