# Schedule
Get class schedule and reserve class of mindbodyonline
1. go get -u github.com/parnurzeal/gorequest  
   go get -u github.com/gin-gonic/gin  
   go get github.com/gin-contrib/cors  
   go get github.com/PuerkitoBio/goquery
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
GET http://localhost:8080/classes?date=09/30/2018
```
Reserve a class
```
POST http://localhost:8080/classes
With JSON body
{
	"Email":"example@gmail.com",
	"Date":"09/30/2018",
	"Time":"4:00 pm",
	"NameID":"cid1764796134",
	"TeacherID":"bio100000417"
}
```
