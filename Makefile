api-up:
	docker compose up api

api-down:
	docker compose down -v --remove-orphans
