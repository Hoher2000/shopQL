package storage

import (
	"encoding/json"
	"fmt"
	"io"
)

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