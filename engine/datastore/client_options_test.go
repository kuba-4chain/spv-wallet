package datastore

import (
	"context"
	"os"
	"testing"

	zLogger "github.com/mrz1836/go-logger"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDefaultClientOptions will test the method defaultClientOptions()
func TestDefaultClientOptions(t *testing.T) {
	t.Run("ensure default values", func(t *testing.T) {
		defaults := defaultClientOptions()
		require.NotNil(t, defaults)
		assert.Equal(t, Empty, defaults.engine)
		assert.False(t, defaults.autoMigrate)
		assert.False(t, defaults.newRelicEnabled)
		assert.NotNil(t, defaults.sqLite)
	})
}

// TestClientOptions_GetTxnCtx will test the method getTxnCtx()
func TestClientOptions_GetTxnCtx(t *testing.T) {
	t.Run("no txn found", func(t *testing.T) {
		defaults := defaultClientOptions()
		require.NotNil(t, defaults)
		defaults.newRelicEnabled = true

		ctx := defaults.getTxnCtx(context.Background())
		require.NotNil(t, ctx)

		txn := newrelic.FromContext(ctx)
		assert.Nil(t, txn)
	})

	t.Run("txn found", func(_ *testing.T) {
		// todo: Need a mock new relic app / txn
	})
}

// TestWithNewRelic will test the method WithNewRelic()
func TestWithNewRelic(t *testing.T) {
	t.Run("get opts", func(t *testing.T) {
		opt := WithNewRelic()
		assert.IsType(t, *new(ClientOps), opt)
	})

	t.Run("apply opts", func(t *testing.T) {
		opts := []ClientOps{WithNewRelic()}
		c, err := NewClient(context.Background(), opts...)
		require.NotNil(t, c)
		require.NoError(t, err)

		assert.True(t, c.IsNewRelicEnabled())
	})

	// Attempt to remove a file created during the test
	t.Cleanup(func() {
		_ = os.Remove("datastore.db")
	})
}

// TestWithDebugging will test the method WithDebugging()
func TestWithDebugging(t *testing.T) {
	t.Run("get opts", func(t *testing.T) {
		opt := WithDebugging()
		assert.IsType(t, *new(ClientOps), opt)
	})

	t.Run("apply opts", func(t *testing.T) {
		opts := []ClientOps{WithDebugging()}
		c, err := NewClient(context.Background(), opts...)
		require.NotNil(t, c)
		require.NoError(t, err)

		assert.True(t, c.IsDebug())
	})

	// Attempt to remove a file created during the test
	t.Cleanup(func() {
		_ = os.Remove("datastore.db")
	})
}

// TestWithAutoMigrate will test the method WithAutoMigrate()
func TestWithAutoMigrate(t *testing.T) {
	t.Run("check type", func(t *testing.T) {
		opt := WithAutoMigrate(nil)
		assert.IsType(t, *new(ClientOps), opt)
	})

	t.Run("test applying nil", func(t *testing.T) {
		options := &clientOptions{}
		opt := WithAutoMigrate(nil)
		opt(options)
		assert.False(t, options.autoMigrate)
		assert.Nil(t, options.migrateModels)
	})

	t.Run("test applying option", func(t *testing.T) {
		options := &clientOptions{}
		testModel2 := struct {
			Field string
		}{Field: "test"}
		opt := WithAutoMigrate(testModel2)
		opt(options)
		assert.True(t, options.autoMigrate)
		assert.Len(t, options.migrateModels, 1)
	})
}

// TestWithSQLite will test the method WithSQLite()
func TestWithSQLite(t *testing.T) {
	t.Run("check type", func(t *testing.T) {
		opt := WithSQLite(nil)
		assert.IsType(t, *new(ClientOps), opt)
	})

	t.Run("test applying nil", func(t *testing.T) {
		options := &clientOptions{}
		opt := WithSQLite(nil)
		opt(options)
		assert.Equal(t, Engine(""), options.engine)
		assert.Nil(t, options.sqLite)
	})

	t.Run("test applying option", func(t *testing.T) {
		options := &clientOptions{}
		config := &SQLiteConfig{
			CommonConfig: CommonConfig{
				Debug:              true,
				MaxIdleConnections: 1,
				MaxOpenConnections: 1,
				TablePrefix:        "test",
			},
			DatabasePath:       "",
			ExistingConnection: nil,
			Shared:             true,
		}
		opt := WithSQLite(config)
		opt(options)
		assert.Equal(t, config, options.sqLite)
		assert.Equal(t, maxIdleConnectionsSQLite, options.sqLite.MaxIdleConnections)
		assert.Equal(t, SQLite, options.engine)
		assert.Equal(t, config.TablePrefix, options.tablePrefix)
		assert.True(t, options.debug)
	})
}

// TestWithSQL will test the method WithSQL()
func TestWithSQL(t *testing.T) {
	t.Run("check type", func(t *testing.T) {
		opt := WithSQL("", nil)
		assert.IsType(t, *new(ClientOps), opt)
	})

	t.Run("test applying empty engine", func(t *testing.T) {
		options := &clientOptions{}
		opt := WithSQL(Empty, nil)
		opt(options)
		assert.Equal(t, Engine(""), options.engine)
		assert.Nil(t, options.sqlConfigs)
	})
}

// TestWithSQLConnection will test the method WithSQLConnection()
func TestWithSQLConnection(t *testing.T) {
	t.Run("check type", func(t *testing.T) {
		opt := WithSQLConnection("", nil, testTablePrefix)
		assert.IsType(t, *new(ClientOps), opt)
	})

	t.Run("test applying empty engine", func(t *testing.T) {
		options := &clientOptions{}
		opt := WithSQLConnection(Empty, nil, testTablePrefix)
		opt(options)
		assert.Equal(t, Engine(""), options.engine)
		assert.Nil(t, options.sqlConfigs)
	})
}

// TestWithLogger will test the method WithLogger()
func TestWithLogger(t *testing.T) {
	t.Run("check type", func(t *testing.T) {
		opt := WithLogger(nil)
		assert.IsType(t, *new(ClientOps), opt)
	})

	t.Run("test applying nil", func(t *testing.T) {
		options := &clientOptions{}
		opt := WithLogger(nil)
		opt(options)
		assert.Nil(t, options.logger)
	})

	t.Run("test applying valid logger", func(t *testing.T) {
		options := &clientOptions{}
		l := zLogger.NewGormLogger(true, 4)
		opt := WithLogger(l)
		opt(options)
		assert.Equal(t, l, options.logger)
	})
}
