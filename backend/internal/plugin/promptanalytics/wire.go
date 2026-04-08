package promptanalytics

import (
	"database/sql"

	"github.com/google/wire"
)

// ProviderSet provides Wire providers for the promptanalytics plugin.
var ProviderSet = wire.NewSet(
	ProvideConfig,
	NewRepository,
	New,
	NewHandler,
)

// ProvideConfig returns the default configuration for the promptanalytics plugin.
func ProvideConfig() Config {
	return DefaultConfig()
}

// ProvideRepository creates a Repository from sql.DB.
// Wire will automatically use NewRepository with the provided *sql.DB.
func ProvideRepository(db *sql.DB) Repository {
	return NewRepository(db)
}
