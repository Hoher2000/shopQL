package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/99designs/gqlgen/graphql"
	"github.com/99designs/gqlgen/graphql/handler"
	"github.com/99designs/gqlgen/graphql/handler/extension"
	"github.com/99designs/gqlgen/graphql/handler/lru"
	"github.com/99designs/gqlgen/graphql/handler/transport"
	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/Hoher2000/shopQL/graph"
	"github.com/Hoher2000/shopQL/storage"
	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/gqlerror"

	custom "github.com/Hoher2000/shopQL/customModels"
)

type contextKey string

var tokenContextKey = contextKey("token")

func TokenFromCtx(ctx context.Context) string {
	return ctx.Value(tokenContextKey).(string)
}

func AuthMiddleWare(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Token ")
		ctx := context.WithValue(r.Context(), tokenContextKey, token)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

func GetApp() http.Handler {
	mux := http.NewServeMux()
	shop, err := storage.ParseShop("testdata.json")
	if err != nil {
		log.Fatal(err)
	}
	config := graph.Config{Resolvers: &graph.Resolver{Shop: shop, Cart: make([]*custom.CartItem, 0)}}
	config.Directives.Auth = func(ctx context.Context, obj any, next graphql.Resolver) (res any, err error) {
		token := TokenFromCtx(ctx)
		if token == "" {
			graphql.AddError(ctx, &gqlerror.Error{
				Message: "User not authorized",
				Path:    graphql.GetFieldContext(ctx).Path(),
			})
			return nil, nil
		}
		return next(ctx)
	}

	srv := handler.New(graph.NewExecutableSchema(config))
	srv.AddTransport(transport.Options{})
	srv.AddTransport(transport.GET{})
	srv.AddTransport(transport.POST{})
	srv.SetQueryCache(lru.New[*ast.QueryDocument](1000))
	srv.Use(extension.Introspection{})
	srv.Use(extension.AutomaticPersistedQuery{
		Cache: lru.New[string](100),
	})

	mux.Handle("/", playground.Handler("GraphQL playground", "/query"))
	mux.Handle("/query", srv)
	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			log.Printf("registration - bad method. Want - POST, get - %v\n", r.Method)
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		data := map[string]map[string]string{}
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			log.Printf("registration - invalid JSON body - %v\n", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()
		//w.WriteHeader(http.StatusOK)
		w.Header().Add("Authorization", "Token "+"12345678")
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]map[string]any{
			"body": {
				"status":  "success",
				"message": "user registrated successfully",
				"token":   "12345678",
			},
		}
		json.NewEncoder(w).Encode(resp)
		log.Printf("registration success - %v\n", data)
	})
	return AuthMiddleWare(mux)
}
