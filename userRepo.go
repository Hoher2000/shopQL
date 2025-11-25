package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
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
	dbName          = "shop"
	userCollName    = "users"
	catalogCollName = "catalogs"
	itemsCollName   = "items"
	sellerCollName  = "sellers"
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
	Secret string
}

func NewUserRepo(cl *mongo.Client, secret string) *UserRepo {
	return &UserRepo{cl, secret}
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

func (u *UserRepo) CheckJWT(ctx context.Context, tokenString string) error {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return u.Secret, nil
	})
	if err != nil {
		return fmt.Errorf("failed to parse token: %w", err)
	}
	if claims, ok := token.Claims.(*jwt.RegisteredClaims); ok && token.Valid {
		hexID := claims.ID
		if hexID == "" {
			return errors.New("empty userID")
		}
		bsonID, err := bson.ObjectIDFromHex(hexID)
		if err != nil {
			return fmt.Errorf("hex string is not valid ObjectId - %w", err)
		}
		ctx = context.WithValue(ctx, "userID", bsonID)
	}
	return errors.New("invalid token or claims")
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
	json.NewEncoder(w).Encode(resp)
	log.Printf("registration success - %v\n", user.Name)
}

func (u *UserRepo) GetUserCart(ctx context.Context) (*custom.Cart, error) {
	ID := ctx.Value("userID").(bson.ObjectID)
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	cart := bson.M{"cart": nil}
	err := u.Database(dbName).Collection(userCollName).FindOne(
		cctx,
		bson.M{"_id": ID},
		options.FindOne().SetProjection(bson.M{"cart": nil}),
	).Decode(&cart)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return &custom.Cart{}, nil
		}
		return nil, errors.New("internal DB error")
	}
	return cart["cart"].(*custom.Cart), nil
}

func (u *UserRepo) UpdateUserCart(ctx context.Context, cart *custom.Cart) error {
	ID := ctx.Value("userID").(bson.ObjectID)
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_, err := u.Database(dbName).Collection(userCollName).UpdateByID(
		cctx,
		ID,
		bson.M{"cart": cart},
	)
	if err != nil {
		return errors.New("internal BD error")
	}
	return nil
}

func (u *UserRepo) GetItemCountInCart(ctx context.Context, itemID int) (int, error) {
	cart, err := u.GetUserCart(ctx)
	if err != nil {
		return -1, err
	}
	for i := range cart.CartItems {
		if cart.CartItems[i].ItemID == itemID {
			return cart.CartItems[i].Quantity, nil
		}
	}
	return 0, nil
}
