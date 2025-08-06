up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f 

proto:
	@echo "Creating output directory..."
	mkdir -p ./gen/go
	@echo "Generating Go code from protobuf definitions..."
	protoc --go_out=./gen/go --go-grpc_out=./gen/go \
			-I./protos protos/*.proto

seed:
	@echo "Seeding database with test data..."
	@docker-compose exec -T postgres sh -c 'psql -U "$$POSTGRES_USER" -d "$$POSTGRES_DB" -f ./scripts/seed.sql'
