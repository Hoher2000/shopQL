package storage

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"
	"time"

	custom "github.com/Hoher2000/shopQL/customModels"
	"github.com/Hoher2000/shopQL/graph/model"
	"github.com/Hoher2000/shopQL/utils"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

var (
	errUserExist = errors.New("user with same login|email already exist")
	errDB        = errors.New("internal database error")
	errBadToken  = errors.New("invalid token or claims")
)

const (
	dbName                 = "shop"
	catalogCollName        = "catalogs"
	itemsCollName          = "items"
	sellerCollName         = "sellers"
	commentsCollName       = "comments"
	itemsRatingCollName    = "itemsRating"
	commentsRatingCollName = "commentsRating"
)

type MongoDB struct {
	*mongo.Client
}

func NewMongoDB(client *mongo.Client) *MongoDB {
	collection := client.Database(dbName).Collection(itemsCollName)

	// Создаём текстовый индекс на поле "name"
	indexModel := mongo.IndexModel{
		Keys: bson.D{{Key: "name", Value: "text"}},
		// Можно добавить несколько полей:
		// Keys: bson.D{{Key: "name", Value: "text"}, {Key: "description", Value: "text"}},
		Options: options.Index().SetName("text_search_index"),
	}
	_, err := collection.Indexes().CreateOne(context.Background(), indexModel)
	if err != nil {
		// Если индекс уже существует - это нормально
		log.Printf("ERROR: %v - Could not create text index for items collection (might already exist): %v\n", utils.GetFuncName(1), err)
	} else {
		log.Printf("INFO: %v - Text index for items collection is created successfully\n", utils.GetFuncName(1))
	}
	return &MongoDB{client}
}

func (m *MongoDB) parseCatMongo(cat Catalog, parentID, level, catCnt, itemsCnt int) (int, int, error) {
	//ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	//defer cancel()
	_, err := m.Database(dbName).Collection(catalogCollName).InsertOne(context.Background(), &custom.Catalog{
		ID:       cat.ID,
		Name:     cat.Name,
		ParentID: parentID,
	})
	catCnt++
	if err != nil {
		return catCnt, itemsCnt, fmt.Errorf("catalog insertion in mongo error - %w", err)
	}
	fmt.Printf(strings.Repeat("\t", level)+"Catalog %v %v\n", cat.ID, cat.Name)
	if cat.Items != nil {
		items := make([]*custom.Item, len(cat.Items))
		for i, item := range cat.Items {
			items[i] = &custom.Item{
				ID:        item.ID,
				Name:      item.Name,
				InStock:   item.InStock,
				SellerID:  item.SellerID,
				CatalogID: cat.ID,
				Price:     item.ID + item.InStock,
			}
			fmt.Printf(strings.Repeat("\t", level)+"---Item{ID: %v, Name: %v, Count: %v, Price: %v}\n", item.ID, item.Name, item.InStock, items[i].Price)
		}
		//ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		//defer cancel()
		_, err := m.Database(dbName).Collection(itemsCollName).InsertMany(context.Background(), items)
		if err != nil {
			return catCnt, itemsCnt, fmt.Errorf("items insertion in mongo error - %w", err)
		}
		itemsCnt += len(cat.Items)
	}
	if cat.Childs != nil {
		for i := range cat.Childs {
			catCnt, itemsCnt, err = m.parseCatMongo(cat.Childs[i], cat.ID, level+1, catCnt, itemsCnt)
			if err != nil {
				return catCnt, itemsCnt, err
			}
		}
	}
	return catCnt, itemsCnt, nil
}

func (m *MongoDB) ParseShop(jsonFile string) error {
	log.Printf("INFO: Starting parse json file %v with shop data into MongoDB\n\n", jsonFile)
	f, err := os.Open(jsonFile)
	if err != nil {
		return fmt.Errorf("failed to open file %v - %w", jsonFile, err)
	}
	defer f.Close()

	shop, err := UnmarshalShop(f)
	if err != nil {
		return fmt.Errorf("failed to unmarshal shop data - %w", err)
	}

	sellers := make([]*custom.Seller, len(shop.Sellers))
	log.Printf("INFO: Starting parse sellers into MongoDB\n")
	for i, sel := range shop.Sellers {
		sellers[i] = &custom.Seller{
			ID:    sel.ID,
			Name:  sel.Name,
			Deals: sel.Deals,
		}
		fmt.Printf("----------Seller {ID:%v, Name:%v, Deals: %v} is parsed into MongoDB\n", sel.ID, sel.Name, sel.Deals)
	}

	//ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	//defer cancel()
	_, err = m.Database(dbName).Collection(sellerCollName).InsertMany(context.Background(), sellers)
	if err != nil {
		return fmt.Errorf("sellers insertion in mongo error - %w", err)
	}
	log.Printf("INFO: %v sellers is parsed into MongoDB\n\n", len(shop.Sellers))
	log.Printf("INFO: Starting parse catalogs into MongoDB\n")
	catCnt, itemCnt, err := m.parseCatMongo(shop.Catalog, 0, 1, 0, 0)
	if err != nil {
		return err
	}
	log.Printf("INFO: %v catalogs with %v items is successfully parsed into MongoDB\n", catCnt, itemCnt)
	//log.Printf("File %v with shop data is successfully parsed into MongoDB\n\n", jsonFile)
	return nil
}

func (m *MongoDB) GetCatChilds(ctx context.Context, catID int) ([]*custom.Catalog, error) {
	//cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	//defer cancel()
	cats := []*custom.Catalog{}
	filter := bson.M{"parentID": catID}
	opts := options.Find().SetSort(bson.M{"_id": 1})
	cursor, err := m.Database(dbName).Collection(catalogCollName).Find(ctx, filter, opts)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			log.Printf("ALERT: %v - no such catalog in db: %v\n", utils.GetFuncName(1), catID)
			return nil, errors.New("catalog is not exist")
		}
		log.Printf("ERROR: %v - finding ID %v in mongo: %v\n", utils.GetFuncName(1), catID, err)
		return nil, errDB
	}
	if err = cursor.All(ctx, &cats); err != nil {
		log.Printf("ERROR: %v - decoding  mongo cursor for ID %v in mongo: %v\n", utils.GetFuncName(1), catID, err)
		return nil, errDB
	}
	log.Printf("SUCCESS: %v. Catalog ID - %v has %v childs.\n", utils.GetFuncName(1), catID, len(cats))
	return cats, nil
}

func (m *MongoDB) GetCatalog(ctx context.Context, catID int) (*custom.Catalog, error) {
	//cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	//defer cancel()
	var cat custom.Catalog
	if err := m.Database(dbName).Collection(catalogCollName).FindOne(
		ctx,
		bson.M{"_id": catID},
	).Decode(&cat); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			log.Printf("ALERT: %v - no such catalog in db: %v\n", utils.GetFuncName(1), catID)
			return nil, errors.New("catalog is not exist")
		}
		log.Printf("ERROR: %v - finding ID %v in mongo: %v\n", utils.GetFuncName(1), catID, err)
		return nil, errDB
	}
	log.Printf("SUCCESS: %v. Catalog ID - %v.\n", utils.GetFuncName(1), catID)
	return &cat, nil
}

func (m *MongoDB) GetCatItems(ctx context.Context, catID int, limit *int, offset *int) ([]*custom.Item, error) {
	//cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	//defer cancel()
	items := []*custom.Item{}
	cursor, err := m.Database(dbName).Collection(itemsCollName).Find(
		ctx,
		bson.M{"catalogID": catID},
		options.Find().SetSort(bson.M{"_id": 1}).SetLimit(int64(*limit)).SetSkip(int64(*offset)),
	)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			log.Printf("ALERT: %v - no such catalog in db: %v\n", utils.GetFuncName(1), catID)
			return nil, errors.New("catalog is not exist")
		}
		log.Printf("ERROR: %v - finding ID %v in mongo: %v\n", utils.GetFuncName(1), catID, err)
		return nil, errDB
	}
	if err = cursor.All(ctx, &items); err != nil {
		log.Printf("ERROR: %v - decoding  mongo cursor for ID %v in mongo: %v\n", utils.GetFuncName(1), catID, err)
		return nil, errDB
	}
	log.Printf("SUCCESS: %v. Catalog ID - %v has %v items.\n", utils.GetFuncName(1), catID, len(items))
	return items, nil
}

func (m *MongoDB) GetSeller(ctx context.Context, selID int) (*custom.Seller, error) {
	//cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	//defer cancel()
	var sel custom.Seller
	if err := m.Database(dbName).Collection(sellerCollName).FindOne(
		ctx,
		bson.M{"_id": selID},
	).Decode(&sel); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			log.Printf("ALERT: %v - no such seller in db: %v\n", utils.GetFuncName(1), selID)
			return nil, errors.New("seller is not exist")
		}
		log.Printf("ERROR: %v - finding ID %v in mongo: %v\n", utils.GetFuncName(1), selID, err)
		return nil, errDB
	}
	log.Printf("SUCCESS: %v. Seller ID - %v.\n", utils.GetFuncName(1), selID)
	return &sel, nil
}
func (m *MongoDB) GetSellerItems(ctx context.Context, selID int, limit *int, offset *int) ([]*custom.Item, error) {
	//cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	//defer cancel()
	items := []*custom.Item{}
	cursor, err := m.Database(dbName).Collection(itemsCollName).Find(
		ctx,
		bson.M{"sellerID": selID},
		options.Find().SetSort(bson.M{"_id": 1}).SetLimit(int64(*limit)).SetSkip(int64(*offset)),
	)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			log.Printf("ALERT: %v - no such seller in db: %v\n", utils.GetFuncName(1), selID)
			return nil, errors.New("seller is not exist")
		}
		log.Printf("ERROR: %v - finding ID %v in mongo: %v\n", utils.GetFuncName(1), selID, err)
		return nil, errDB
	}
	if err = cursor.All(ctx, &items); err != nil {
		log.Printf("ERROR: %v - decoding  mongo cursor for ID %v in mongo: %v\n", utils.GetFuncName(1), selID, err)
		return nil, errDB
	}
	log.Printf("SUCCESS: %v. Seller ID %v has %v items.\n", utils.GetFuncName(1), selID, len(items))
	return items, nil
}

func (m *MongoDB) GetItemInStock(ctx context.Context, itemID int) (int, error) {
	//cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	//defer cancel()
	var res struct {
		InStock int `bson:"inStock"`
	}
	if err := m.Database(dbName).Collection(itemsCollName).FindOne(
		ctx,
		bson.M{"_id": itemID},
		options.FindOne().SetProjection(bson.M{"inStock": 1}),
	).Decode(&res); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			log.Printf("ALERT: %v - no such item in db: %v\n", utils.GetFuncName(1), itemID)
			return -1, errors.New("no such item in stock")
		}
		log.Printf("ERROR: %v - finding ID %v in mongo: %v\n", utils.GetFuncName(1), itemID, err)
		return -1, errDB
	}
	log.Printf("SUCCESS: %v. Item ID %v has %v in stock quantity.\n", utils.GetFuncName(1), itemID, res.InStock)
	return res.InStock, nil
}

func (m *MongoDB) UpdateItemInStock(ctx context.Context, itemID, quantity int) error {
	//cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	//defer cancel()
	res, err := m.Database(dbName).Collection(itemsCollName).UpdateByID(
		ctx,
		itemID,
		bson.M{"$inc": bson.M{"inStock": quantity}},
	)
	if err != nil {
		log.Printf("ERROR: %v - updating ID %v in mongo: %v\n", utils.GetFuncName(1), itemID, err)
		return errDB
	}
	if res.MatchedCount == 0 {
		log.Printf("ALERT: %v - no such item in db: %v\n", utils.GetFuncName(1), itemID)
		return errors.New("no such item in stock")
	}
	log.Printf("SUCCESS: %v. Item ID %v in stock quantity is changed to %v.\n", utils.GetFuncName(1), itemID, quantity)
	return nil
}

func (m *MongoDB) GetItemPrice(ctx context.Context, itemID int) (int, error) {
	var res struct {
		Price int `bson:"price"`
	}
	if err := m.Database(dbName).Collection(itemsCollName).FindOne(
		ctx,
		bson.M{"_id": itemID},
		options.FindOne().SetProjection(bson.M{"price": 1}),
	).Decode(&res); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			log.Printf("ALERT: %v - no such item in db: %v\n", utils.GetFuncName(1), itemID)
			return -1, errors.New("no such item in catalogs")
		}
		log.Printf("ERROR: %v - finding ID %v in mongo: %v\n", utils.GetFuncName(1), itemID, err)
		return -1, errDB
	}
	log.Printf("SUCCESS: %v. Item ID %v cost %v.\n", utils.GetFuncName(1), itemID, res.Price)
	return res.Price, nil
}

func (m *MongoDB) GetItemComments(ctx context.Context, itemID int, limit *int, offset *int) ([]*custom.Comment, error) {
	//cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	//defer cancel()
	comments := []*custom.Comment{}
	cursor, err := m.Database(dbName).Collection(commentsCollName).Find(
		ctx,
		bson.M{"itemID": itemID},
		options.Find().SetSort(bson.M{"createdAt": -1}).SetLimit(int64(*limit)).SetSkip(int64(*offset)),
	)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			log.Printf("ALERT: %v - no such item in db: %v\n", utils.GetFuncName(1), itemID)
			return nil, errors.New("seller is not exist")
		}
		log.Printf("ERROR: %v - finding ID %v in mongo: %v\n", utils.GetFuncName(1), itemID, err)
		return nil, errDB
	}
	if err = cursor.All(ctx, &comments); err != nil {
		log.Printf("ERROR: %v - decoding  mongo cursor for ID %v in mongo: %v\n", utils.GetFuncName(1), itemID, err)
		return nil, errDB
	}
	log.Printf("SUCCESS: %v. Item ID %v has %v comments.\n", utils.GetFuncName(1), itemID, len(comments))
	return comments, nil
}

func (m *MongoDB) AddItemComment(ctx context.Context, itemID int, text, userName string) (*custom.Comment, error) {
	var comment custom.Comment
	filter := bson.M{"user": userName, "itemID": itemID}
	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	now := time.Now()
	update := bson.M{
		"$set": bson.M{
			"text":      text,
			"updatedAt": now,
		},
		"$setOnInsert": bson.M{
			"_id":       bson.NewObjectID().Hex(),
			"user":      userName,
			"itemID":    itemID,
			"createdAt": now,
		},
	}
	err := m.Database(dbName).Collection(commentsCollName).FindOneAndUpdate(ctx, filter, update, opts).Decode(&comment)
	if err != nil {
		log.Printf("ERROR: %v - finding ID %v in mongo: %v\n", utils.GetFuncName(1), itemID, err)
		return nil, errDB
	}
	if comment.UpdatedAt.Equal(comment.CreatedAt) {
		//fmt.Println("!!!!!!!!!!NEW COMMENT!!!!!!!!!!!!")
		go func(itemID int) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("PANIC RECOVER: %v - %v\n", utils.GetFuncName(1), r)
				}
			}()
			bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			update := bson.M{"$inc": bson.M{"commentsCount": 1}}
			_, err := m.Database(dbName).Collection(itemsCollName).UpdateByID(bgCtx, itemID, update)
			if err != nil {
				log.Printf("ERROR: %v - updating ID %v in mongo: %v\n", utils.GetFuncName(1), itemID, err)
			}
			log.Printf("SUCCESS: %v. Item ID %v comments count is incremented.\n", utils.GetFuncName(1), itemID)
		}(itemID)
	}
	log.Printf("SUCCESS: %v. For Item ID %v is added comment with text: %v. User - %v.\n", utils.GetFuncName(1), itemID, text, userName)
	return &comment, nil
}

func (m *MongoDB) GetItem(ctx context.Context, itemID int) (*custom.Item, error) {
	//cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	//defer cancel()
	var it custom.Item
	if err := m.Database(dbName).Collection(itemsCollName).FindOne(
		ctx,
		bson.M{"_id": itemID},
	).Decode(&it); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			log.Printf("ALERT: %v - no such item in db: %v\n", utils.GetFuncName(1), itemID)
			return nil, errors.New("no such item in catalogs")
		}
		log.Printf("ERROR: %v - finding ID %v in mongo: %v\n", utils.GetFuncName(1), itemID, err)
		return nil, errors.New("no such item in catalogs")
	}
	log.Printf("SUCCESS: %v. Item ID %v.\n", utils.GetFuncName(1), itemID)
	return &it, nil
}

func (m *MongoDB) RateItem(ctx context.Context, itemID, rating int) error {
	userID, err := utils.GetUserObjectIDFromCtx(ctx)
	if err != nil {
		log.Printf("ALERT: %v - fetching User ID from context: %v\n", utils.GetFuncName(1), err)
		return err
	}
	filter := bson.M{"itemID": itemID, "userID": userID}
	opts := options.UpdateOne().SetUpsert(true)
	update := bson.M{
		"$set": bson.M{
			"rating":  rating,
			"ratedAt": time.Now(),
		},
		"$setOnInsert": bson.M{
			"createdAt": time.Now(),
		},
	}
	_, err = m.Database(dbName).Collection(itemsRatingCollName).UpdateOne(ctx, filter, update, opts)
	if err != nil {
		log.Printf("ERROR: %v - updating in mongo for itemID %v and userID %v: %v\n", utils.GetFuncName(1), itemID, userID.Hex(), err)
		return errDB
	}
	log.Printf("SUCCESS: %v - for itemID %v is setted rating %v by userID %v.\n", utils.GetFuncName(1), itemID, rating, userID.Hex())
	return nil
}

func (m *MongoDB) GetItemRating(ctx context.Context, itemID int) (float64, error) {
	pipe := []bson.M{
		{"$match": bson.M{"itemID": itemID}},
		{"$group": bson.M{
			"_id":       "$itemID",
			"avgRating": bson.M{"$avg": "$rating"}},
		},
	}
	var res []bson.M
	cursor, err := m.Database(dbName).Collection(itemsRatingCollName).Aggregate(ctx, pipe)
	if err != nil {
		log.Printf("ERROR: %v - finding ID %v in mongo: %v\n", utils.GetFuncName(1), itemID, err)
		return 0, errDB
	}
	if err = cursor.All(ctx, &res); err != nil {
		log.Printf("ERROR: %v - decoding  mongo cursor for ID %v in mongo: %v\n", utils.GetFuncName(1), itemID, err)
		return 0, errDB
	}
	var r float64
	if len(res) > 0 {
		r = res[0]["avgRating"].(float64)
	}
	log.Printf("SUCCESS: %v - for itemID %v rating is %v.\n", utils.GetFuncName(1), itemID, r)
	return r, nil
}

func (m *MongoDB) UpdateItemRating(ctx context.Context, itemID int, rating float64) (*custom.Item, error) {
	filter := bson.M{"_id": itemID}
	options := options.FindOneAndUpdate().SetReturnDocument(options.After)
	update := bson.M{"$set": bson.M{"rating": rating}}
	var item custom.Item
	err := m.Database(dbName).Collection(itemsCollName).FindOneAndUpdate(ctx, filter, update, options).Decode(&item)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			log.Printf("ALERT: %v - no such item in db: %v\n", utils.GetFuncName(1), itemID)
			return nil, errors.New("no such item")
		}
		log.Printf("ERROR: %v - finding ID %v in mongo: %v\n", utils.GetFuncName(1), itemID, err)
		return nil, errDB
	}
	log.Printf("SUCCESS: %v - for itemID %v is updated rating - %v.\n", utils.GetFuncName(1), itemID, rating)
	return &item, nil
}

func (m *MongoDB) RateItemComment(ctx context.Context, itemID int, userName string, rating int) error {
	userID, err := utils.GetUserObjectIDFromCtx(ctx)
	if err != nil {
		log.Printf("ALERT: %v - fetching User ID from context: %v\n", utils.GetFuncName(1), err)
		return err
	}
	filter := bson.M{"itemID": itemID, "userID": userID, "creator": userName}
	opts := options.UpdateOne().SetUpsert(true)
	update := bson.M{
		"$set": bson.M{
			"rating":  rating,
			"ratedAt": time.Now(),
		},
		"$setOnInsert": bson.M{
			"createdAt": time.Now(),
		},
	}
	_, err = m.Database(dbName).Collection(commentsRatingCollName).UpdateOne(ctx, filter, update, opts)
	if err != nil {
		log.Printf("ERROR: %v - updating ID %v in mongo: %v\n", utils.GetFuncName(1), itemID, err)
		return errDB
	}
	log.Printf("SUCCESS: %v - for comment by user %v for itemID %v is setted rating %v.\n", utils.GetFuncName(1), userID.Hex(), itemID, rating)
	return nil
}

func (m *MongoDB) GetCommentRating(ctx context.Context, itemID int, userName string) (float64, error) {
	pipe := []bson.M{
		{"$match": bson.M{"creator": userName, "itemID": itemID}},
		{"$group": bson.M{
			"_id":       "$commentID",
			"avgRating": bson.M{"$avg": "$rating"}},
		},
	}
	var res []bson.M
	cursor, err := m.Database(dbName).Collection(commentsRatingCollName).Aggregate(ctx, pipe)
	if err != nil {
		log.Printf("ERROR: %v - finding ID %v in mongo: %v\n", utils.GetFuncName(1), itemID, err)
		return 0, errDB
	}
	if err = cursor.All(ctx, &res); err != nil {
		log.Printf("ERROR: %v - decoding  mongo cursor for ID %v in mongo: %v\n", utils.GetFuncName(1), itemID, err)
		return 0, errDB
	}
	var r float64
	if len(res) > 0 {
		r = res[0]["avgRating"].(float64)
	}
	log.Printf("SUCCESS: %v - for comment by user %v for itemID %v is setted rating %v.\n", utils.GetFuncName(1), userName, itemID, r)
	return r, nil
}

func (m *MongoDB) GetComment(ctx context.Context, itemID int, userID string) (*custom.Comment, error) {
	var comment *custom.Comment
	err := m.Database(dbName).Collection(commentsCollName).FindOne(
		ctx,
		bson.M{"itemID": itemID, "user": userID},
	).Decode(&comment)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			log.Printf("ALERT: %v - no comment for item %v by user %v in db.\n", utils.GetFuncName(1), itemID, userID)
			return nil, errors.New("comment is not exist")
		}
		log.Printf("ERROR: %v - finding comment for itemID %v by useID %v in mongo: %v\n", utils.GetFuncName(1), itemID, userID, err)
		return nil, errDB
	}
	log.Printf("SUCCESS: %v - comment by user %v for itemID %v, text - %v.\n", utils.GetFuncName(1), userID, itemID, comment.Text)
	return comment, nil
}

func (m *MongoDB) SearchItem(ctx context.Context, params *model.SearchParameters) ([]*custom.Item, error) {
	pipeline := mongo.Pipeline{}
	matchFilter := bson.D{}
	logText := strings.Builder{}
	logText.WriteString("INFO: Searching items ")
	if params.CatalogID != nil {
		catIDString := *params.CatalogID
		var catID int
		if err := utils.Int(&catID, catIDString); err != nil {
			log.Printf("ALERT: %v - %v\n", utils.GetFuncName(1), err)
			return nil, err
		}
		matchFilter = append(matchFilter, bson.E{Key: "catalogID", Value: catID})
		logText.WriteString(fmt.Sprintf("in catalog %v ", catID))
	}
	if params.Keyword != nil && *params.Keyword != "" {
		matchFilter = append(matchFilter, bson.E{Key: "$text", Value: bson.D{{Key: "$search", Value: *params.Keyword}}})
		logText.WriteString(fmt.Sprintf("with keyword %v ", *params.Keyword))
	}

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
		matchFilter = append(matchFilter, bson.E{Key: "price", Value: priceFilter})
	}

	if len(matchFilter) > 0 {
		pipeline = append(pipeline, bson.D{{Key: "$match", Value: matchFilter}})
	}

	logText.WriteString(fmt.Sprintf("sort by %v ", params.Sort))
	sortKey := bson.D{}
	switch params.Sort {
	case model.SortByPrice:
		sortKey = append(sortKey, bson.E{Key: "price", Value: 1})
	case model.SortByName:
		sortKey = append(sortKey, bson.E{Key: "name", Value: 1})
	case model.SortByStockQuantity:
		sortKey = append(sortKey, bson.E{Key: "inStock", Value: 1})
	case model.SortByRating:
		sortKey = append(sortKey, bson.E{Key: "rating", Value: -1})
	case model.SortByCommentsCount:
		sortKey = append(sortKey, bson.E{Key: "commentsCount", Value: -1})
	}

	pipeline = append(pipeline, bson.D{{Key: "$sort", Value: sortKey}})

	pipeline = append(pipeline, bson.D{{Key: "$skip", Value: *params.Offset}})
	pipeline = append(pipeline, bson.D{{Key: "$limit", Value: *params.Limit}})
	logText.WriteString(fmt.Sprintf("skip-%v, limit-%v\n", *params.Offset, *params.Limit))
	log.Print(logText.String())
	cursor, err := m.Database(dbName).Collection(itemsCollName).Aggregate(ctx, pipeline)
	if err != nil {
		log.Printf("ERROR: %v -  aggregate in mongo: %v\n", utils.GetFuncName(1), err)
		return nil, errDB
	}

	var items []*custom.Item
	if err = cursor.All(ctx, &items); err != nil {
		log.Printf("ERROR: %v -  mongo cursor decoding: %v\n", utils.GetFuncName(1), err)
		return nil, errDB
	}
	if items == nil {
		items = []*custom.Item{}
	}
	log.Printf("SUCCESS: %v - finded %v items\n", utils.GetFuncName(1), len(items))
	return items, nil
}

func (m *MongoDB) GetOrderItemsFromCart(ctx context.Context, cart *custom.Cart) ([]*custom.OrderItem, error) {
	slices.SortFunc(cart.CartItems, func(a, b *custom.CartItem) int { return a.ItemID - b.ItemID })
	ids := make([]int, len(cart.CartItems))
	for i := range ids {
		ids[i] = cart.CartItems[i].ItemID
	}
	res := make([]struct {
		ID            int    `bson:"_id"`
		ItemName      string `bson:"name"`
		PurchasePrice int    `bson:"price"`
	}, len(ids))
	filter := bson.M{"_id": bson.M{"$in": ids}}
	opts := options.Find().SetProjection(bson.M{"name": 1, "price": 1, "_id": 1}).SetSort(bson.M{"_id": 1})
	cursor, err := m.Database(dbName).Collection(itemsCollName).Find(ctx, filter, opts)
	if err != nil {
		log.Printf("ERROR: %v -  finding in mongo: %v\n", utils.GetFuncName(1), err)
		return nil, errDB
	}
	if err = cursor.All(ctx, &res); err != nil {
		log.Printf("ERROR: %v -  mongo cursor decoding: %v\n", utils.GetFuncName(1), err)
		return nil, errDB
	}
	items := make([]*custom.OrderItem, len(ids))
	for i := range ids {
		items[i] = &custom.OrderItem{
			ID:            bson.NewObjectID().Hex(),
			ItemID:        res[i].ID,
			Name:          res[i].ItemName,
			PurchasePrice: res[i].PurchasePrice,
			Quantity:      cart.CartItems[i].Quantity,
		}
	}
	return items, nil
}
