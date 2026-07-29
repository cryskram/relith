package app

import (
	"database/sql"
	"log/slog"

	"github.com/cryskram/relith/internal/config"
)

type App struct {
	Config *config.Config
	Logger *slog.Logger
	DB     *sql.DB
}
