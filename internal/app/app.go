package app

import (
	"database/sql"

	"github.com/cryskram/relith/internal/config"
	"github.com/rs/zerolog"
)

type App struct {
	Config *config.Config
	Logger zerolog.Logger
	DB     *sql.DB
}
