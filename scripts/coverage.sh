#!/bin/bash

# Test Coverage Script for NinjaBot
# Usage: ./scripts/coverage.sh [package]

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Default to all packages
PACKAGE="${1:-./.../}"

echo "🧪 Running tests with coverage..."
echo ""

# Run tests with coverage
go test ${PACKAGE} -coverprofile=coverage.out -covermode=atomic

echo ""
echo "📊 Coverage Summary:"
echo "===================="

# Extract coverage by package
go tool cover -func=coverage.out | grep -E "^(github.com|total)" | while read line; do
    if [[ $line == *"total:"* ]]; then
        coverage=$(echo $line | awk '{print $NF}')
        percentage=${coverage%\%}
        
        if (( $(echo "$percentage >= 70" | bc -l) )); then
            echo -e "${GREEN}✓ $line${NC}"
        elif (( $(echo "$percentage >= 50" | bc -l) )); then
            echo -e "${YELLOW}⚠ $line${NC}"
        else
            echo -e "${RED}✗ $line${NC}"
        fi
    else
        package=$(echo $line | awk '{print $1}' | sed 's/:.*//')
        coverage=$(echo $line | awk '{print $NF}')
        percentage=${coverage%\%}
        
        if (( $(echo "$percentage >= 70" | bc -l) )); then
            color=$GREEN
            symbol="✓"
        elif (( $(echo "$percentage >= 50" | bc -l) )); then
            color=$YELLOW
            symbol="⚠"
        else
            color=$RED
            symbol="✗"
        fi
        
        echo -e "${color}${symbol} ${package}: ${coverage}${NC}"
    fi
done

echo ""
echo "📈 Detailed Report:"
echo "==================="
echo "Run 'go tool cover -html=coverage.out' to view detailed HTML report"

# Generate HTML report
if command -v xdg-open &> /dev/null; then
    read -p "Open HTML report in browser? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        go tool cover -html=coverage.out
    fi
fi
