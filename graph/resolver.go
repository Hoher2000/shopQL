package graph

import (
	"context"

	custom "github.com/Hoher2000/shopQL/customModels"
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
	GetItem(ctx context.Context, itemID int) (*custom.Item, error)
	GetItemCatalog(ctx context.Context, obj *custom.Item) (*custom.Catalog, error)
	GetItemInStock(ctx context.Context, itemID int) (int, error)
	GetItemPrice(ctx context.Context, ItemID int) (int, error)	
	UpdateItemInStock(ctx context.Context, itemID, quantity int) error
}

type UserInterface interface {
	GetUserCart(ctx context.Context) (*custom.Cart, error)
	UpdateUserCart(ctx context.Context, cart *custom.Cart) error
	GetItemCountInCart(ctx context.Context, itemID int) (int, error)	
}

type Resolver struct {
	Shop Shoper
	User UserInterface
}
