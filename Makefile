up:
	docker-compose up -d
	./setup_dependencies.sh

up-dependencies:
	docker-compose up -d rabbitmq pubsub-emulator
	./setup_dependencies.sh

down:
	docker-compose down

docker:
	docker build -t eu.gcr.io/ssdc-rm-ci/rm/ssdc-rm-pubsub-adapter .

build:
	go build -race .

format:
	go fmt ./...

format-check:
	./format_check.sh

logs:
	docker-compose logs --follow

unit-test:
	go test -race ./... -tags=unitTest

run-int-test:
	PUBSUB_EMULATOR_HOST=localhost:8538 go test -count 1 github.com/ONSdigital/ssdc-rm-pubsub-adapter

int-test: down up-dependencies run-int-test down

test: unit-test int-test

build-test: format build test docker
