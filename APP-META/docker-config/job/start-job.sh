#!/usr/bin/env bash

set -ex

for FILE in config/*.yaml; do
  ./kubectl replace -f $FILE || ./kubectl create -f $FILE
done
