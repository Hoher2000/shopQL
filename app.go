package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

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
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

type contextKey string

var tokenContextKey = contextKey("token")

func NewMongoClient(mongoURI string) *mongo.Client {
	options := options.
		Client().
		ApplyURI(mongoURI)

	client, err := mongo.Connect(options)
	if err != nil {
		log.Fatalf("MongoDB client options error - %v\n", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		log.Fatalf("MongoDB connection error - %v\n", err)
	}
	log.Printf("MongoDB started at %v\n", strings.Split(mongoURI, "@")[1])
	return client
}

func TokenFromCtx(ctx context.Context) string {
	return ctx.Value(tokenContextKey).(string)
}

func AuthMiddleWare(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Token ")
		log.Printf("request from user with token - %v\n", token)
		ctx := context.WithValue(r.Context(), tokenContextKey, token)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

func GetApp() http.Handler {
	mux := http.NewServeMux()
	cl := NewMongoClient("mongodb://admin:admin@localhost:27017")
	defer func() {
		ctx, cancel := context.WithTimeout(context.TODO(), 2*time.Second)
		defer cancel()
		if err := cl.Disconnect(ctx); err != nil {
			log.Fatal(err)
		}
	}()
	shopDB := storage.NewMongoDB(cl)
	if err := shopDB.ParseShop("testdata.json"); err != nil {
		log.Fatal(err)
	}
	config := graph.Config{Resolvers: &graph.Resolver{Shop: shopDB}}
	config.Directives.Auth = func(ctx context.Context, obj any, next graphql.Resolver) (res any, err error) {
		token := TokenFromCtx(ctx)
		if token == "" {
			log.Println("@auth directive - empty token")
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
