package storage

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
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

func (m *MongoDB) GetCatChilds(ctx context.Context, obj *custom.Catalog) ([]*custom.Catalog, error) {
	if obj == nil {
		return nil, errors.New("catalog object cannot be nil")
	}
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	cats := []*custom.Catalog{}
	cursor, err := m.Database(dbName).Collection(catalogCollName).Find(
		cctx,
		bson.M{"parentID": obj.ID},
		options.Find().SetSort(bson.M{"_id": 1}),
	)
	if err != nil {
		return nil, err
	}
	err = cursor.All(cctx, &cats)
	if err != nil {
		return nil, err
	}
	return cats, err
}

func (m *MongoDB) GetCatParent(ctx context.Context, obj *custom.Catalog) (*custom.Catalog, error) {
	if obj == nil {
		return nil, errors.New("catalog object cannot be nil")
	}
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var cat custom.Catalog
	err := m.Database(dbName).Collection(catalogCollName).FindOne(
		cctx,
		bson.M{"_id": obj.ParentID},
	).Decode(&cat)
	if err != nil {
		return nil, err
	}
	return &cat, err
}

func (m *MongoDB) GetCatItems(ctx context.Context, obj *custom.Catalog, limit *int, offset *int) ([]*custom.Item, error) {
	if obj == nil {
		return nil, errors.New("catalog object cannot be nil")
	}
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	items := []*custom.Item{}
	cursor, err := m.Database(dbName).Collection(itemsCollName).Find(
		cctx,
		bson.M{"catalogID": obj.ID},
		options.Find().SetSort(bson.M{"_id": 1}).SetLimit(int64(*limit)).SetSkip(int64(*offset)),
	)
	if err != nil {
		return nil, err
	}
	err = cursor.All(cctx, &items)
	if err != nil {
		return nil, err
	}
	return items, err
}

func (m *MongoDB) GetItemSeller(ctx context.Context, obj *custom.Item) (*custom.Seller, error) {
	if obj == nil {
		return nil, errors.New("item object cannot be nil")
	}
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var sel custom.Seller
	err := m.Database(dbName).Collection(sellerCollName).FindOne(
		cctx,
		bson.M{"_id": obj.SellerID},
	).Decode(&sel)
	if err != nil {
		return nil, err
	}
	return &sel, err
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

func (m *MongoDB) GetOrderItem(ctx context.Context, obj *custom.OrderItem) (*custom.Item, error) {
	if obj == nil {
		return nil, errors.New("item object cannot be nil")
	}
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var it custom.Item
	err := m.Database(dbName).Collection(itemsCollName).FindOne(
		cctx,
		bson.M{"_id": obj.ItemID},
	).Decode(&it)
	if err != nil {
		return nil, err
	}
	return &it, err
}

func (m *MongoDB) GetCatalog(ctx context.Context, id string) (*custom.Catalog, error) {
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return nil, err
	}
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var cat custom.Catalog
	err = m.Database(dbName).Collection(catalogCollName).FindOne(
		cctx,
		bson.M{"_id": idInt},
	).Decode(&cat)
	if err != nil {
		return nil, err
	}
	return &cat, err
}

func (m *MongoDB) GetSeller(ctx context.Context, id string) (*custom.Seller, error) {
	idInt, err := strconv.Atoi(id)
	if err != nil {
		return nil, err
	}
	cctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	var sel custom.Seller
	err = m.Database(dbName).Collection(sellerCollName).FindOne(
		cctx,
		bson.M{"_id": idInt},
	).Decode(&sel)
	if err != nil {
		return nil, err
	}
	return &sel, err
}
