package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	custom "github.com/Hoher2000/shopQL/customModels"
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
	errUserExist            = errors.New("user with same login|email already exist")
	errDB                   = errors.New("internal database error")
	errBadToken             = errors.New("invalid token or claims")
	userKey      ctxUserKey = "userID"
)

type ctxUserKey string

type User struct {
	ID       bson.ObjectID `json:"id" bson:"_id"`
	Name     string        `json:"username" bson:"username, unique" validate:"required,min=4"`
	Email    string        `json:"email" bson:"email, unique" validate:"required,email"`
	Password string        `json:"password" bson:"password" validate:"required,min=4"`
	Version  int           `json:"version" bson:"version"`
	Cart     *custom.Cart  `json:"cart" bson:"cart"`
}

type CustomClaims struct {
	*jwt.RegisteredClaims
	UserVersion int
}

type UserRepo struct {
	*mongo.Client
	Secret []byte
}

func NewUserRepo(cl *mongo.Client, secret string) *UserRepo {
	return &UserRepo{cl, []byte(secret)}
}

func (u *UserRepo) Coll() *mongo.Collection {
	return u.Database(dbName).Collection(userCollName)
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
		log.Printf("User creating: checking email && name error - %v\n", err)
		return nil, errDB
	}
	if count > 0 {
		log.Printf("User creating: user with same credentials already exist - %v\n", err)
		return nil, errUserExist
	}
	user.Password = string(u.hashPass(user.Password, randstr.String(10)))
	user.Cart = &custom.Cart{CartItems: make([]*custom.OrderItem, 0)}
	user.ID = bson.NewObjectID()
	_, err = u.Coll().InsertOne(ctx, user)
	if err != nil {
		log.Printf("User creating: user inserting error - %v\n", err)
		return nil, errDB
	}
	log.Printf("User creating: user %v is created, ID - %v\n", user.Name, user.ID)
	return user, nil
}

func (u *UserRepo) CheckJWT(ctx context.Context, tokenString string) (context.Context, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			log.Printf("CheckJWT: bad signing method - %v\n", token.Header["alg"])
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return u.Secret, nil
	})
	if err != nil {
		log.Printf("CheckJWT: failed to parse token - %v\n", err)
		return nil, errBadToken
	}
	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		if issuer := claims.Issuer; issuer != "GQLShop" {
			log.Printf("CheckJWT: bad issuer in claims\n")
			return nil, errBadToken
		}
		var hexID string
		if hexID = claims.ID; hexID == "" {
			log.Printf("CheckJWT: empty user hex ID in claims\n")
			return nil, errBadToken
		}
		bsonID, err := bson.ObjectIDFromHex(hexID)
		if err != nil {
			log.Printf("CheckJWT: hex string is not valid ObjectId - %v\n", err)
			return nil, errBadToken
		}
		var res struct {
			Version int `bson:"version"`
		}
		filter, opts := bson.M{"_id": bsonID}, options.FindOne().SetProjection(bson.M{"version": 1})
		if err := u.Coll().FindOne(ctx, filter, opts).Decode(&res); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				log.Printf("CheckJWT: user is not exist - %v\n", err)
				return nil, errBadToken
			}
			log.Printf("CheckJWT: fetch user error - %v\n", err)
			return nil, errDB
		}
		if res.Version != claims.UserVersion {
			log.Printf("CheckJWT: bad user version in claims\n")
			return nil, errBadToken
		}
		newCtx := context.WithValue(ctx, userKey, bsonID)
		log.Printf("CheckJWT: success, userID - %v\n", bsonID)
		return newCtx, nil
	}
	log.Printf("CheckJWT: invalid token or claims")
	return nil, errors.New("invalid token or claims")
}

func (u *UserRepo) Reg(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		log.Printf("registration - bad method. Want - POST, get - %v\n", r.Method)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var userData struct{ User *User }
	if err := json.NewDecoder(r.Body).Decode(&userData); err != nil {
		log.Printf("registration - invalid JSON body - %v\n", err)
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
		log.Printf("registration - invalid input data - %v\n", strings.Join(badFields, " "))
		resp := map[string]map[string]any{
			"body": {
				"status":  "fail",
				"message": "invalid input data: " + strings.Join(badFields, " "),
			},
		}
		w.WriteHeader(http.StatusBadRequest)
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("registration - body encoding error - %v\n", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		//http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	user, err := u.Create(r.Context(), userData.User)
	if err != nil {
		if errors.Is(err, errUserExist) {
			log.Printf("registration - duplicate username or email - %v\n", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		log.Printf("registration - user creating error - %v\n", err)
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
		log.Printf("registration - signing token error - %v\n", err)
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
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Printf("registration - body encoding error - %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("registration success - %v\n", user.Name)
}

func (u *UserRepo) GetUserCart(ctx context.Context) (*custom.Cart, error) {
	ID, ok := ctx.Value(userKey).(bson.ObjectID)
	if !ok {
		log.Printf("GetUserCart: user Id is not in context\n")
		return nil, errDB
	}
	//cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	//defer cancel()
	var res struct {
		Cart *custom.Cart `bson:"cart"`
	}
	if err := u.Database(dbName).Collection(userCollName).FindOne(
		ctx,
		bson.M{"_id": ID},
		options.FindOne().SetProjection(bson.M{"cart": 1}),
	).Decode(&res); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			log.Printf("GetUserCart: user Id %v is not exist\n", ID)
			return nil, errors.New("user is not exist")
		}
		log.Printf("GetUserCart: finding in db error - %v\n", err)
		return nil, errDB
	}
	log.Printf("GetUserCart: succes fo user %v\n", ID)
	return res.Cart, nil
}

func (u *UserRepo) UpdateUserCart(ctx context.Context, cart *custom.Cart) error {
	ID, ok := ctx.Value(userKey).(bson.ObjectID)
	if !ok {
		log.Printf("UpdateUserCart: user Id is not in context\n")
		return errDB
	}
	//cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	//defer cancel()
	res, err := u.Database(dbName).Collection(userCollName).UpdateByID(
		ctx,
		ID,
		bson.M{"$set": bson.M{"cart": cart}},
	)
	if err != nil {
		log.Printf("UpdateUserCart: finding in db error - %v\n", err)
		return errDB
	}
	if res.MatchedCount == 0 {
		log.Printf("UpdateUserCart: user Id %v is not exist\n", ID)
		return errors.New("user is not exist")
	}
	log.Printf("UpdateUserCart: success fo user ID %v\n", ID)
	return nil
}

func (u *UserRepo) GetItemCountInCart(ctx context.Context, itemID int) (int, error) {
	cart, err := u.GetUserCart(ctx)
	if err != nil {
		log.Printf("GetItemCountInCart: GetUserCart error - %v\n", err)
		return -1, err
	}
	for i := range cart.CartItems {
		if cart.CartItems[i].ItemID == itemID {
			return cart.CartItems[i].Quantity, nil
		}
	}
	log.Printf("GetItemCountInCart: success fo item ID %v\n", itemID)
	return 0, nil
}