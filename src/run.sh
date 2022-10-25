#!/usr/bin/env bash

(cd certs && bash gen_certs.sh)

docker-compose up "$@"
docker-compose down