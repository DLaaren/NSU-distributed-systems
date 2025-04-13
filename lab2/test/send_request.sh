#!/bin/bash

curl -X POST http://localhost:8080/api/hash/crack \
    -H "Content-Type: application/json" \
    -d '{"hash":"5d41402abc4b2a76b9719d911017c592", "maxLength":5}'
