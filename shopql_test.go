package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/mcuadros/go-lookup"
	"github.com/sanity-io/litter"
	"gopkg.in/d4l3k/messagediff.v1"
)

type CR map[string]interface{}

type GQLParams struct {
	Query     string `json:"query"`
	Variables CR     `json:"variables"`
}

type ApiTestCase struct {
	Name           string
	Event          []string
	Method         string
	BodyRaw        string
	GQL            string
	GQLVars        CR
	URL            string
	TokenName      string
	ResponseStatus int
	ResponsePath   string
	Expected       interface{}
	ExpectedRaw    string
	Before         func()
	After          func(*http.Response, []byte, interface{}) error
	CheckFunc      func(interface{}) error
}

var (
	client = &http.Client{Timeout: 10 * time.Second}
)

func dd(elems ...any) {
	litter.Dump(elems...)
}

func WeirdMagicClone(in interface{}) interface{} {
	return reflect.New(reflect.TypeOf(in).Elem()).Interface()
}

func JSONString(in interface{}) string {
	data, _ := json.Marshal(in)
	return string(data)
}

func TestApp(t *testing.T) {
	rand.Seed(time.Now().UnixNano())

	var (
		app = GetApp()
		ts  = httptest.NewServer(app)

		gqlURL = "/query"

		username = "golang"
		password = "love"
	)

	tplParams := map[string]string{
		"EMAIL":     username + "@example.com",
		"PASSWORD":  password,
		"USERNAME":  username,
		"EMAIL1":    "yahoo@example.com",
		"BAD_EMAIL": "yahooexample.com",
		"PASSWORD1": "1234",
		"USERNAME1": "user",
	}

	replaceRe := regexp.MustCompile("{{(.*?)}}")
	replaceBrackets := strings.NewReplacer("{", "", "}", "")
	replacer := func(key []byte) []byte {
		k := replaceBrackets.Replace(string(key))
		val, ok := tplParams[k]
		if !ok {
			t.Fatalf("not found key %s during tpl substitution", string(key))
		}
		return []byte(val)
	}

	testCases := []*ApiTestCase{
		&ApiTestCase{
			Name: "Catalogs list",
			GQL: `
			{
				Catalog(ID: "1") {
				  id
				  name
				  childs {
					id
					name
				  }
				}
			}
			`,
			URL: gqlURL,
			ExpectedRaw: `
			{
				"data": {
				  "Catalog": {
					"id": 1,
					"name": "ShopQL",
					"childs": [
					  {
						"id": 2,
						"name": "Книги"
					  },
					  {
						"id": 5,
						"name": "Чай"
					  }
					]
				  }
				}
			  }
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "Catalog page with param and items list - Tea - with default limit",
			GQL: `
			{
				Catalog(ID: "5") {
				  id
				  name
				  items {
					id
					name
				  }
				}
			}
			`,
			URL: gqlURL,
			ExpectedRaw: `
			{
				"data": {
				  "Catalog": {
					"id": 5,
					"name": "Чай",
					"items": [
					  {
						"id": 9,
						"name": "Си Пу Юань, Шен Пуэр"
					  },
					  {
						"id": 10,
						"name": "Мэнхай 7542, Шен Пуэр"
					  },
					  {
						"id": 11,
						"name": "Дянь Хун"
					  }
					]
				  }
				}
			  }
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "Catalog page with param and items list - Books - items in subcatalog",
			GQL: `
			{
				Catalog(ID: "2") {
				  id
				  name
				  childs {
					id
					name
					items {
					  id
					  name
					}
				  }
				}
			}
			`,
			URL: gqlURL,
			ExpectedRaw: `
			{
				"data": {
				  "Catalog": {
					"id": 2,
					"name": "Книги",
					"childs": [
					  {
						"id": 3,
						"name": "Алгоритмы",
						"items": [
						  {
							"id": 1,
							"name": "Грокаем алгоритмы | Бхаргава Адитья"
						  },
						  {
							"id": 2,
							"name": "Теоретический минимум по Computer Science | Фило Владстон Феррейра"
						  },
						  {
							"id": 3,
							"name": "Совершенный алгоритм. Основы | Рафгарден Тим"
						  }
						]
					  },
					  {
						"id": 4,
						"name": "Golang",
						"items": [
						  {
							"id": 5,
							"name": "Язык программирования Go | Донован Алан А. А., Керниган Брайан У."
						  },
						  {
							"id": 6,
							"name": "Go на практике | Butcher Matt, Фарина Мэтт Мэтт"
						  },
						  {
							"id": 7,
							"name": "Программирование на Go. Разработка приложений XXI века | Саммерфильд Марк"
						  }
						]
					  }
					]
				  }
				}
			  }
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "Catalog page with param and items list and pagination",
			GQL: `
			{
				Catalog(ID: "5") {
					id
					name
					items(limit: 1, offset: 1) {
					id
					name
					}
				}
			}
			`,
			URL: gqlURL,
			ExpectedRaw: `
			{
				"data": {
					"Catalog": {
					"id": 5,
					"name": "Чай",
					"items": [
						{
						"id": 10,
						"name": "Мэнхай 7542, Шен Пуэр"
						}
					]
					}
				}
				}
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "Catalog with seller name",
			GQL: `
			{
				Catalog(ID: "5") {
				  id
				  name
				  items(limit: 1, offset: 1) {
					id
					name
					seller {
					  name
					}
				  }
				}
			}
			`,
			URL: gqlURL,
			ExpectedRaw: `
			{
				"data": {
				  "Catalog": {
					"id": 5,
					"name": "Чай",
					"items": [
					  {
						"id": 10,
						"name": "Мэнхай 7542, Шен Пуэр",
						"seller": {
						  "name": "Дядюшка Ляо"
						}
					  }
					]
				  }
				}
			  }
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "Catalog with other seller",
			GQL: `
			{
				Catalog(ID: "3") {
				  id
				  name
				  items(limit: 5) {
					id
					name
					seller {
					  name
					}
				  }
				}
			}
			`,
			URL: gqlURL,
			ExpectedRaw: `
			{
				"data": {
				  "Catalog": {
					"id": 3,
					"name": "Алгоритмы",
					"items": [
					  {
						"id": 1,
						"name": "Грокаем алгоритмы | Бхаргава Адитья",
						"seller": {
						  "name": "Издательство Питер"
						}
					  },
					  {
						"id": 2,
						"name": "Теоретический минимум по Computer Science | Фило Владстон Феррейра",
						"seller": {
						  "name": "Издательство Питер"
						}
					  },
					  {
						"id": 3,
						"name": "Совершенный алгоритм. Основы | Рафгарден Тим",
						"seller": {
						  "name": "Издательство Питер"
						}
					  },
					  {
						"id": 4,
						"name": "Алгоритмы на Java | Джитер Кевин Уэйн, Седжвик Роберт",
						"seller": {
						  "name": "Издательство Вильямс"
						}
					  }
					]
				  }
				}
			}
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "Catalog - inStockText",
			GQL: `
			{
				Catalog(ID: "5") {
				  id
				  name
				  items(limit: 5) {
					id
					name
					inStockText
				  }
				}
			}			  
			`,
			URL: gqlURL,
			ExpectedRaw: `
			{
				"data": {
				  "Catalog": {
					"id": 5,
					"name": "Чай",
					"items": [
					  {
						"id": 9,
						"name": "Си Пу Юань, Шен Пуэр",
						"inStockText": "мало"
					  },
					  {
						"id": 10,
						"name": "Мэнхай 7542, Шен Пуэр",
						"inStockText": "хватает"
					  },
					  {
						"id": 11,
						"name": "Дянь Хун",
						"inStockText": "хватает"
					  },
					  {
						"id": 12,
						"name": "Да Хун Пао",
						"inStockText": "много"
					  },
					  {
						"id": 13,
						"name": "Габа Улун",
						"inStockText": "много"
					  }
					]
				  }
				}
			  }
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "Seller with items and item catalog",
			GQL: `
			{
				Seller(ID: "3") {
				  id
				  name
				  items(limit: 5) {
					id
					name
					parent {
					  id
					  name
					}
				  }
				}
			}
			`,
			URL: gqlURL,
			ExpectedRaw: `
			{
				"data": {
				  "Seller": {
					"id": 3,
					"name": "Издательство Питер",
					"items": [
					  {
						"id": 1,
						"name": "Грокаем алгоритмы | Бхаргава Адитья",
						"parent": {
						  "id": 3,
						  "name": "Алгоритмы"
						}
					  },
					  {
						"id": 2,
						"name": "Теоретический минимум по Computer Science | Фило Владстон Феррейра",
						"parent": {
						  "id": 3,
						  "name": "Алгоритмы"
						}
					  },
					  {
						"id": 3,
						"name": "Совершенный алгоритм. Основы | Рафгарден Тим",
						"parent": {
						  "id": 3,
						  "name": "Алгоритмы"
						}
					  },
					  {
						"id": 8,
						"name": "Head First. Изучаем Go | Макгаврен Джей",
						"parent": {
						  "id": 4,
						  "name": "Golang"
						}
					  }
					]
				  }
				}
			  }
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "Catalog - how many in cart - ERROR(no access) - directive @authorized",
			GQL: `
			{
				Catalog(ID: "5") {
				  id
				  name
				  items(limit: 1) {
					id
					name
					inCart
				  }
				}
			}
			`,
			URL: gqlURL,
			ExpectedRaw: `
			{
				"errors": [
				  {
					"message": "User not authorized",
					"path": [
					  "Catalog",
					  "items",
					  0,
					  "inCart"
					]
				  }
				],
				"data": {
				  "Catalog": null
				}
			}
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "Add to cart - ERROR(no access) - directive @authorized",
			GQL: `
			mutation {
				AddToCart(in: {itemID: 1, quantity: 2}) {
				  items {
				  	item {
						id
						name
				  	}
				  	quantity
		}
				}
			}
			`,
			URL: gqlURL,
			ExpectedRaw: `
			{
				"errors": [
				  {
					"message": "User not authorized",
					"path": [
					  "AddToCart"
					]
				  }
				],
				"data": null
			}
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "Rate item - ERROR(no access) - directive @authorized",
			GQL: `
			mutation {
				RateItem(in: {ID: "1", Rating: 1}) {
						id
						name
						inStockText
						seller{
							name
						}
						price						
						rating
					}
		}`,
			URL: gqlURL,
			ExpectedRaw: `
			{
				"errors": [
				  {
					"message": "User not authorized",
					"path": [
					  "RateItem"
					]
				  }
				],
				"data": null
			}
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "Add comment - ERROR(no access) - directive @authorized",
			GQL: `
			mutation {
				AddComment(in: {itemID: 1, text: "хороший качество"}) {
						id
						text
						userName
						item{
							id
							name
							}
					}
		}`,
			URL: gqlURL,
			ExpectedRaw: `
			{
				"errors": [
				  {
					"message": "User not authorized",
					"path": [
					  "AddComment"
					]
				  }
				],
				"data": null
			}
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name:           "Register user1 - success",
			URL:            "/register",
			Method:         http.MethodPost,
			BodyRaw:        "{\"user\":{\"email\":\"{{EMAIL}}\", \"password\":\"{{PASSWORD}}\", \"username\":\"{{USERNAME}}\"}}",
			ResponseStatus: 200,
			CheckFunc: func(resp interface{}) error {
				fmt.Println("CheckFunc got resp", resp)
				val, err := lookup.LookupString(resp, "body.token")
				if err != nil {
					return err
				}
				fmt.Println("-------------------- TOKEN:", val)
				tplParams["token1"] = val.String()
				return nil
			},
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name:           "Register user2 - bad credentials",
			URL:            "/register",
			Method:         http.MethodPost,
			BodyRaw:        "{\"user\":{\"email\":\"{{BAD_EMAIL}}\", \"password\":\"{{PASSWORD1}}\", \"username\":\"{{USERNAME1}}\"}}",
			ResponseStatus: 400,
			ExpectedRaw: `
			{
			"body": {
				"message": "invalid input data: Email",
				"status": "fail"
			}
			}`,
		},
		&ApiTestCase{
			Name:           "Register user2 - success",
			URL:            "/register",
			Method:         http.MethodPost,
			BodyRaw:        "{\"user\":{\"email\":\"{{EMAIL1}}\", \"password\":\"{{PASSWORD1}}\", \"username\":\"{{USERNAME1}}\"}}",
			ResponseStatus: 200,
			CheckFunc: func(resp interface{}) error {
				fmt.Println("CheckFunc got resp", resp)
				val, err := lookup.LookupString(resp, "body.token")
				if err != nil {
					return err
				}
				fmt.Println("-------------------- TOKEN:", val)
				tplParams["token2"] = val.String()
				return nil
			},
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "User 1 - Rate item - Success",
			GQL: `
			mutation {
				RateItem(in: {ID: "1", Rating: 1}) {
						id
						name
						inStockText
						seller{
							name
						}
						price						
						rating
					}
		}`,
			URL:       gqlURL,
			TokenName: "token1",
			ExpectedRaw: `
			{"data":{"RateItem":{"id":1,"name":"Грокаем алгоритмы | Бхаргава Адитья","inStockText":"мало","seller":{"name":"Издательство Питер"},"price":2,"rating":1}}}
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "User 1 - ReRate item - Success",
			GQL: `
			mutation {
				RateItem(in: {ID: "1", Rating: 5}) {
						id
						name
						inStockText
						seller{
							name
						}
						price						
						rating
					}
		}`,
			URL:       gqlURL,
			TokenName: "token1",
			ExpectedRaw: `
			{"data":{"RateItem":{"id":1,"name":"Грокаем алгоритмы | Бхаргава Адитья","inStockText":"мало","seller":{"name":"Издательство Питер"},"price":2,"rating":5}}}
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "Search in catalog 3, sort by quantity",
			GQL: `
			{
				Search(params: {catalogID: "3", sort: STOCK_QUANTITY}) {
				name
  				seller{
					name
				}
  				rating
  				commentsCount
				inStockText
				}		
			}
			`,
			URL: gqlURL,
			ExpectedRaw: `
			{"data":{
				"Search":[
					{
						"name":"Грокаем алгоритмы | Бхаргава Адитья",
						"seller":{"name":"Издательство Питер"},
						"rating":5,
						"commentsCount":0,
						"inStockText":"мало"
					},
					{	
						"name":"Теоретический минимум по Computer Science | Фило Владстон Феррейра",
						"seller":{"name":"Издательство Питер"},
						"rating":0,
						"commentsCount":0,
						"inStockText":"хватает"
					},
					{
						"name":"Совершенный алгоритм. Основы | Рафгарден Тим",
						"seller":{"name":"Издательство Питер"},
						"rating":0,
						"commentsCount":0,
						"inStockText":"хватает"
					},
					{
						"name":"Алгоритмы на Java | Джитер Кевин Уэйн, Седжвик Роберт",
						"seller":{"name":"Издательство Вильямс"},
						"rating":0,
						"commentsCount":0,
						"inStockText":"много"
					}
				]
			}
		}`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "Search in catalog 3, sort by name",
			GQL: `
			{
				Search(params: {catalogID: "3", sort: NAME}) {
				name
  				rating
				}		
			}
			`,
			URL: gqlURL,
			ExpectedRaw: `
			 {"data":{
			 	"Search":[
					{"name":"Алгоритмы на Java | Джитер Кевин Уэйн, Седжвик Роберт","rating":0},
					{"name":"Грокаем алгоритмы | Бхаргава Адитья","rating":5},
					{"name":"Совершенный алгоритм. Основы | Рафгарден Тим","rating":0},
					{"name":"Теоретический минимум по Computer Science | Фило Владстон Феррейра","rating":0}
				]
			}
		}
			 `,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "Search in catalog 3, sort by rating, limit = 1",
			GQL: `
			{
				Search(params: {catalogID: "3", sort: RATING, limit: 1,  offset: 0}) {
				name
  				rating
				}		
			}
			`,
			URL: gqlURL,
			ExpectedRaw: `
			{
    "data": {
        "Search": [
            {
                "name": "Грокаем алгоритмы | Бхаргава Адитья",
                "rating": 5
            }
        ]
    }
}
			 `,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "Search, min price > 1000 - empty slice",
			GQL: `
			{
				Search(params: {minPrice: 1000}) {
				name
  				rating
				}		
			}
			`,
			URL: gqlURL,
			ExpectedRaw: `
			{"data":{"Search":[]}}
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "Search by keyword - Go",
			GQL: `
			{
				Search(params: {keyword: "Go"}) {
				name
				}		
			}
			`,
			URL: gqlURL,
			ExpectedRaw: `
			{"data":{
				"Search":[
					{"name":"Язык программирования Go | Донован Алан А. А., Керниган Брайан У."},
					{"name":"Go на практике | Butcher Matt, Фарина Мэтт Мэтт"},				
					{"name":"Программирование на Go. Разработка приложений XXI века | Саммерфильд Марк"},
					{"name":"Head First. Изучаем Go | Макгаврен Джей"}					
				]
			}}
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "Add to cart - first item - success",
			GQL: `
			mutation {
				AddToCart(in: {itemID: 12, quantity: 2}) {
				  items {
				  item {
					id
					name
					inStockText
				  }
				  quantity
		}
				  cost
				}
			}
			`,
			URL:       gqlURL,
			TokenName: "token1",
			ExpectedRaw: `
			{
    "data": {
        "AddToCart": {
            "items": [
                {
                    "item": {
                        "id": 12,
                        "name": "Да Хун Пао",
                        "inStockText": "хватает"
                    },
                    "quantity": 2
                }
            ],
            "cost": 34
        }
    }
}			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "User 2 - Add comment - ERROR(censor filter) - directive @validate",
			GQL: `
			mutation {
				AddComment(in: {itemID: 1, text: "хороший качество шалава"}) {
						id
						text
						userName
						item{
							id
							name
							}
					}
		}`,
			URL:       gqlURL,
			TokenName: "token2",
			ExpectedRaw: `
			{
				"errors": [
				  {
					"message": "bad words in comment",
					"path": [
					  "AddComment"
					]
				  }
				],
				"data": null
			}
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "User 1 - Add comment - Success ",
			GQL: `
			mutation {
				AddComment(in: {itemID: 12, text: "хороший качество товар"}) {
						text
						userName
						item{
							id
							name							
							}
					}
		}`,
			URL:       gqlURL,
			TokenName: "token1",
			ExpectedRaw: `
			 {"data":{"AddComment":{"text":"хороший качество товар","userName":"golang","item":{"id":12,"name":"Да Хун Пао"}}}}
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "User 2 - Add comment - Success ",
			GQL: `
			mutation {
				AddComment(in: {itemID: 12, text: "люблю этот чай"}) {
						text
						userName
						item{
							id
							name							
							}
					}
		}`,
			URL:       gqlURL,
			TokenName: "token2",
			ExpectedRaw: `
			 {"data":{"AddComment":{"text":"люблю этот чай","userName":"user","item":{"id":12,"name":"Да Хун Пао"}}}}
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "User 2 - Update comment - Success ",
			GQL: `
			mutation {
				AddComment(in: {itemID: 12, text: "чай чушь полная"}) {
						text
						userName
						item{
							id
							name							
							}
					}
		}`,
			URL:       gqlURL,
			TokenName: "token2",
			ExpectedRaw: `
			 {"data":{"AddComment":{"text":"чай чушь полная","userName":"user","item":{"id":12,"name":"Да Хун Пао"}}}}
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "User 2 - Rate comment - Success ",
			GQL: `
			mutation {
				RateComment(item: 12, user: "user", rating: 5) {
						text
						userName
						rating
						item{
							id
							name							
							}
					}
		}`,
			URL:       gqlURL,
			TokenName: "token2",
			ExpectedRaw: `
			 {"data":{"RateComment":{"text":"чай чушь полная","userName":"user","rating":5,"item":{"id":12,"name":"Да Хун Пао"}}}}
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "User 2 - Re Rate comment - Success ",
			GQL: `
			mutation {
				RateComment(item: 12, user: "user", rating: 2) {
						text
						userName
						rating
						item{
							id
							name							
							}
					}
		}`,
			URL:       gqlURL,
			TokenName: "token2",
			ExpectedRaw: `
			 {"data":{"RateComment":{"text":"чай чушь полная","userName":"user","rating":2,"item":{"id":12,"name":"Да Хун Пао"}}}}
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "User 1 - Rate comment - Success ",
			GQL: `
			mutation {
				RateComment(item: 12, user: "user", rating: 4) {
						text
						userName
						rating
						item{
							id
							name							
							}
					}
		}`,
			URL:       gqlURL,
			TokenName: "token1",
			ExpectedRaw: `
			 {"data":{"RateComment":{"text":"чай чушь полная","userName":"user","rating":3,"item":{"id":12,"name":"Да Хун Пао"}}}}
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "Search, sort by COMMENTS_COUNT with limit",
			GQL: `
			{
				Search(params: {sort: COMMENTS_COUNT, limit: 5}) {
				name
  				commentsCount
				}		
			}
			`,
			URL: gqlURL,
			ExpectedRaw: `{
				"data":{
					"Search":[
						{"name":"Да Хун Пао","commentsCount":2},
						{"name":"Алгоритмы на Java | Джитер Кевин Уэйн, Седжвик Роберт","commentsCount":0},
						{"name":"Язык программирования Go | Донован Алан А. А., Керниган Брайан У.","commentsCount":0},
						{"name":"Теоретический минимум по Computer Science | Фило Владстон Феррейра","commentsCount":0},
						{"name":"Грокаем алгоритмы | Бхаргава Адитья","commentsCount":0}
					]
				}
			}
			 `,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "Search, catalog 5, sort by price with limit && offset",
			GQL: `
			{
				Search(params: {catalogID: "5", sort: PRICE, limit: 2, offset: 2}) {
				name
  				inStockText
				price
				}		
			}
			`,
			URL: gqlURL,
			ExpectedRaw: `{
				"data":{
					"Search":[
						{"name":"Дянь Хун","inStockText":"хватает","price":14},
						{"name":"Да Хун Пао","inStockText":"хватает","price":17}
					]
				}
			}
			 `,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "Catalog page with param and items list and pagination, items with comments",
			GQL: `
			{
				Catalog(ID: "5") {
					id
					name
					items(limit: 3, offset: 3) {
					id
					name
					comments {
						userName
						text
						}
					}
				}
			}
			`,
			URL: gqlURL,
			ExpectedRaw: `
			{
			"data":{
				"Catalog":{
					"id":5,
					"name":"Чай",
					"items":[
						{"id":12,
						"name":"Да Хун Пао",
						"comments":[
							{"userName":"user",	"text":"чай чушь полная"},
							{"userName":"golang","text":"хороший качество товар"}]
						},
						{"id":13,
						"name":"Габа Улун",
						"comments":[]
						}
					]
				}
			}
		}
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "Catalog page with param and items list and pagination, items with comments list and pagination",
			GQL: `
			{
				Catalog(ID: "5") {
					id
					name
					items(limit: 1, offset: 3) {
					id
					name
					commentsCount
					comments(limit: 1, offset: 0) {
						userName
						text
						}
					}
				}
			}
			`,
			URL: gqlURL,
			ExpectedRaw: `
			{
			"data":{
				"Catalog":{
					"id":5,
					"name":"Чай",
					"items":[
						{"id":12,
						"name":"Да Хун Пао",
						"commentsCount": 2,
						"comments":[
							{"userName":"user",	"text":"чай чушь полная"}
							]
						}
					]
				}
			}
		}
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "Add to cart - first item - check correct increment in cart",
			GQL: `
			mutation {
				AddToCart(in: {itemID: 12, quantity: 2}) {
				  items {
				  item {
					id
					name
					inStockText
				  }
				  quantity
				}
		}
			}
			`,
			URL:       gqlURL,
			TokenName: "token1",
			ExpectedRaw: `
			{
    "data": {
        "AddToCart": {
            "items": [
                {
                    "item": {
                        "id": 12,
                        "name": "Да Хун Пао",
                        "inStockText": "мало"
                    },
                    "quantity": 4
                }
            ]
        }
    }
}
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "Add to cart - first item - check quantity availability",
			GQL: `
			mutation {
				AddToCart(in: {itemID: 12, quantity: 2}) {
				  items {
				  item {
					id
					name
					inStockText
				  }
				  quantity
				}
		}
			}
			`,
			URL:       gqlURL,
			TokenName: "token1",
			ExpectedRaw: `
			{
				"errors": [
				  {
					"message": "not enough quantity",
					"path": [
					  "AddToCart"
					]
				  }
				],
				"data": null
			}
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "Add to cart - second item - before delete check",
			GQL: `
			mutation {
				AddToCart(in: {itemID: 1, quantity: 1}) {
				  items {
				  item {
					id
					name
					inStockText
				  }
				  quantity
				}
			}
		}
			`,
			URL:       gqlURL,
			TokenName: "token1",
			ExpectedRaw: `
			{
    "data": {
        "AddToCart": {
            "items": [
                {
                    "item": {
                        "id": 12,
                        "name": "Да Хун Пао",
                        "inStockText": "мало"
                    },
                    "quantity": 4
                },
                {
                    "item": {
                        "id": 1,
                        "name": "Грокаем алгоритмы | Бхаргава Адитья",
                        "inStockText": "мало"
                    },
                    "quantity": 1
                }
            ]
        }
    }
}
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "Remove from cart",
			GQL: `
			mutation {
				RemoveFromCart(in: {itemID: 1, quantity: 1}) {
				  items {
				  item {
					id
					name
				  }
				  quantity
				}
			}
		}
			`,
			URL:       gqlURL,
			TokenName: "token1",
			ExpectedRaw: `
			{
    "data": {
        "RemoveFromCart": {
            "items": [
                {
                    "item": {
                        "id": 12,
                        "name": "Да Хун Пао"
                    },
                    "quantity": 4
                }
            ]
        }
    }
}
			`,
		},

		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "My Cart",
			GQL: `
			{
				MyCart {
				  items {
				  item {
					id
					name
					inStockText
				  }
				  quantity				  
				}
				  cost
				  count
			}
		}
			`,
			URL:       gqlURL,
			TokenName: "token1",
			ExpectedRaw: `
			{
				"data": {
				  "MyCart": {
				  "items":
				  [
					{
					  "item": {
						"id": 12,
						"name": "Да Хун Пао",
						"inStockText": "мало"
					  },
					  "quantity": 4
					}
				  ],
				  "cost": 68,
				  "count": 4
				}
			}
		}
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "Catalog page with inCart param",
			GQL: `
			query{
				Catalog(ID: "5") {
				  id
				  name
				  items(limit:8) {
					id
					name
					inCart
					price
				  }
				}
			}
			`,
			URL:       gqlURL,
			TokenName: "token1",
			ExpectedRaw: `
			{
				"data": {
				  "Catalog": {
					"id": 5,
					"name": "Чай",
					"items": [
					  {
						"id": 9,
						"name": "Си Пу Юань, Шен Пуэр",
						"inCart": 0,
						"price":10
					  },
					  {
						"id": 10,
						"name": "Мэнхай 7542, Шен Пуэр",
						"inCart": 0,
						"price":12
					  },
					  {
						"id": 11,
						"name": "Дянь Хун",
						"inCart": 0,
						"price":14
					  },
					  {
						"id": 12,
						"name": "Да Хун Пао",
						"inCart": 4,
						"price":17
					  },
					  {
						"id": 13,
						"name": "Габа Улун",
						"inCart": 0,
						"price":17
					  }
					]
				  }
				}
			  }
			`,
		},
		&ApiTestCase{
			Name: "User 1 - Make Order",
			GQL: `
			mutation {
				MakeOrder {
				  totalSum
				  items {
				  	itemID
					name
					purchasePrice
					quantity
				  }
			}
		}
			`,
			URL:       gqlURL,
			TokenName: "token1",
			ExpectedRaw: `{
				"data":{
					"MakeOrder":{
						"totalSum":68,
						"items":[
							{"itemID":12,"name":"Да Хун Пао","purchasePrice":17,"quantity":4}
						]
					}
				}
			}
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "User 1 - Make Order twice - Error",
			GQL: `
			mutation {
				MakeOrder {
				  items {
				  	itemID
					name
					purchasePrice
					quantity
				  }
			}
		}
			`,
			URL:       gqlURL,
			TokenName: "token1",
			ExpectedRaw: `{
				"errors":[
					{"message":"cart is empty. Nothing to order","path":["MakeOrder"]}
				],
				"data":null
			}
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "User 1 - Search Orders",
			GQL: `
			{
				 MyOrders(params: {}) {
				  totalSum
				  items {
				  	itemID
					name
					purchasePrice
					quantity
				  }
			}
		}
			`,
			URL:       gqlURL,
			TokenName: "token1",
			ExpectedRaw: `{
				"data":{
					"MyOrders":[
						{
							"totalSum":68,
							"items":[
								{
									"itemID":12,
									"name":"Да Хун Пао",
									"purchasePrice":17,
									"quantity":4
								}
							]
						}	
					]
				}
			}
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "Catalog page with inCart param after order",
			GQL: `
			query{
				Catalog(ID: "5") {
				  id
				  name
				  items(limit:8) {
					id
					name
					inCart
					price
				  }
				}
			}
			`,
			URL:       gqlURL,
			TokenName: "token1",
			ExpectedRaw: `
			{
				"data": {
				  "Catalog": {
					"id": 5,
					"name": "Чай",
					"items": [
					  {
						"id": 9,
						"name": "Си Пу Юань, Шен Пуэр",
						"inCart": 0,
						"price":10
					  },
					  {
						"id": 10,
						"name": "Мэнхай 7542, Шен Пуэр",
						"inCart": 0,
						"price":12
					  },
					  {
						"id": 11,
						"name": "Дянь Хун",
						"inCart": 0,
						"price":14
					  },
					  {
						"id": 12,
						"name": "Да Хун Пао",
						"inCart": 0,
						"price":17
					  },
					  {
						"id": 13,
						"name": "Габа Улун",
						"inCart": 0,
						"price":17
					  }
					]
				  }
				}
			  }
			`,
		},
		&ApiTestCase{
			Name: "User 1: Add to cart - after making order",
			GQL: `
			mutation {
				AddToCart(in: {itemID: 8, quantity: 3}) {
				  items {
				  item {
					id
					name
					inStockText
				  }
				  quantity
				}
			}
		}
			`,
			URL:       gqlURL,
			TokenName: "token1",
			ExpectedRaw: `{
				"data":{
					"AddToCart":{
						"items":[
							{
								"item":{
									"id":8,
									"name":"Head First. Изучаем Go | Макгаврен Джей",
									"inStockText":"мало"
								},
								"quantity":3
							}
						]
					}
				}
			}`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "User 1 - Make Order",
			GQL: `
			mutation {
				MakeOrder {
				  totalSum
				  items {
				  	itemID
					name
					purchasePrice
					quantity
				  }
			}
		}
			`,
			URL:       gqlURL,
			TokenName: "token1",
			ExpectedRaw: `{
				"data":{
					"MakeOrder":{
						"totalSum":36,
						"items":[
							{
								"itemID":8,
								"name":"Head First. Изучаем Go | Макгаврен Джей",
								"purchasePrice":12,
								"quantity":3
							}
						]
					}
				}
			}
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "User 1 - Search Orders Sort by sum",
			GQL: `
			{
				 MyOrders(params: {sort: PRICE}) {
				  totalSum
				  items {
				  	itemID
					name
					purchasePrice
					quantity
				  }
			}
		}
			`,
			URL:       gqlURL,
			TokenName: "token1",
			ExpectedRaw: `{
    "data": {
        "MyOrders": [
            {
                "totalSum": 36,
                "items": [
                    {
                        "itemID": 8,
                        "name": "Head First. Изучаем Go | Макгаврен Джей",
                        "purchasePrice": 12,
                        "quantity": 3
                    }
                ]
            },
            {
                "totalSum": 68,
                "items": [
                    {
                        "itemID": 12,
                        "name": "Да Хун Пао",
                        "purchasePrice": 17,
                        "quantity": 4
                    }
                ]
            }
        ]
    }
}
			`,
		},
		// ----------------------------------------------------------------------------------------
		&ApiTestCase{
			Name: "User 1 - Search Orders Sort by date",
			GQL: `
			{
				 MyOrders(params: {sort: CREATED_AT}) {
				  totalSum
				  items {
				  	itemID
					name
					purchasePrice
					quantity
				  }
			}
		}
			`,
			URL:       gqlURL,
			TokenName: "token1",
			ExpectedRaw: `{
    "data": {
        "MyOrders": [			 
            {
                "totalSum": 36,
                "items": [
                    {
                        "itemID": 8,
                        "name": "Head First. Изучаем Go | Макгаврен Джей",
                        "purchasePrice": 12,
                        "quantity": 3
                    }
                ]
            },
			{
                "totalSum": 68,				
                "items": [
                    {
                        "itemID": 12,
                        "name": "Да Хун Пао",
                        "purchasePrice": 17,
                        "quantity": 4
                    }
                ]
            }        
        ]
    }
}
			`,
		},
		// ----------------------------------------------------------------------------------------

	}

	for _, item := range testCases {
		ok := t.Run(item.Name, func(t *testing.T) {
			if item.Before != nil {
				item.Before()
			}
			// some kind of eval params with substitution
			if item.Expected != nil {
				item.Expected = item.Expected.(func() interface{})()
			} else if item.ExpectedRaw != "" {
				// var data CR
				err := json.Unmarshal([]byte(item.ExpectedRaw), &item.Expected)
				if err != nil {
					t.Fatalf("cant unmarshal json: %v", err)
				}
			}

			var (
				body []byte
				url  = replaceRe.ReplaceAllFunc([]byte(ts.URL+item.URL), replacer)
			)
			if item.GQL != "" {
				item.BodyRaw = JSONString(&GQLParams{
					Query:     item.GQL,
					Variables: item.GQLVars,
				})
			}
			if item.BodyRaw != "" {
				body = replaceRe.ReplaceAllFunc([]byte(item.BodyRaw), replacer)
			}

			// t.Log("body", item.BodyRaw)
			if item.URL == gqlURL {
				item.Method = "POST"
			}
			req, _ := http.NewRequest(item.Method, string(url), bytes.NewReader(body))
			req.Header.Add("Content-Type", "application/json")

			if item.TokenName != "" {
				req.Header.Add("Authorization", "Token "+tplParams[item.TokenName])
			}

			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("request error: %v", err)
			}
			defer resp.Body.Close()
			respBody, _ := io.ReadAll(resp.Body)

			// t.Logf("\nreq body: %s\nresp body: %s", body, respBody)

			// t.Log((item.ResponseStatus == 0 && resp.StatusCode != 200), item.ResponseStatus == 0, resp.StatusCode != 200, item.ResponseStatus, resp.StatusCode, resp.StatusCode == 200)

			if item.ResponseStatus != 0 && (item.ResponseStatus != resp.StatusCode) {
				t.Fatalf("bad status code, want: %v, have:%v", item.ResponseStatus, resp.StatusCode)
			}

			// for cases with just status check
			if item.Expected == nil && item.CheckFunc == nil {
				return
			}
			var got interface{}
			err = json.Unmarshal(respBody, &got)
			if err != nil {
				t.Fatalf("cant unmarshal resp: %s, body: %s", err, respBody)
			}

			// for custom checking logic
			// i'm to lazy to code entire registrtion flow, so it's just check and set token inside
			if item.CheckFunc != nil {
				if err := item.CheckFunc(got); err != nil {
					t.Fatal("CheckFunc failed:", err)
				}
				return
			}

			diff, equal := messagediff.PrettyDiff(item.Expected, got)
			if !equal {
				dd(item.Expected, got)
				t.Fatalf("\033[1;31mresults not match\033[0m\n\033[1;35mbody\033[0m: %s\n\033[1;32mwant\033[0m %#v\n\033[1;34mgot\033[0m %#v\n\033[1;33mdiff\033[0m:\n%s", respBody, item.Expected, got, diff)
			}

			if item.After != nil {
				err = item.After(resp, respBody, got)
				if err != nil {
					t.Fatalf("after func failed %s", err)
				}
			}
		})
		if !ok {
			break
		}
	}
}

func __test_dummy() {
	fmt.Println(123)
}
