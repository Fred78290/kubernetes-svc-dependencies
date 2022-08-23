#!/bin/sh
sudo rm -rf out

VERSION=v1.24.3
REGISTRY=devregistry.aldunelabs.com

make -e REGISTRY=$REGISTRY -e TAG=$VERSION build-in-docker
make -e REGISTRY=$REGISTRY -e TAG=$VERSION push-manifest
