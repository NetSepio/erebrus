#!/bin/bash

VERSION=$(cat src/version.txt)
sed -i 's/^version:.*/version: '$VERSION'/' snapcraft.yaml
snapcraft --debug --verbose
