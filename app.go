package main

import (
	"context"
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

	//ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	//defer cancel()
	err = client.Ping(context.Background(), readpref.Primary())
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
		if r.URL.Path != "/query" {
			next.ServeHTTP(w, r)
			return
		}
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Token ")
		//log.Printf("request from user with token - %v\n", token)
		ctx := context.WithValue(r.Context(), tokenContextKey, token)
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

func GetApp() http.Handler {
	mux := http.NewServeMux()
	cl := NewMongoClient("mongodb://admin:admin@localhost:27017")
	/*defer func() {
		//ctx, cancel := context.WithTimeout(context.TODO(), 2*time.Second)
		//defer cancel()
		if err := cl.Disconnect(context.Background()); err != nil {
			log.Fatalf("disconnect err - %v", err)
		}
		log.Println("mongo is disconnected")
	}()*/
	shopDB := storage.NewMongoDB(cl)
	userRepo := NewUserRepo(cl, "gpaphquerylanguage")
	if err := shopDB.ParseShop("testdata.json"); err != nil {
		log.Fatal(err)
	}
	config := graph.Config{Resolvers: &graph.Resolver{Shop: shopDB, User: userRepo}}
	config.Directives.Auth = func(ctx context.Context, obj any, next graphql.Resolver) (res any, err error) {
		token := TokenFromCtx(ctx)
		newCtx, err := userRepo.CheckJWT(ctx, token)
		if err != nil {
			log.Printf("@auth directive - bad token %v\n", err)
			graphql.AddError(ctx, &gqlerror.Error{
				Message: "User not authorized",
				Path:    graphql.GetFieldContext(ctx).Path(),
			})
			return nil, nil
		}
		return next(newCtx)
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
	mux.HandleFunc("/register", userRepo.Reg)
	return AuthMiddleWare(mux)
}
