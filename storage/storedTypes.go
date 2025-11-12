package storage

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	custom "github.com/Hoher2000/shopQL/customModels"
)

// todo - add mutex
type SchemShop struct {
	CatalogsMap map[int]*custom.Catalog
	ItemsMap    map[int]*custom.Item
	SellersMap  map[int]*custom.Seller
}

func UnmarshalShop(in io.Reader) (*Shop, error) {
	var r Shop
	if err := json.NewDecoder(in).Decode(&r); err != nil {
		return nil, fmt.Errorf("failed to unmarshal shop: %w", err)
	}
	return &r, nil
}

func (r *Shop) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type Shop struct {
	Catalog Catalog  `json:"catalog"`
	Sellers []Seller `json:"sellers"`
}

type Catalog struct {
	ID     int       `json:"id"`
	Name   string    `json:"name"`
	Childs []Catalog `json:"childs,omitempty"`
	Items  []Item    `json:"items,omitempty"`
}

type Item struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	InStock  int    `json:"in_stock"`
	SellerID int    `json:"seller_id"`
}

type Seller struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Deals int    `json:"deals"`
}

func parseCat(shop *SchemShop, cat Catalog, parentID *int) {
	log.Printf("Parsing catalog %v...\n", cat.Name)
	shop.CatalogsMap[cat.ID] = &custom.Catalog{
		ID:       cat.ID,
		Name:     cat.Name,
		ParentID: parentID,
	}
	if cat.Items != nil {
		shop.CatalogsMap[cat.ID].ItemsID = make([]int, len(cat.Items))
		shop.CatalogsMap[cat.ID].ItemsCount = len(cat.Items)
		for i, item := range cat.Items {
			shop.CatalogsMap[cat.ID].ItemsID[i] = item.ID
			shop.ItemsMap[item.ID] = &custom.Item{
				ID:        item.ID,
				Name:      item.Name,
				InStock:   item.InStock,
				SellerID:  item.SellerID,
				CatalogID: cat.ID,
			}
			shop.SellersMap[item.SellerID].ItemsID = append(shop.SellersMap[item.SellerID].ItemsID, item.ID)
		}
	}
	if cat.Childs != nil {
		shop.CatalogsMap[cat.ID].ChildsID = make([]int, len(cat.Childs))
		for i, c := range cat.Childs {
			shop.CatalogsMap[cat.ID].ChildsID[i] = c.ID
			parseCat(shop, c, &cat.ID)
		}
	}
}

func ParseShop(file string) (*SchemShop, error) {
	log.Printf("Starting parse json file %v with shop data\n", file)
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	shop, err := UnmarshalShop(f)
	if err != nil {
		return nil, err
	}
	schshop := &SchemShop{
		CatalogsMap: map[int]*custom.Catalog{},
		ItemsMap:    map[int]*custom.Item{},
		SellersMap:  map[int]*custom.Seller{},
	}
	for _, sel := range shop.Sellers {
		log.Printf("Parsing seller %v...\n", sel.Name)
		schshop.SellersMap[sel.ID] = &custom.Seller{
			ID:      sel.ID,
			Name:    sel.Name,
			Deals:   sel.Deals,
			ItemsID: make([]int, 0),
		}
	}
	parseCat(schshop, shop.Catalog, nil)
	log.Printf("File %v with shop data is parsed\n", file)
	return schshop, nil
}
