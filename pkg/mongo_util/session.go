package mongo_util

import (
	"context"
	"go.mongodb.org/mongo-driver/mongo"
)

func WithSession(database *mongo.Database, ctx context.Context, fn func(sc mongo.SessionContext) error) error {
	return database.Client().UseSession(ctx, func(sc mongo.SessionContext) error {
		return fn(sc)
	})
}

func WithTransaction(database *mongo.Database, ctx context.Context, fn func(sc mongo.SessionContext) error) error {
	err := database.Client().UseSession(ctx, func(sc mongo.SessionContext) error {
		_, err := sc.WithTransaction(ctx, func(sc mongo.SessionContext) (interface{}, error) {
			return nil, fn(sc)
		})
		return err
	})
	return err
}
