package custom

type CartItem struct {
	ItemID   int `json:"itemID"`
	Quantity int `json:"quantity"`
}

type Catalog struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	ChildsID   []int  `json:"childs"`
	ParentID   *int   `json:"parent,omitempty"`
	ItemsID    []int  `json:"items"`
	ItemsCount int    `json:"itemsCount"`
}

type Item struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	InStock   int    `json:"inStock"`
	InCart    int    `json:"inCart"`
	SellerID  int    `json:"seller"`
	CatalogID int    `json:"catalog"`
}

type Seller struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Deals   int    `json:"deals"`
	ItemsID []int  `json:"items"`
}
