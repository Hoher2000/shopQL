package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Hoher2000/shopQL/utils"
	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	"github.com/thanhpk/randstr"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"golang.org/x/crypto/argon2"
)

const (
	dbName       = "shop"
	userCollName = "users"
)

var (
	errUserExist = errors.New("user with same login|email already exist")
	errDB        = errors.New("internal database error")
	errBadToken  = errors.New("invalid token or claims")
)

type User struct {
	ID       bson.ObjectID `json:"id" bson:"_id"`
	Name     string        `json:"username" bson:"username, unique" validate:"required,min=4"`
	Email    string        `json:"email" bson:"email, unique" validate:"required,email"`
	Password string        `json:"password" bson:"password" validate:"required,min=4"`
	Version  int           `json:"version" bson:"version"`
}

type CustomClaims struct {
	*jwt.RegisteredClaims
	UserVersion int
}

type UserRepo struct {
	*mongo.Client
	Secret []byte
	cache  map[bson.ObjectID]struct {
		createdAT time.Time
		userVer   int
	}
	mu sync.RWMutex
}

func NewUserRepo(cl *mongo.Client, secret string) *UserRepo {
	r := &UserRepo{
		cl,
		[]byte(secret),
		map[bson.ObjectID]struct {
			createdAT time.Time
			userVer   int
		}{},
		sync.RWMutex{},
	}
	go r.initCaching(5)
	return r
}

func (u *UserRepo) initCaching(d int) {
	ticker := time.NewTicker(time.Duration(d) * time.Second)
	for range ticker.C {
		u.mu.Lock()
		for id, data := range u.cache {
			if time.Since(data.createdAT) > 5*time.Minute {
				delete(u.cache, id)
			}
		}
		u.mu.Unlock()
	}
}

func (u *UserRepo) Coll() *mongo.Collection {
	return u.Database(dbName).Collection(userCollName)
}

func (u *UserRepo) GetUserName(ctx context.Context) (string, error) {
	userID, err := utils.GetUserObjectIDFromCtx(ctx)
	if err != nil {
		log.Printf("ALERT: %v - fetching User ID from context: %v\n", utils.GetFuncName(1), err)
		return "", err
	}
	filter := bson.M{"_id": userID}
	var res struct {
		Name string `bson:"username"`
	}
	opts := options.FindOne().SetProjection(bson.M{"username": 1})
	if err := u.Coll().FindOne(ctx, filter, opts).Decode(&res); err != nil {
		log.Printf("ERROR: %v - finding in mongo: %v\n", utils.GetFuncName(1), err)
		return "", errDB
	}
	log.Printf("SUCCESS: %v. User ID - %v, username - %v\n", utils.GetFuncName(1), userID.Hex(), res.Name)
	return res.Name, nil
}

func (u *UserRepo) hashPass(plainPassword, salt string) []byte {
	hashedPass := argon2.IDKey([]byte(plainPassword), []byte(salt), 1, 64*1024, 4, 32)
	res := make([]byte, len(salt))
	copy(res, salt[:])
	return append(res, hashedPass...)
}

func (u *UserRepo) Create(ctx context.Context, user *User) (*User, error) {
	//cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	//defer cancel()
	filter := bson.M{
		"$or": []bson.M{
			{"username": user.Name},
			{"email": user.Email},
		},
	}
	count, err := u.Coll().CountDocuments(ctx, filter)
	if err != nil {
		log.Printf("ERROR: %v - finding in mongo: %v\n", utils.GetFuncName(1), err)
		return nil, errDB
	}
	if count > 0 {
		log.Printf("ALERT: %v - user with same credentials: %v, %v is already exist.\n", utils.GetFuncName(1), user.Name, user.Email)
		return nil, errUserExist
	}
	user.Password = string(u.hashPass(user.Password, randstr.String(10)))
	//user.Cart = &custom.Cart{CartItems: make([]*custom.OrderItem, 0)}
	user.ID = bson.NewObjectID()
	_, err = u.Coll().InsertOne(ctx, user)
	if err != nil {
		log.Printf("ERROR: %v - inserting in mongo: %v\n", utils.GetFuncName(1), err)
		return nil, errDB
	}
	log.Printf("SUCCESS: %v. User is created, ID - %v, username - %v\n", utils.GetFuncName(1), user.ID.Hex(), user.Name)
	return user, nil
}

func (u *UserRepo) CheckJWT(ctx context.Context, tokenString string) (context.Context, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			log.Printf("ALERT: %v - bad signing method - %v\n", utils.GetFuncName(1), token.Header["alg"])
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return u.Secret, nil
	})
	if err != nil {
		log.Printf("ERROR: %v - failed to parse token: %v\n", utils.GetFuncName(1), err)
		return nil, errBadToken
	}
	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		if issuer := claims.Issuer; issuer != "GQLShop" {
			log.Printf("ALERT: %v - bad issuer in claims - %v\n", utils.GetFuncName(1), issuer)
			return nil, errBadToken
		}
		var userIDHex string
		if userIDHex = claims.ID; userIDHex == "" {
			log.Printf("ALERT: %v - empty ID in claims\n", utils.GetFuncName(1))
			return nil, errBadToken
		}
		userID, err := bson.ObjectIDFromHex(userIDHex)
		if err != nil {
			log.Printf("ALERT: %v - hex string is not valid ObjectId - %v\n", utils.GetFuncName(1), err)
			return nil, errBadToken
		}
		var res struct {
			Version int `bson:"version"`
		}
		u.mu.RLock()
		if data, ok := u.cache[userID]; ok {
			log.Printf("INFO: %v - hit cache for user ID - %v\n", utils.GetFuncName(1), userID.Hex())
			res.Version = data.userVer
			u.mu.RUnlock()
		} else {
			log.Printf("INFO: %v - cache missing for user ID - %v. Will be get from mongo.\n", utils.GetFuncName(1), userID.Hex())
			u.mu.RUnlock()
			filter, opts := bson.M{"_id": userID}, options.FindOne().SetProjection(bson.M{"version": 1})
			if err := u.Coll().FindOne(ctx, filter, opts).Decode(&res); err != nil {
				if errors.Is(err, mongo.ErrNoDocuments) {
					log.Printf("ALERT: %v - user with ID %v is not exist in mongo\n", utils.GetFuncName(1), userID.Hex())
					return nil, errBadToken
				}
				log.Printf("ERROR: %v - finding in mongo: %v\n", utils.GetFuncName(1), err)
				return nil, errDB
			}
			u.mu.Lock()
			u.cache[userID] = struct {
				createdAT time.Time
				userVer   int
			}{time.Now(), res.Version}
			u.mu.Unlock()
			log.Printf("INFO: %v - user with ID %v is added to cache.\n", utils.GetFuncName(1), userID.Hex())
		}
		if res.Version != claims.UserVersion {
			log.Printf("ALERT: %v - user with ID %v - bad version: want - %v, got - %v/\n", utils.GetFuncName(1), userID.Hex(), res.Version, claims.UserVersion)
			return nil, errBadToken
		}
		newCtx := context.WithValue(ctx, utils.UserKey, userID.Hex())
		log.Printf("SUCCESS: %v. Token is checked, user ID - %v.\n", utils.GetFuncName(1), userID.Hex())
		return newCtx, nil
	}
	log.Printf("ALERT: %v - bad token - %v\n", utils.GetFuncName(1), tokenString)
	return nil, errBadToken
}

func (u *UserRepo) Reg(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		log.Printf("ALERT: %v - bad HTTP method. Want - POST, get - %v\n", utils.GetFuncName(1), r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var userData struct{ User *User }
	if err := json.NewDecoder(r.Body).Decode(&userData); err != nil {
		log.Printf("ALERT: %v - invalid JSON body - %v\n", utils.GetFuncName(1), err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	validate := validator.New(validator.WithRequiredStructEnabled())
	if err := validate.Struct(userData.User); err != nil {
		valErr := err.(validator.ValidationErrors)
		badFields := make([]string, 0)
		for _, fe := range valErr {
			badFields = append(badFields, fe.StructField())
		}
		log.Printf("ALERT: %v - invalid input data - %v\n", utils.GetFuncName(1), strings.Join(badFields, " "))
		resp := map[string]map[string]any{
			"body": {
				"status":  "fail",
				"message": "invalid input data: " + strings.Join(badFields, " "),
			},
		}
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("ERROR: %v - body encoding error - %v\n", utils.GetFuncName(1), err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		//http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	user, err := u.Create(r.Context(), userData.User)
	if err != nil {
		if errors.Is(err, errUserExist) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	claims := CustomClaims{
		&jwt.RegisteredClaims{
			ID:        user.ID.Hex(),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * 24 * time.Hour)),
			Issuer:    "GQLShop"},
		user.Version,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	ss, err := token.SignedString(u.Secret)
	if err != nil {
		log.Printf("ERROR: %v - signing token error - %v\n", utils.GetFuncName(1), err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Add("Authorization", "Token "+ss)
	w.Header().Set("Content-Type", "application/json")
	resp := map[string]map[string]any{
		"body": {
			"status":  "success",
			"message": "user is registrated successfully",
			"token":   ss,
		},
	}
	if err = json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("ERROR: %v - body encoding error - %v\n", utils.GetFuncName(1), err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("SUCCESS: %v. User is registered, user ID - %v, user name - %v.\n", utils.GetFuncName(1), user.ID.Hex(), user.Name)
}
