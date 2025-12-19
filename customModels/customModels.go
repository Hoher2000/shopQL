package custom

import (
	"fmt"
	"strings"
	"time"
)

type CartItem struct {
	ItemID   string `json:"itemID" bson:"itemID"`
	Quantity int    `json:"quantity" bson:"quantity"`
}

type Cart struct {
	CartItems []*CartItem `json:"items" bson:"items"`
}

func (c Cart) String() string {
	items := make([]string, len(c.CartItems))
	for i, item := range c.CartItems {
		items[i] = fmt.Sprintf("ID %v: %v pcs", item.ItemID, item.Quantity)
	}
	return strings.Join(items, ", ")
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

type Catalog struct {
	ID       string `json:"id" bson:"_id, unique"`
	Name     string `json:"name" bson:"name"`
	ParentID string `json:"parentID,omitempty" bson:"parentID"`
}

type Item struct {
	ID            string  `json:"id" bson:"_id, unique"`
	Name          string  `json:"name" bson:"name"`
	InStock       int     `json:"inStock" bson:"inStock"`
	InCart        int     `json:"inCart" bson:"inCart"`
	SellerID      string  `json:"sellerID" bson:"sellerID"`
	CatalogID     string  `json:"catalogID" bson:"catalogID"`
	Price         int     `json:"price" bson:"price"`
	Rating        float64 `json:"rating" bson:"rating"`
	CommentsCount int     `json:"commentsCount" bson:"commentsCount"`
}

type Comment struct {
	ID        string    `json:"id" bson:"_id,omitempty"`
	Text      string    `json:"text" bson:"text"`
	UserID    string    `json:"userID" bson:"userID"`
	ItemID    string    `json:"itemID" bson:"itemID"`
	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `bson:"updatedAt" json:"updatedAt"`
}

type Seller struct {
	ID    string `json:"id" bson:"_id, unique"`
	Name  string `json:"name" bson:"name"`
	Deals int    `json:"deals" bson:"deals"`
}
