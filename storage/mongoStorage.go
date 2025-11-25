package storage

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	custom "github.com/Hoher2000/shopQL/customModels"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	dbName          = "shop"
	catalogCollName = "catalogs"
	itemsCollName   = "items"
	sellerCollName  = "sellers"
)

type MongoDB struct {
	*mongo.Client
}

func NewMongoDB(client *mongo.Client) *MongoDB {
	return &MongoDB{client}
}

func (m *MongoDB) parseCatMongo(cat Catalog, parentID int, level, catCnt, itemsCnt int) (int, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := m.Database(dbName).Collection(catalogCollName).InsertOne(ctx, &custom.Catalog{
		ID:       cat.ID,
		Name:     cat.Name,
		ParentID: parentID,
	})
	catCnt++
	if err != nil {
		return catCnt, itemsCnt, err
	}
	log.Printf(strings.Repeat("\t", level)+"Catalog %v %v\n", cat.ID, cat.Name)
	if cat.Items != nil {
		items := make([]*custom.Item, len(cat.Items))
		for i, item := range cat.Items {
			items[i] = &custom.Item{
				ID:        item.ID,
				Name:      item.Name,
				InStock:   item.InStock,
				SellerID:  item.SellerID,
				CatalogID: cat.ID,
			}
			log.Printf(strings.Repeat("\t", level)+"---Item{ID: %v, Name: %v, Count: %v}\n", item.ID, item.Name, item.InStock)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, err := m.Database(dbName).Collection(itemsCollName).InsertMany(ctx, items)
		itemsCnt += len(cat.Items)
		if err != nil {
			return catCnt, itemsCnt, err
		}
	}
	if cat.Childs != nil {
		for i := range cat.Childs {
			catCnt, itemsCnt, err = m.parseCatMongo(cat.Childs[i], cat.ID, level+1, catCnt, itemsCnt)
			if err != nil {
				return catCnt, itemsCnt, err
			}
		}
	}
	return catCnt, itemsCnt, err
}

func (m *MongoDB) ParseShop(jsonFile string) error {
	log.Printf("Starting parse json file %v with shop data into MongoDB\n\n", jsonFile)
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
	log.Printf("Starting parse sellers into MongoDB\n")
	for i, sel := range shop.Sellers {
		sellers[i] = &custom.Seller{
			ID:    sel.ID,
			Name:  sel.Name,
			Deals: sel.Deals,
		}
		log.Printf("----------Seller {ID:%v, Name:%v, Deals: %v} is parsed into MongoDB\n", sel.ID, sel.Name, sel.Deals)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = m.Database(dbName).Collection(sellerCollName).InsertMany(ctx, sellers)
	if err != nil {
		return err
	}
	log.Printf("%v sellers is parsed into MongoDB\n\n", len(shop.Sellers))
	log.Printf("Starting parse catalogs into MongoDB\n")
	catCnt, itemCnt, err := m.parseCatMongo(shop.Catalog, -1, 1, 0, 0)
	if err != nil {
		return err
	}
	log.Printf("%v catalogs with %v items is successfully parsed into MongoDB\n", catCnt, itemCnt)
	log.Printf("File %v with shop data is successfully parsed into MongoDB\n\n", jsonFile)
	return nil
}

func (m *MongoDB) GetCatChilds(ctx context.Context, catID int) ([]*custom.Catalog, error) {
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	cats := []*custom.Catalog{}
	cursor, err := m.Database(dbName).Collection(catalogCollName).Find(
		cctx,
		bson.M{"parentID": catID},
		options.Find().SetSort(bson.M{"_id": 1}),
	)
	if err != nil {
		return nil, err
	}
	if err = cursor.All(cctx, &cats); err != nil {
		return nil, err
	}
	return cats, err
}

func (m *MongoDB) GetCatalog(ctx context.Context, catID int) (*custom.Catalog, error) {
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var cat custom.Catalog
	if err := m.Database(dbName).Collection(catalogCollName).FindOne(
		cctx,
		bson.M{"_id": catID},
	).Decode(&cat); err != nil {
		return nil, err
	}
	return &cat, nil
}

func (m *MongoDB) GetCatItems(ctx context.Context, catID int, limit *int, offset *int) ([]*custom.Item, error) {
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	items := []*custom.Item{}
	cursor, err := m.Database(dbName).Collection(itemsCollName).Find(
		cctx,
		bson.M{"catalogID": catID},
		options.Find().SetSort(bson.M{"_id": 1}).SetLimit(int64(*limit)).SetSkip(int64(*offset)),
	)
	if err != nil {
		return nil, err
	}
	if err = cursor.All(cctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (m *MongoDB) GetSeller(ctx context.Context, selID int) (*custom.Seller, error) {
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var sel custom.Seller
	if err := m.Database(dbName).Collection(sellerCollName).FindOne(
		cctx,
		bson.M{"_id": sel},
	).Decode(&sel); err != nil {
		return nil, err
	}
	return &sel, nil
}
func (m *MongoDB) GetSellerItems(ctx context.Context, selID int, limit *int, offset *int) ([]*custom.Item, error) {
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	items := []*custom.Item{}
	cursor, err := m.Database(dbName).Collection(itemsCollName).Find(
		cctx,
		bson.M{"sellerID": selID},
		options.Find().SetSort(bson.M{"_id": 1}).SetLimit(int64(*limit)).SetSkip(int64(*offset)),
	)
	if err != nil {
		return nil, err
	}
	if err = cursor.All(cctx, &items); err != nil {
		return nil, err
	}
	return items, nil
}

func (m *MongoDB) GetItemCatalog(ctx context.Context, obj *custom.Item) (*custom.Catalog, error) {
	if obj == nil {
		return nil, errors.New("item object cannot be nil")
	}
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var cat custom.Catalog
	err := m.Database(dbName).Collection(catalogCollName).FindOne(
		cctx,
		bson.M{"_id": obj.CatalogID},
	).Decode(&cat)
	if err != nil {
		return nil, err
	}
	return &cat, err
}

func (m *MongoDB) GetItemInStock(ctx context.Context, ItemID int) (int, error) {
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	res := bson.M{"inStock": 0}
	err := m.Database(dbName).Collection(itemsCollName).FindOne(
		cctx,
		bson.M{"_id": ItemID},
		options.FindOne().SetProjection(bson.M{"inStock": 0}),
	).Decode(&res)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return 0, nil
		}
		return -1, errors.New("internal BD error")
	}
	return res["inStock"].(int), err
}

func (m *MongoDB) UpdateItemInStock(ctx context.Context, itemID, quantity int) error {
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	_, err := m.Database(dbName).Collection(itemsCollName).UpdateByID(
		cctx,
		itemID,
		bson.M{"inStock": quantity},
	)
	if err != nil {
		return errors.New("internal BD error")
	}
	return nil
}

func (m *MongoDB) GetItemPrice(ctx context.Context, ItemID int) (int, error) {
	res := bson.M{"price": 0}
	err := m.Database(dbName).Collection(itemsCollName).FindOne(
		ctx,
		bson.M{"_id": ItemID},
		options.FindOne().SetProjection(bson.M{"price": 0}),
	).Decode(&res)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return 0, nil
		}
		return -1, errors.New("internal BD error")
	}
	return res["price"].(int), err
}

func (m *MongoDB) GetItem(ctx context.Context, itemID int) (*custom.Item, error) {
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var it custom.Item
	err := m.Database(dbName).Collection(itemsCollName).FindOne(
		cctx,
		bson.M{"_id": itemID},
	).Decode(&it)
	if err != nil {
		return nil, err
	}
	return &it, err
}
