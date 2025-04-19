#!/bin/bash
echo "=== Testing worker crash handling ==="

# 1. Get initial worker count
INITIAL_WORKERS=$(docker ps --filter "name=worker_" | wc -l)

# 2. Crash a random worker
WORKER_TO_CRASH=$(docker ps --filter "name=worker_" --format "{{.Names}}" | shuf -n 1)
echo "Crashing worker: $WORKER_TO_CRASH"
docker kill $WORKER_TO_CRASH

# 3. Wait for recovery
sleep 10  # Adjust based on your recovery time

# 4. Verify reassignment
CURRENT_WORKERS=$(docker ps --filter "name=worker_" | wc -l)
if [ $CURRENT_WORKERS -eq $INITIAL_WORKERS ]; then
  echo "Success: Worker count maintained"
else
  echo "Warning: Worker count changed"
fi

# 5. Check logs
echo "=== Coordinator logs ==="
docker logs coordinator | tail -n 20