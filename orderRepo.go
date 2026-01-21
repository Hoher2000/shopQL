package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	custom "github.com/Hoher2000/shopQL/customModels"
	"github.com/Hoher2000/shopQL/graph/model"
	"github.com/Hoher2000/shopQL/utils"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	cartCollName         = "carts"
	ordersByUserCollName = "ordersByUser"
	orderCollName        = "orders"
)

type CartItem struct {
	ID       string `json:"ID" bson:"_id"`
	UserID   string `json:"userID" bson:"userID"`
	ItemID   int    `json:"itemID" bson:"itemID"`
	Quantity int    `json:"quantity" bson:"quantity"`
}

type OrderItem struct {
	ID            string `json:"ID" bson:"_id"`
	OrderID       string `json:"orderID" bson:"orderID"`
	ItemID        int    `json:"itemID" bson:"itemID"`
	ItemName      string `json:"itemName" bson:"itemName"`
	PurchasePrice int    `json:"price" bson:"price"`
	Quantity      int    `json:"quantity" bson:"quantity"`
}

type Order struct {
	ID        string    `json:"ID" bson:"_id"`
	UserID    int       `json:"userID" bson:"userID"`
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

func (o *OrderRepo) OrderUserColl() *mongo.Collection {
	return o.Database(dbName).Collection(ordersByUserCollName)
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
		{"$project": bson.M{"itemID": "$itemID", "quantity": "$quantity"}},
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

func (o *OrderRepo) UpdateItemInCart(ctx context.Context, itemID, quantityDelta int) error {
	userID, err := utils.GetUserObjectIDFromCtx(ctx)
	if err != nil {
		log.Printf("ALERT: %v - fetching User ID from context: %v\n", utils.GetFuncName(1), err)
		return err
	}
	filter := bson.M{"user": userID, "itemID": itemID}
	update := bson.M{"$inc": bson.M{"quantity": quantityDelta}}
	if _, err := o.CartColl().UpdateOne(ctx, filter, update); err != nil {
		log.Printf("ERROR: %v - updating in mongo: %v\n", utils.GetFuncName(1), err)
		return errDB
	}
	log.Printf("SUCCESS: %v. User ID - %v, item ID - %v changed to %v.\n", utils.GetFuncName(1), userID.Hex(), itemID, quantityDelta)
	return nil
}

func (o *OrderRepo) AddItemToCart(ctx context.Context, itemID, quantityDelta int) error {
	userID, err := utils.GetUserObjectIDFromCtx(ctx)
	if err != nil {
		log.Printf("ALERT: %v - fetching User ID from context: %v\n", utils.GetFuncName(1), err)
		return err
	}
	insert := bson.M{
		"_id":      bson.NewObjectID(),
		"user":     userID,
		"itemID":   itemID,
		"quantity": quantityDelta,
	}
	if _, err := o.CartColl().InsertOne(ctx, insert); err != nil {
		log.Printf("ERROR: %v - inserting in mongo: %v\n", utils.GetFuncName(1), err)
		return errDB
	}
	log.Printf("SUCCESS: %v. User ID - %v, item ID - %v added %v.\n", utils.GetFuncName(1), userID.Hex(), itemID, quantityDelta)
	return nil
}

func (o *OrderRepo) DeleteItemFromCart(ctx context.Context, itemID int) error {
	userID, err := utils.GetUserObjectIDFromCtx(ctx)
	if err != nil {
		log.Printf("ALERT: %v - fetching User ID from context: %v\n", utils.GetFuncName(1), err)
		return err
	}
	filter := bson.M{"user": userID, "itemID": itemID}
	if _, err := o.CartColl().DeleteOne(ctx, filter); err != nil {
		log.Printf("ERROR: %v - deleting in mongo: %v\n", utils.GetFuncName(1), err)
		return errDB
	}
	log.Printf("SUCCESS: %v. User ID - %v, item ID - %v is deleted from cart.\n", utils.GetFuncName(1), userID.Hex(), itemID)
	return nil
}

func (o *OrderRepo) GetItemCountInCart(ctx context.Context, itemID int) (int, error) {
	userID, err := utils.GetUserObjectIDFromCtx(ctx)
	if err != nil {
		log.Printf("ALERT: %v - fetching User ID from context: %v\n", utils.GetFuncName(1), err)
		return -1, err
	}
	var res struct {
		Quantity int `bson:"quantity"`
	}
	filter := bson.M{"user": userID, "itemID": itemID}
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

func (o *OrderRepo) ClearUserCart(ctx context.Context, userID bson.ObjectID) error {
	filter := bson.M{"user": userID}
	if _, err := o.CartColl().DeleteMany(ctx, filter); err != nil {
		log.Printf("ERROR: %v - deleting in mongo: %v\n", utils.GetFuncName(1), err)
		return errDB
	}
	log.Printf("SUCCESS: %v. User ID - %v, cart is cleared.\n", utils.GetFuncName(1), userID.Hex())
	return nil
}

func (o *OrderRepo) MakeOrder(ctx context.Context, items []*custom.OrderItem) (*custom.Order, error) {
	userID, err := utils.GetUserObjectIDFromCtx(ctx)
	if err != nil {
		log.Printf("ALERT: %v - fetching User ID from context: %v\n", utils.GetFuncName(1), err)
		return nil, err
	}
	objID, now := bson.NewObjectID(), time.Now()

	var sum int
	for i := range items {
		sum += items[i].PurchasePrice * items[i].Quantity
		items[i].OrderID = objID.Hex()
	}
	fmt.Println("total", sum)
	insert := bson.M{
		"_id":       objID,
		"userID":    userID,
		"createdAt": now,
		"sum":       sum,
	}
	if _, err := o.OrderColl().InsertOne(ctx, insert); err != nil {
		log.Printf("ERROR: %v - inserting in mongo: %v\n", utils.GetFuncName(1), err)
		return nil, errDB
	}
	if _, err := o.OrderUserColl().InsertMany(ctx, items); err != nil {
		log.Printf("ERROR: %v - inserting in mongo: %v\n", utils.GetFuncName(1), err)
		return nil, errDB
	}
	if err = o.ClearUserCart(ctx, userID); err != nil {
		return nil, err
	}
	log.Printf("SUCCESS: %v. User ID - %v.\n", utils.GetFuncName(1), userID.Hex())
	return &custom.Order{ID: objID.Hex(), UserID: userID.Hex(), CreatedAt: now, TotalSum: sum}, nil
}

func (o *OrderRepo) GetOrderItems(ctx context.Context, orderID string) ([]*custom.OrderItem, error) {
	var items []*custom.OrderItem
	filter := bson.M{"orderID": orderID}
	opts := options.Find().SetSort(bson.M{"itemID": 1})
	cursor, err := o.OrderUserColl().Find(ctx, filter, opts)
	if err != nil {
		log.Printf("ERROR: %v -  finding in mongo: %v\n", utils.GetFuncName(1), err)
		return nil, errDB
	}
	if err = cursor.All(ctx, &items); err != nil {
		log.Printf("ERROR: %v -  mongo cursor decoding: %v\n", utils.GetFuncName(1), err)
		return nil, errDB
	}
	return items, nil
}

func (o *OrderRepo) Search(ctx context.Context, params *model.SearchParameters) ([]*custom.Order, error) {
	pipeline := mongo.Pipeline{}
	matchFilter := bson.D{}
	logText := strings.Builder{}
	logText.WriteString("INFO: Searching orders ")

	if params.MinPrice != nil || params.MaxPrice != nil {
		priceFilter := bson.D{}
		if params.MinPrice != nil {
			priceFilter = append(priceFilter, bson.E{Key: "$gte", Value: *params.MinPrice})
			logText.WriteString(fmt.Sprintf("minprice %v ", *params.MinPrice))
		}
		if params.MaxPrice != nil {
			priceFilter = append(priceFilter, bson.E{Key: "$lte", Value: *params.MaxPrice})
			logText.WriteString(fmt.Sprintf("maxprice %v ", *params.MaxPrice))
		}
		matchFilter = append(matchFilter, bson.E{Key: "sum", Value: priceFilter})
	}

	if len(matchFilter) > 0 {
		pipeline = append(pipeline, bson.D{{Key: "$match", Value: matchFilter}})
	}

	logText.WriteString(fmt.Sprintf("sort by %v ", params.Sort))
	sortKey := bson.D{}
	switch params.Sort {
	case model.SortByPrice:
		sortKey = append(sortKey, bson.E{Key: "sum", Value: 1})
	case model.SortByName:
		sortKey = append(sortKey, bson.E{Key: "name", Value: 1})
	case model.SortByStockQuantity:
		sortKey = append(sortKey, bson.E{Key: "inStock", Value: 1})
	case model.SortByRating:
		sortKey = append(sortKey, bson.E{Key: "rating", Value: -1})
	case model.SortByCommentsCount:
		sortKey = append(sortKey, bson.E{Key: "commentsCount", Value: -1})
	case model.SortByCreatedAt:
		sortKey = append(sortKey, bson.E{Key: "createdAt", Value: -1})
	}

	pipeline = append(pipeline, bson.D{{Key: "$sort", Value: sortKey}})

	pipeline = append(pipeline, bson.D{{Key: "$skip", Value: *params.Offset}})
	pipeline = append(pipeline, bson.D{{Key: "$limit", Value: *params.Limit}})
	logText.WriteString(fmt.Sprintf("skip-%v, limit-%v\n", *params.Offset, *params.Limit))
	log.Print(logText.String())
	cursor, err := o.OrderColl().Aggregate(ctx, pipeline)
	if err != nil {
		log.Printf("ERROR: %v -  aggregate in mongo: %v\n", utils.GetFuncName(1), err)
		return nil, errDB
	}

	var orders []*struct {
		ID        bson.ObjectID `json:"ID" bson:"_id"`
		UserID    bson.ObjectID `json:"userID" bson:"userID"`
		TotalSum  int           `json:"sum" bson:"sum"`
		CreatedAt time.Time     `bson:"createdAt" json:"createdAt"`
	}
	if err = cursor.All(ctx, &orders); err != nil {
		log.Printf("ERROR: %v -  mongo cursor decoding: %v\n", utils.GetFuncName(1), err)
		return nil, errDB
	}
	res := make([]*custom.Order, len(orders))
	if orders == nil {
		res = []*custom.Order{}
	}
	for i := range orders {
		res[i] = &custom.Order{
			ID:        orders[i].ID.Hex(),
			UserID:    orders[i].UserID.Hex(),
			CreatedAt: orders[i].CreatedAt,
			TotalSum:  orders[i].TotalSum,
		}
	}
	log.Printf("SUCCESS: %v - finded %v items\n", utils.GetFuncName(1), len(orders))
	return res, nil
}
