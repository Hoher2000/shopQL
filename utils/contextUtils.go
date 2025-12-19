package utils

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"
)

var (
	errUserExist                     = errors.New("user with same login|email already exist")
	errDB                            = errors.New("internal database error")
	errBadToken                      = errors.New("invalid token or claims")
	errInvalidIDInContext            = errors.New("user ID must be string")
	UserKey               ctxUserKey = "userID"
)

type ctxUserKey string

// GetUserIDFromCtx return userID from context in string format
func GetUserStringIDFromCtx(ctx context.Context) (string, error) {
	userID, ok := ctx.Value(UserKey).(string)
	if !ok {
		return "", errInvalidIDInContext
	}
	return userID, nil
}

// GetUserOjectIDFromCtx return userID from context in bson.ObjectID format
func GetUserObjectIDFromCtx(ctx context.Context) (userID bson.ObjectID, err error) {
	var userIDHex string
	if userIDHex, err = GetUserStringIDFromCtx(ctx); err != nil {
		return
	}
	userID, err = bson.ObjectIDFromHex(userIDHex)
	return
}
