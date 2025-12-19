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
	GetCatalog(ctx context.Context, catID string) (*custom.Catalog, error)
	GetCatChilds(ctx context.Context, catID string) ([]*custom.Catalog, error)
	GetCatItems(ctx context.Context, catID string, limit *int, offset *int) ([]*custom.Item, error)
	GetSeller(ctx context.Context, selID string) (*custom.Seller, error)
	GetSellerItems(ctx context.Context, selID string, limit *int, offset *int) ([]*custom.Item, error)
	SearchItem(ctx context.Context, params *model.SearchParameters) ([]*custom.Item, error)
	GetItem(ctx context.Context, itemID string) (*custom.Item, error)
	GetItemInStock(ctx context.Context, itemID string) (int, error)
	GetItemPrice(ctx context.Context, ItemID string) (int, error)
	GetItemComments(ctx context.Context, itemID string, limit *int, offset *int) ([]*custom.Comment, error)
	AddItemComment(ctx context.Context, itemID string, text, userName string) (*custom.Comment, error)
	RateItem(ctx context.Context, itemID string, rating int) error
	GetItemRating(ctx context.Context, itemID string) (float64, error)
	UpdateItemRating(ctx context.Context, itemID string, rating float64) (*custom.Item, error)
	RateItemComment(ctx context.Context, itemID string, userName string, rating int) error
	GetCommentRating(ctx context.Context, itemID string, userName string) (float64, error)
	UpdateItemInStock(ctx context.Context, itemID string, quantity int) error
	GetComment(ctx context.Context, itemID string, userName string) (*custom.Comment, error)
}

type Orderer interface {
	GetUserCart(ctx context.Context) (*custom.Cart, error)
	UpdateItemInCart(ctx context.Context, itemID string, quantityDelta int) error
	AddItemToCart(ctx context.Context, itemID string, quantityDelta int) error
	DeleteItemFromCart(ctx context.Context, itemID string) error
	GetItemCountInCart(ctx context.Context, itemID string) (int, error)
}

type Userer interface {
	GetUserName(ctx context.Context) (string, error)
}

type Resolver struct {
	Shoper Shoper
	Order  Orderer
	Userer Userer
}
