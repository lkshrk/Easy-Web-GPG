#!/bin/bash
set -e

echo "================================"
echo "Running Easy Web GPG Tests"
echo "================================"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Track overall status
ALL_TESTS_PASSED=true

# Function to run Go tests
run_go_tests() {
  echo -e "\n${YELLOW}Running Go tests...${NC}"

  docker build --target go-test -t easy-web-gpg:test-go -f Dockerfile.test .

  if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Go tests passed${NC}"
  else
    echo -e "${RED}✗ Go tests failed${NC}"
    ALL_TESTS_PASSED=false
  fi
}

# Function to run E2E tests
run_e2e_tests() {
  echo -e "\n${YELLOW}Running E2E tests...${NC}"

  # Start the app container in background
  echo "Starting application container..."
  docker run -d --name easy-web-gpg-test-app \
    -p 8080:8080 \
    -e MASTER_PASSWORD="test-password" \
    easy-web-gpg:latest || {
    echo "Building application image first..."
    docker build -t easy-web-gpg:latest .
    docker run -d --name easy-web-gpg-test-app \
      -p 8080:8080 \
      -e MASTER_PASSWORD="test-password" \
      easy-web-gpg:latest
  }

  # Wait for app to be ready
  echo "Waiting for application to be ready..."
  for i in {1..30}; do
    if curl -sf http://localhost:8080/ > /dev/null 2>&1; then
      echo -e "${GREEN}Application is ready${NC}"
      break
    fi
    if [ $i -eq 30 ]; then
      echo -e "${RED}Timeout waiting for application${NC}"
      docker logs easy-web-gpg-test-app
      docker stop easy-web-gpg-test-app
      docker rm easy-web-gpg-test-app
      ALL_TESTS_PASSED=false
      return 1
    fi
    sleep 1
  done

  # Run Playwright tests
  docker build --target playwright-test -t easy-web-gpg:test-e2e -f Dockerfile.test .
  E2E_RESULT=$?

  # Cleanup
  echo "Cleaning up test containers..."
  docker stop easy-web-gpg-test-app > /dev/null 2>&1 || true
  docker rm easy-web-gpg-test-app > /dev/null 2>&1 || true

  if [ $E2E_RESULT -eq 0 ]; then
    echo -e "${GREEN}✓ E2E tests passed${NC}"
  else
    echo -e "${RED}✗ E2E tests failed${NC}"
    ALL_TESTS_PASSED=false
  fi
}

# Main test flow
case "${1:-all}" in
  go)
    run_go_tests
    ;;
  e2e)
    run_e2e_tests
    ;;
  all)
    run_go_tests
    run_e2e_tests
    ;;
  *)
    echo "Usage: $0 [go|e2e|all]"
    echo "  go  - Run Go tests only"
    echo "  e2e - Run E2E tests only"
    echo "  all - Run all tests (default)"
    exit 1
    ;;
esac

# Final status
echo ""
echo "================================"
if [ "$ALL_TESTS_PASSED" = true ]; then
  echo -e "${GREEN}All tests passed!${NC}"
  echo "================================"
  exit 0
else
  echo -e "${RED}Some tests failed${NC}"
  echo "================================"
  exit 1
fi
