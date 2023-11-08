package mongo_util

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func InsertedIDToObjectId(insertedID interface{}) primitive.ObjectID {
	switch v := insertedID.(type) {
	case primitive.ObjectID:
		return v
	case *primitive.ObjectID:
		return *v
	}
	return primitive.ObjectID{}
}
