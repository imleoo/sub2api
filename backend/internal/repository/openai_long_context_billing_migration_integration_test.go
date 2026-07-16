//go:build integration

package repository

import (
	"context"
	"testing"

	dbmigrations "github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestMigration175EnforcesOpenAILongContextBillingWriteInvariant(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()
	migrationSQL, err := dbmigrations.FS.ReadFile("175_default_openai_long_context_billing.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `
DROP TRIGGER IF EXISTS accounts_propagate_openai_long_context_billing_extra ON accounts;
DROP TRIGGER IF EXISTS accounts_enforce_openai_long_context_billing_extra ON accounts;
`)
	require.NoError(t, err)

	var ordinaryID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
INSERT INTO accounts (name, platform, type, extra)
VALUES ('migration-175-ordinary', 'openai', 'apikey', '{}'::jsonb)
RETURNING id
`).Scan(&ordinaryID))

	var malformedLegacyID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
INSERT INTO accounts (name, platform, type, extra)
VALUES ('migration-175-malformed-legacy', 'openai', 'apikey', '{"openai_long_context_billing_enabled":"false"}'::jsonb)
RETURNING id
`).Scan(&malformedLegacyID))

	_, err = tx.ExecContext(ctx, string(migrationSQL))
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migrationSQL))
	require.NoError(t, err)

	var ordinaryEnabled bool
	require.NoError(t, tx.QueryRowContext(ctx, `
SELECT (extra->>'openai_long_context_billing_enabled')::boolean
FROM accounts
WHERE id = $1
`, ordinaryID).Scan(&ordinaryEnabled))
	require.False(t, ordinaryEnabled)

	var malformedLegacyEnabled bool
	require.NoError(t, tx.QueryRowContext(ctx, `
SELECT (extra->>'openai_long_context_billing_enabled')::boolean
FROM accounts
WHERE id = $1
`, malformedLegacyID).Scan(&malformedLegacyEnabled))
	require.False(t, malformedLegacyEnabled)
	_, err = tx.ExecContext(ctx, `
UPDATE accounts
SET extra = extra || '{"migration_175_unrelated_update":true}'::jsonb
WHERE id = $1
`, malformedLegacyID)
	require.NoError(t, err)

	// A writer that clears extra entirely on UPDATE should have the trigger
	// restore the previously-enforced boolean instead of leaving the key unset.
	_, err = tx.ExecContext(ctx, `
UPDATE accounts
SET extra = '{"legacy_writer_replaced_extra":true}'::jsonb
WHERE id = $1
`, ordinaryID)
	require.NoError(t, err)
	require.NoError(t, tx.QueryRowContext(ctx, `
SELECT (extra->>'openai_long_context_billing_enabled')::boolean
FROM accounts
WHERE id = $1
`, ordinaryID).Scan(&ordinaryEnabled))
	require.False(t, ordinaryEnabled)

	require.NoError(t, tx.QueryRowContext(ctx, `
INSERT INTO accounts (name, platform, type, extra)
VALUES ('migration-175-rolling-writer', 'openai', 'apikey', '{}'::jsonb)
RETURNING (extra->>'openai_long_context_billing_enabled')::boolean
`).Scan(&ordinaryEnabled))
	require.False(t, ordinaryEnabled)

	_, err = tx.ExecContext(ctx, `
INSERT INTO accounts (name, platform, type, extra)
VALUES ('migration-175-malformed', 'openai', 'apikey', '{"openai_long_context_billing_enabled":"false"}'::jsonb)
`)
	require.ErrorContains(t, err, "openai_long_context_billing_enabled must be a boolean")
}
