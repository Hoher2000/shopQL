package storage

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

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

var dbErr = errors.New("internal database error")

type MongoDB struct {
	*mongo.Client
}

func NewMongoDB(client *mongo.Client) *MongoDB {
	return &MongoDB{client}
}

func (m *MongoDB) parseCatMongo(cat Catalog, parentID int, level, catCnt, itemsCnt int) (int, int, error) {
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

	//ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	//defer cancel()
	_, err = m.Database(dbName).Collection(sellerCollName).InsertMany(context.Background(), sellers)
	if err != nil {
		return fmt.Errorf("sellers insertion in mongo error - %w", err)
	}
	log.Printf("%v sellers is parsed into MongoDB\n\n", len(shop.Sellers))
	log.Printf("Starting parse catalogs into MongoDB\n")
	catCnt, itemCnt, err := m.parseCatMongo(shop.Catalog, -1, 1, 0, 0)
	if err != nil {
		return err
	}
	log.Printf("%v catalogs with %v items is successfully parsed into MongoDB\n", catCnt, itemCnt)
	//log.Printf("File %v with shop data is successfully parsed into MongoDB\n\n", jsonFile)
	return nil
}

func (m *MongoDB) GetCatChilds(ctx context.Context, catID int) ([]*custom.Catalog, error) {
	//cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	//defer cancel()
	cats := []*custom.Catalog{}
	cursor, err := m.Database(dbName).Collection(catalogCollName).Find(
		ctx,
		bson.M{"parentID": catID},
		options.Find().SetSort(bson.M{"_id": 1}),
	)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			log.Printf("GetCatChilds: no such catalog in db - %v\n", err)
			return nil, errors.New("catalog is not exist")
		}
		log.Printf("GetCatChilds: find in db error - %v\n", err)
		return nil, dbErr
	}
	if err = cursor.All(ctx, &cats); err != nil {
		log.Printf("GetCatChilds: decoding mongo cursor error - %v\n", err)
		return nil, dbErr
	}
	log.Printf("GetCatChilds: success for catalog %v\n", catID)
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
			log.Printf("GetCatalog: no such catalog in db - %v\n", err)
			return nil, errors.New("catalog is not exist")
		}
		log.Printf("GetCatalog: find in db error - %v\n", err)
		return nil, dbErr
	}
	log.Printf("GetCatalog: success for catalog - %v\n", catID)
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
			log.Printf("GetCatItems: no such catalog in db - %v\n", err)
			return nil, errors.New("catalog is not exist")
		}
		log.Printf("GetCatItems: find in db error - %v\n", err)
		return nil, dbErr
	}
	if err = cursor.All(ctx, &items); err != nil {
		log.Printf("GetCatItems: decoding cursor error - %v\n", err)
		return nil, dbErr
	}
	log.Printf("GetCatItems: success for catalog - %v\n", catID)
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
			log.Printf("GetSeller: no such seller ID %v in db - %v\n", selID, err)
			return nil, errors.New("seller is not exist")
		}
		log.Printf("GetSeller: find in db error - %v\n", err)
		return nil, dbErr
	}
	log.Printf("GetSeller: success for seller - %v\n", selID)
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
			log.Printf("GetSellerItems: no such seller in db - %v\n", err)
			return nil, errors.New("seller is not exist")
		}
		log.Printf("GetSellerItems: find in db error - %v\n", err)
		return nil, dbErr
	}
	if err = cursor.All(ctx, &items); err != nil {
		log.Printf("GetSellerItems: decoding cursor error - %v\n", err)
		return nil, dbErr
	}
	log.Printf("GetSellerItems: success for seller - %v\n", selID)
	return items, nil
}

func (m *MongoDB) GetItemInStock(ctx context.Context, itemID int) (int, error) {
	//cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	//defer cancel()
	res := bson.M{"inStock": 0}
	if err := m.Database(dbName).Collection(itemsCollName).FindOne(
		ctx,
		bson.M{"_id": itemID},
		options.FindOne().SetProjection(bson.M{"inStock": 0}),
	).Decode(&res); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			log.Printf("GetItemInStock: no item in stock - %v\n", itemID)
			return -1, errors.New("no such item in stock")
		}
		log.Printf("GetItemInStock: find in db error - %v\n", err)
		return -1, dbErr
	}
	log.Printf("GetItemInStock: success for item - %v\n", itemID)
	return res["inStock"].(int), nil
}

func (m *MongoDB) UpdateItemInStock(ctx context.Context, itemID, quantity int) error {
	//cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	//defer cancel()
	_, err := m.Database(dbName).Collection(itemsCollName).UpdateByID(
		ctx,
		itemID,
		bson.M{"inStock": quantity},
	)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			log.Printf("GetItemInStock: no item in stock - %v\n", itemID)
			return errors.New("no such item in stock")
		}
		log.Printf("UpdateItemInStock: update db error - %v\n", err)
		return dbErr
	}
	log.Printf("UpdateItemInStock: qauntity for item %v is setted to %v\n", itemID, quantity)
	return nil
}

func (m *MongoDB) GetItemPrice(ctx context.Context, itemID int) (int, error) {
	res := bson.M{"price": 0}
	if err := m.Database(dbName).Collection(itemsCollName).FindOne(
		ctx,
		bson.M{"_id": itemID},
		options.FindOne().SetProjection(bson.M{"price": 0}),
	).Decode(&res); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			log.Printf("GetItemPrice: no such item in catalogs - %v\n", itemID)
			return -1, errors.New("no such item in catalogs")
		}
		log.Printf("GetItemPrice: find in db error - %v\n", err)
		return -1, dbErr
	}
	log.Printf("GetItemPrice: success for item %v\n", itemID)
	return res["price"].(int), nil
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
			log.Printf("GetItem: no such item in catalogs - %v\n", itemID)
			return nil, errors.New("no such item in catalogs")
		}
		log.Printf("GetItem: find in db error - %v\n", err)
		return nil, errors.New("no such item in catalogs")
	}
	log.Printf("GetItem: success for item - %v\n", itemID)
	return &it, nil
}
