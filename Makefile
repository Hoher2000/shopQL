all:
	go run .

.PHONY: runmongo
runmongo:
	@echo "-- starting mongoDB"
	docker-compose up -d

.PHONY: statusmongo
statusmongo:
	docker-compose ps

.PHONY: stopmongo
stopmongo:
	docker-compose down

.PHONY: clearmongo
clearmongo:
	docker-compose down -v

.PHONY: test
test:
	$(MAKE) clearmongo
	$(MAKE) runmongo
	sleep 0.5
	go test -v
	$(MAKE) clearmongo
	$(MAKE) statusmongo

.PHONY: install
install:
	go install github.com/99designs/gqlgen@v0.17.45

.PHONY: init
init:
	go run github.com/99designs/gqlgen init

.PHONY: gen
gen: 
	@echo "-- generatiog graphql files"
	go run github.com/99designs/gqlgen generate --verbose --config ./gqlgen.yml


