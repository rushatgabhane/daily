.PHONY: build install run clean

BINARY_NAME=daily
INSTALL_PATH=$(HOME)/.local/bin

build:
	go build -o $(BINARY_NAME) .

install: build
	mkdir -p $(INSTALL_PATH)
	cp $(BINARY_NAME) $(INSTALL_PATH)/$(BINARY_NAME)
	chmod +x $(INSTALL_PATH)/$(BINARY_NAME)

run:
	go run .

clean:
	rm -f $(BINARY_NAME)

uninstall:
	rm -f $(INSTALL_PATH)/$(BINARY_NAME)
