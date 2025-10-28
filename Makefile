.PHONY: all cli simulator clean help

BINDIR := bin

all: cli simulator

cli:
	@mkdir -p $(BINDIR)
	go build -o $(BINDIR)/modbus-cli ./cmd/modbus-cli

simulator:
	@mkdir -p $(BINDIR)
	go build -o $(BINDIR)/modbus-simulator ./cmd/simulator

clean:
	rm -rf $(BINDIR)

help:
	@echo "Available targets:"
	@echo "  all        - Build both CLI and simulator (default)"
	@echo "  cli        - Build modbus-cli in bin/"
	@echo "  simulator  - Build modbus-simulator in bin/"
	@echo "  clean      - Remove bin/ directory"
	@echo "  help       - Show this help message"
