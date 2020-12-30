package pg_test

import (
	"context"
	"os"
	"testing"

	"github.com/kelseyhightower/envconfig"
	"github.com/plgd-dev/cloud/resource-aggregate/cqrs/eventstore/pg"
	"github.com/plgd-dev/cqrs/eventstore/test"
	"github.com/plgd-dev/kit/security/certManager"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
)

func TestEventStore(t *testing.T) {
	var config certManager.Config
	err := envconfig.Process("DIAL", &config)
	assert.NoError(t, err)

	//dialCertManager, err := certManager.NewCertManager(config)
	//require.NoError(t, err)

	//tlsConfig := dialCertManager.GetClientTLSConfig()
	user := os.Getenv("TEST_SQL_USER")
	if user == "" {
		user = "admin"
	}
	var cfg pg.Config
	err = envconfig.Process("", &cfg)
	assert.NoError(t, err)

	ctx := context.Background()
	cfg.User = user

	store, err := pg.NewEventStore(ctx,
		cfg,
		func(f func()) error { go f(); return nil },
		pg.WithMarshaler(bson.Marshal),
		pg.WithUnmarshaler(bson.Unmarshal),
	)
	require.NoError(t, err)
	require.NotNil(t, store)

	defer store.Close(ctx)
	defer func() {
		t.Log("clearing db")
		err := store.Clear(ctx)
		require.NoError(t, err)
	}()

	t.Log("event store with default namespace")
	test.AcceptanceTest(t, ctx, store)
}
