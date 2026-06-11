.PHONY: build up down status logs proto

proto:
	@echo "Proto generation placeholder"

build:
	docker compose build

up:
	docker compose up -d

down:
	docker compose down

status:
	docker compose ps

logs:
	docker compose logs
