package main

import (
	"context"
	"errors"
	"log"
	"time"

	custom "github.com/Hoher2000/shopQL/customModels"
	"github.com/Hoher2000/shopQL/utils"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	cartCollName         = "carts"
	ordersByUserCollName = "ordersBy user"
	orderCollName        = "orders"
)

type CartItem struct {
	ID       string `json:"ID" bson:"_id"`
	UserID   string `json:"userID" bson:"userID"`
	ItemID   string `json:"itemID" bson:"itemID"`
	Quantity int    `json:"quantity" bson:"quantity"`
}

type OrderItem struct {
	ID            string `json:"ID" bson:"_id"`
	OrderID       string `json:"orderID" bson:"orderID"`
	ItemID        string `json:"itemID" bson:"itemID"`
	ItemName      string `json:"itemName" bson:"itemName"`
	PurchasePrice int    `json:"price" bson:"price"`
	Quantity      int    `json:"quantity" bson:"quantity"`
}

type Order struct {
	ID        string    `json:"ID" bson:"_id"`
	UserID    string    `json:"userID" bson:"userID"`
	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
}

type OrderRepo struct {
	*mongo.Client
}

func NewOrderRepo(cl *mongo.Client) *OrderRepo {
	return &OrderRepo{cl}
}

func (o *OrderRepo) CartColl() *mongo.Collection {
	return o.Database(dbName).Collection(cartCollName)
}

func (o *OrderRepo) OrderColl() *mongo.Collection {
	return o.Database(dbName).Collection(orderCollName)
}

func (o *OrderRepo) GetUserCart(ctx context.Context) (*custom.Cart, error) {
	userID, err := utils.GetUserObjectIDFromCtx(ctx)
	if err != nil {
		log.Printf("ALERT: %v - fetching User ID from context: %v\n", utils.GetFuncName(1), err)
		return nil, err
	}
	var cart custom.Cart
	pipeline := []bson.M{
		{"$match": bson.M{"user": userID}},
		{"$project": bson.M{"item": "$item", "quantity": "$quantity"}},
	}
	cursor, err := o.CartColl().Aggregate(ctx, pipeline)
	if err != nil {
		log.Printf("ERROR: %v - aggregate in mongo: %v\n", utils.GetFuncName(1), err)
		return nil, errDB
	}
	if err = cursor.All(ctx, &cart.CartItems); err != nil {
		log.Printf("ERROR: %v - mongo cursor decoding: %v\n", utils.GetFuncName(1), err)
		return nil, errDB
	}
	if cart.CartItems == nil {
		cart.CartItems = make([]*custom.CartItem, 0)
	}
	log.Printf("SUCCESS: %v. User ID - %v, cart - %v\n", utils.GetFuncName(1), userID.Hex(), cart)
	return &cart, nil
}

func (o *OrderRepo) UpdateItemInCart(ctx context.Context, itemID string, quantityDelta int) error {
	userID, err := utils.GetUserObjectIDFromCtx(ctx)
	if err != nil {
		log.Printf("ALERT: %v - fetching User ID from context: %v\n", utils.GetFuncName(1), err)
		return err
	}
	filter := bson.M{"user": userID, "item": itemID}
	update := bson.M{"$inc": bson.M{"quantity": quantityDelta}}
	if _, err := o.CartColl().UpdateOne(ctx, filter, update); err != nil {
		log.Printf("ERROR: %v - updating in mongo: %v\n", utils.GetFuncName(1), err)
		return errDB
	}
	log.Printf("SUCCESS: %v. User ID - %v, item ID - %v changed to %v.\n", utils.GetFuncName(1), userID.Hex(), itemID, quantityDelta)
	return nil
}

func (o *OrderRepo) AddItemToCart(ctx context.Context, itemID string, quantityDelta int) error {
	userID, err := utils.GetUserObjectIDFromCtx(ctx)
	if err != nil {
		log.Printf("ALERT: %v - fetching User ID from context: %v\n", utils.GetFuncName(1), err)
		return err
	}
	insert := bson.M{
		"_id":      bson.NewObjectID(),
		"user":     userID,
		"item":     itemID,
		"quantity": quantityDelta,
	}
	if _, err := o.CartColl().InsertOne(ctx, insert); err != nil {
		log.Printf("ERROR: %v - inserting in mongo: %v\n", utils.GetFuncName(1), err)
		return errDB
	}
	log.Printf("SUCCESS: %v. User ID - %v, item ID - %v added %v.\n", utils.GetFuncName(1), userID.Hex(), itemID, quantityDelta)
	return nil
}

func (o *OrderRepo) DeleteItemFromCart(ctx context.Context, itemID string) error {
	userID, err := utils.GetUserObjectIDFromCtx(ctx)
	if err != nil {
		log.Printf("ALERT: %v - fetching User ID from context: %v\n", utils.GetFuncName(1), err)
		return err
	}
	filter := bson.M{"user": userID, "item": itemID}
	if _, err := o.CartColl().DeleteOne(ctx, filter); err != nil {
		log.Printf("ERROR: %v - deleting in mongo: %v\n", utils.GetFuncName(1), err)
		return errDB
	}
	log.Printf("SUCCESS: %v. User ID - %v, item ID - %v is deleted from cart.\n", utils.GetFuncName(1), userID.Hex(), itemID)
	return nil
}

func (o *OrderRepo) GetItemCountInCart(ctx context.Context, itemID string) (int, error) {
	userID, err := utils.GetUserObjectIDFromCtx(ctx)
	if err != nil {
		log.Printf("ALERT: %v - fetching User ID from context: %v\n", utils.GetFuncName(1), err)
		return -1, err
	}
	var res struct {
		Quantity int `bson:"quantity"`
	}
	filter := bson.M{"user": userID, "item": itemID}
	opts := options.FindOne().SetProjection(bson.M{"quantity": 1})
	err = o.CartColl().FindOne(ctx, filter, opts).Decode(&res)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			log.Printf("ALERT: %v - item is not exist in user cart: UserID - %v, ItemID - %v\n", utils.GetFuncName(1), userID, itemID)
			return 0, nil
		}
		log.Printf("ERROR: %v - finding in mongo: %v\n", utils.GetFuncName(1), err)
		return -1, errDB
	}
	log.Printf("SUCCESS: %v. User ID - %v, item ID - %v is %v pcs in cart.\n", utils.GetFuncName(1), userID.Hex(), itemID, res.Quantity)
	return res.Quantity, nil
}
