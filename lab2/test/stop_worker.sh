#!/bin/bash

CONTAINER_NAME="lab2-worker-$1"

docker stop "$CONTAINER_NAME"

sleep 10;

docker start "$CONTAINER_NAME"

