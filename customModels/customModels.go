package custom

type OrderItem struct {
	ItemID   int `json:"itemID"`
	Quantity int `json:"quantity"`
}

type Cart struct {
	CartItems []*OrderItem `json:"items"`
}

type Catalog struct {
	ID       int    `json:"id" bson:"_id"`
	Name     string `json:"name" bson:"name"`
	ParentID int    `json:"parent,omitempty" bson:"parentID"`
}

type Item struct {
	ID        int    `json:"id" bson:"_id"`
	Name      string `json:"name" bson:"name"`
	InStock   int    `json:"inStock" bson:"inStock"`
	InCart    int    `json:"inCart" bson:"inCart"`
	SellerID  int    `json:"seller" bson:"sellerID"`
	CatalogID int    `json:"catalog" bson:"catalogID"`
	Price     int    `json:"price" bson:"price"`
	Rating    int    `json:"rating" bson:"rating"`
}

type Seller struct {
	ID    int    `json:"id" bson:"_id"`
	Name  string `json:"name" bson:"name"`
	Deals int    `json:"deals" bson:"deals"`
}

type User struct {
	ID        int    `json:"id"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	Cart      *Cart  `json:"cart"`
}
