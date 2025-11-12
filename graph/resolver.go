package graph

import (
	custom "github.com/Hoher2000/shopQL/customModels"
	"github.com/Hoher2000/shopQL/storage"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	Shop *storage.SchemShop
	Cart []*custom.CartItem
}
