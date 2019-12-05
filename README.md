# feature-serach-db

## How To Use

### Calc Node


./feature-search-db -hwaddr 00:00:00:00:00:01 -nickname a -mesh :6001 -state_api 0.0.0.0:8001 -feature_api 0.0.0.0:8081 -node_role calc -ipaddress 172.31.0.2 -peer 172.30.0.1:6004 -size_of_init_brick 10000
./feature-search-db -hwaddr 00:00:00:00:00:02 -nickname b -mesh :6002 -state_api 0.0.0.0:8002 -feature_api 0.0.0.0:8082 -node_role calc -ipaddress 172.31.0.3 -peer 172.30.0.1:6004 -size_of_init_brick 10000
./feature-search-db -hwaddr 00:00:00:00:00:03 -nickname c -mesh :6003 -state_api 0.0.0.0:8003 -feature_api 0.0.0.0:8083 -node_role calc -ipaddress 172.31.0.4 -peer 172.30.0.1:6004 -size_of_init_brick 10000


### Reverse Proxy Node

./feature-search-db -hwaddr 00:00:00:00:00:04 -nickname d -mesh :6004 -state_api 0.0.0.0:8004 -feature_api 0.0.0.0:8084 -node_role reverseProxy


## How To Test

### State API

curl http://172.31.0.2:8001/

### Feature Search API

http://172.30.0.2:8081/api/v1/searchQuery?featureGroupID=0&calcMode=naive

