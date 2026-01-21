package graph

import (
	"context"

	custom "github.com/Hoher2000/shopQL/customModels"
	"github.com/Hoher2000/shopQL/graph/model"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Shoper interface {
	ParseShop(string) error
	GetCatalog(ctx context.Context, catID int) (*custom.Catalog, error)
	GetCatChilds(ctx context.Context, catID int) ([]*custom.Catalog, error)
	GetCatItems(ctx context.Context, catID int, limit *int, offset *int) ([]*custom.Item, error)
	GetSeller(ctx context.Context, selID int) (*custom.Seller, error)
	GetSellerItems(ctx context.Context, selID int, limit *int, offset *int) ([]*custom.Item, error)
	SearchItem(ctx context.Context, params *model.SearchParameters) ([]*custom.Item, error)
	GetItem(ctx context.Context, itemID int) (*custom.Item, error)
	GetItemInStock(ctx context.Context, itemID int) (int, error)
	GetItemPrice(ctx context.Context, ItemID int) (int, error)
	GetItemComments(ctx context.Context, itemID int, limit *int, offset *int) ([]*custom.Comment, error)
	GetOrderItemsFromCart(ctx context.Context, cart *custom.Cart) ([]*custom.OrderItem, error)
	AddItemComment(ctx context.Context, itemID int, text, userName string) (*custom.Comment, error)
	RateItem(ctx context.Context, itemID, rating int) error
	GetItemRating(ctx context.Context, itemID int) (float64, error)
	UpdateItemRating(ctx context.Context, itemID int, rating float64) (*custom.Item, error)
	RateItemComment(ctx context.Context, itemID int, userName string, rating int) error
	GetCommentRating(ctx context.Context, itemID int, userName string) (float64, error)
	UpdateItemInStock(ctx context.Context, itemID int, quantity int) error
	GetComment(ctx context.Context, itemID int, userName string) (*custom.Comment, error)
}

type Orderer interface {
	GetUserCart(ctx context.Context) (*custom.Cart, error)
	UpdateItemInCart(ctx context.Context, itemID int, quantityDelta int) error
	AddItemToCart(ctx context.Context, itemID int, quantityDelta int) error
	DeleteItemFromCart(ctx context.Context, itemID int) error
	GetItemCountInCart(ctx context.Context, itemID int) (int, error)
	MakeOrder(ctx context.Context, items []*custom.OrderItem) (*custom.Order, error)
	GetOrderItems(ctx context.Context, orderID string) ([]*custom.OrderItem, error)
	Search(ctx context.Context, params *model.SearchParameters) ([]*custom.Order, error)
}

type Userer interface {
	GetUserName(ctx context.Context) (string, error)
}

type Resolver struct {
	Shoper Shoper
	Order  Orderer
	Userer Userer
}
