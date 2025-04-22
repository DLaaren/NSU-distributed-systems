#!/bin/bash

CONTAINER_NAME="coordinator"

docker compose stop "$CONTAINER_NAME"

sleep 10;

docker compose start "$CONTAINER_NAME"

