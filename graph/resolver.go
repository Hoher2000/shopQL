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
	GetCatChilds(ctx context.Context, obj *custom.Catalog) ([]*custom.Catalog, error)
	GetCatParent(ctx context.Context, obj *custom.Catalog) (*custom.Catalog, error)
	GetCatItems(ctx context.Context, obj *custom.Catalog, limit *int, offset *int) ([]*custom.Item, error)
	GetItemSeller(ctx context.Context, obj *custom.Item) (*custom.Seller, error)
	GetItemCatalog(ctx context.Context, obj *custom.Item) (*custom.Catalog, error)
	GetOrderItem(ctx context.Context, obj *custom.OrderItem) (*custom.Item, error)
	GetCatalog(ctx context.Context, id string) (*custom.Catalog, error)
	GetSeller(ctx context.Context, id string) (*custom.Seller, error)
}

type UserInterface interface {
	AddToCart(ctx context.Context, in model.CartItemInput) (*custom.Cart, error)
	RemoveFromCart(ctx context.Context, in model.CartItemInput) (*custom.Cart, error)
	GetItemInCart(ctx context.Context, obj *custom.Item) (int, error)
	GetCartCost(ctx context.Context, obj *custom.Cart) (int, error)
	MyCart(ctx context.Context) (*custom.Cart, error)
}

type Resolver struct {
	Shop Shoper
	User UserInterface
}
