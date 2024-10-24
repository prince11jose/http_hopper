APP_NAME = http_hopper
SOURCES = main.go forwarder.go handlers.go logger.go mongodb.go router.go

build:
	go build -o $(APP_NAME) $(SOURCES)

run: build
	./$(APP_NAME)

clean:
	rm -f $(APP_NAME)
