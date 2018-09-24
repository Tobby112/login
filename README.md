# Schedule
Get class schedule and reseve class of mindbodyonline
1. go get -u github.com/parnurzeal/gorequest  
   go get -u github.com/gin-gonic/gin
2. set `email`, `password` and `pmtRefNo` in main.go `configenceMap`
```
confidenceMap = map[string]confidence{
	"example@gmail.com": {
		"password",
		"pmtRefNo",
	},
}
```
3. go run main.go
* * *
Get Class Schedule
```
http://localhost:8080/classes?date=09/30/2018
```
