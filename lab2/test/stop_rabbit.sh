#!/bin/bash

CONTAINER_NAME="rabbitmq"

docker compose stop "$CONTAINER_NAME"

sleep 10;

docker compose start "$CONTAINER_NAME"

