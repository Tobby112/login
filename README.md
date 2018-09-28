# Schedule
Get class schedule and reserve class of mindbodyonline
1. govendor sync
2. set `email`, `password` and `pmtRefNo` in main.go `configenceMap`
```
confidenceMap = map[string]confidence{
	"example@gmail.com": {
		"password",
		"pmtRefNo",
	},
}
```
3. go run main.go -mysqlEndpoint="root:@tcp(127.0.0.1:3306)/class?charset=utf8mb4"
* * *
Get Class Schedule
```
GET http://localhost:80/classes?date=09/30/2018
```
Reserve a class
```
POST http://localhost:80/classes
With JSON body
{
	"Email":"example@gmail.com",
	"Date":"09/30/2018",
	"Time":"4:00 pm",
	"NameID":"cid1764796134",
	"TeacherID":"bio100000417"
}
```
