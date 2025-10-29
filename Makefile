.PHONY: all cli simulator clean test test-unit test-integration test-coverage help

BINDIR := bin
COVERAGE_FILE := coverage.txt

all: cli simulator

cli:
	@mkdir -p $(BINDIR)
	go build -o $(BINDIR)/modbus-cli ./cmd/cli

simulator:
	@mkdir -p $(BINDIR)
	go build -o $(BINDIR)/modbus-simulator ./cmd/simulator

clean:
	rm -rf $(BINDIR)
	rm -f $(COVERAGE_FILE)

# Run all tests (unit + integration)
test: test-unit test-integration

# Run unit tests in the root package
test-unit:
	@echo "Running unit tests..."
	go test -v -race .

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	go test -v -race ./integration

# Run unit tests with coverage report
test-coverage:
	@echo "Running unit tests with coverage..."
	go test -v -race -coverprofile=$(COVERAGE_FILE) -covermode=atomic .
	@echo "\nCoverage report generated: $(COVERAGE_FILE)"
	@echo "View coverage with: go tool cover -html=$(COVERAGE_FILE)"

help:
	@echo "Available targets:"
	@echo "  all              - Build both CLI and simulator (default)"
	@echo "  cli              - Build modbus-cli in bin/"
	@echo "  simulator        - Build modbus-simulator in bin/"
	@echo "  test             - Run all tests (unit + integration)"
	@echo "  test-unit        - Run unit tests only"
	@echo "  test-integration - Run integration tests only"
	@echo "  test-coverage    - Run unit tests with coverage report"
	@echo "  clean            - Remove bin/ directory and coverage files"
	@echo "  help             - Show this help message"
