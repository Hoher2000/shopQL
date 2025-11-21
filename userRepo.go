package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	custom "github.com/Hoher2000/shopQL/customModels"
	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	"github.com/thanhpk/randstr"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"golang.org/x/crypto/argon2"
)

const (
	dbName          = "shop"
	userCollName    = "users"
	catalogCollName = "catalogs"
	itemsCollName   = "items"
	sellerCollName  = "sellers"
	mySigningKey    = "gpaphquerylanguage"
)

var userExistErr = errors.New("user with same login|email already exist")

type User struct {
	ID       bson.ObjectID `json:"id" bson:"_id"`
	Name     string        `json:"username" bson:"username, unique" validate:"required,min=4"`
	Email    string        `json:"email" bson:"email, unique" validate:"required,email"`
	Password string        `json:"password" bson:"password" validate:"required,min=4"`
	Version  int           `json:"version" bson:"version"`
	Cart     *custom.Cart  `json:"cart" bson:"cart"`
}

type UserRepo struct {
	*mongo.Client
}

func NewUserRepo(cl *mongo.Client) *UserRepo {
	return &UserRepo{cl}
}

func (u *UserRepo) hashPass(plainPassword, salt string) []byte {
	hashedPass := argon2.IDKey([]byte(plainPassword), []byte(salt), 1, 64*1024, 4, 32)
	res := make([]byte, len(salt))
	copy(res, salt[:len(salt)])
	return append(res, hashedPass...)
}

func (u *UserRepo) Create(ctx context.Context, user *User) (*User, error) {
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_, err := u.Database(dbName).Collection(userCollName).Find(
		cctx,
		bson.M{"username": user.Name, "email": user.Email},
	)
	if err == nil {
		return nil, userExistErr
	}
	if err != mongo.ErrNoDocuments {
		return nil, err
	}
	user.Password = string(u.hashPass(user.Password, randstr.String(10)))
	res, err := u.Database(dbName).Collection(userCollName).InsertOne(cctx, user)
	if err != nil {
		return nil, err
	}
	user.ID = res.InsertedID.(bson.ObjectID)
	return user, nil
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
		log.Printf("registration - invalid input data - %v\n", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	user, err := u.Create(r.Context(), userData.User)
	if err != nil {
		if errors.Is(err, userExistErr) {
			log.Printf("registration - duplicate username or email - %v\n", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		log.Printf("registration - user creating error - %v\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	claims := jwt.RegisteredClaims{
		ID:        user.ID.Hex(),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(10 * 24 * time.Hour)),
		Issuer:    "GQLShop",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	ss, err := token.SignedString(mySigningKey)
	if err != nil {
		log.Printf("registration - signing token erroe - %v\n", err)
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
	json.NewEncoder(w).Encode(resp)
	log.Printf("registration success - %v\n", user.Name)
}
