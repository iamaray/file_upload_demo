#!/bin/bash

# Test script to demonstrate error responses
echo "Testing error responses for the file upload API"
echo "Make sure the server is running on localhost:8080"
echo ""

# Test 1: Wrong HTTP method
echo "1. Testing wrong HTTP method (GET instead of POST):"
curl -s -X GET http://localhost:8080/v1/files/ | jq .
echo ""

# Test 2: Missing file
echo "2. Testing missing file in multipart form:"
curl -s -X POST http://localhost:8080/v1/files/ \
  -F "notfile=@/dev/null" | jq .
echo ""

# Test 3: Non-CSV file (if you have a text file)
echo "3. Testing non-CSV file:"
echo "test content" > /tmp/test.txt
curl -s -X POST http://localhost:8080/v1/files/ \
  -F "file=@/tmp/test.txt" | jq .
rm /tmp/test.txt
echo ""

# Test 4: Empty file
echo "4. Testing empty CSV file:"
touch /tmp/empty.csv
curl -s -X POST http://localhost:8080/v1/files/ \
  -F "file=@/tmp/empty.csv" | jq .
rm /tmp/empty.csv
echo ""

# Test 5: Valid CSV file (using the test data)
echo "5. Testing valid CSV file:"
curl -s -X POST http://localhost:8080/v1/files/ \
  -F "file=@testing/test_data/dummy.csv" | jq .
echo ""

echo "Done testing error responses!"
