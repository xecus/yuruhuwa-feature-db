#!/bin/bash

echo "buiding..."
cd cmd/featuredb
go build -o feature-search-db
if [ $? -gt 0 ]; then
    exit 1
fi

echo "run db..."
ddtrace-run ./feature-search-db -hwaddr 00:00:00:00:00:01 -nickname a -mesh :6001 -state_api 0.0.0.0:8001 -feature_api 0.0.0.0:8081 -node_role calc -ipaddress 0.0.0.0 -peer 0.0.0.0:6004 -size_of_init_brick 10000 -strategy goroutine_2 &
ddtrace-run ./feature-search-db -hwaddr 00:00:00:00:00:02 -nickname b -mesh :6002 -state_api 0.0.0.0:8002 -feature_api 0.0.0.0:8082 -node_role calc -ipaddress 0.0.0.0 -peer 0.0.0.0:6004 -size_of_init_brick 10000 -strategy goroutine_2 &
ddtrace-run ./feature-search-db -hwaddr 00:00:00:00:00:04 -nickname d -mesh :6004 -state_api 0.0.0.0:8004 -feature_api 0.0.0.0:8084 -node_role reverseProxy
if [ $? -gt 0 ]; then
    exit 1
fi