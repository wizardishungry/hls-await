#!/bin/sh
# Pre: set $CR_PAT
# https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry
IMAGE_NAME=ghcr.io/wizardishungry/hls-await
docker build -t $IMAGE_NAME .
docker push $IMAGE_NAME:latest