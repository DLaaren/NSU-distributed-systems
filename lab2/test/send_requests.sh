#!/bin/bash

for i in {1..10}; do
    curl -X POST http://localhost:8080/api/hash/crack \
        -H "Content-Type: application/json" \
        -d '{"hash":"187ef4436122d1cc2f40dc2b92f0eba0", "maxLength":2}'
    echo ""
done