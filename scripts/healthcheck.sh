#!/bin/sh
curl -f http://localhost:8080/healthz || exit 1
