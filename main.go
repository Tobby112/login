package main

import (
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/parnurzeal/gorequest"
	"golang.org/x/crypto/acme/autocert"
)

const (
	domain          = "www.fulfilledpromise.site"
	userAgent       = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/69.0.3497.100 Safari/537.36"
	mindbody        = "https://clients.mindbodyonline.com"
	site            = 831
	classDateFormat = "01/02/2006"
)

var (
	logCookies = false
	// query string of studio ID
	studioID      = fmt.Sprintf("studioid=%v", site)
	confidenceMap = map[string]confidence{
		"example@gmail.com": {
			"password",
			"pmtRefNo",
		},
	}
)

// parameter of POST /classes
type classReservation struct {
	Email     string
	Date      string // classDateFormat
	Time      string
	NameID    string // cid1764796689
	TeacherID string // bio100000157
}

func main() {
	flag.Parse()
	classService := New()
	r := gin.Default()
	r.Use(cors.Default())
	r.GET("/classes", func(c *gin.Context) {
		date, ok := c.GetQuery("date")
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "date not found"})
			return
		}
		_, err := time.Parse(classDateFormat, date)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		classes, err := classService.getClasses(date)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, classes)
	})
	r.POST("/classes", func(c *gin.Context) {
		r := classReservation{}
		if err := c.Bind(&r); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if !userExists(r.Email) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "user not exists"})
			return
		}

		if err := classService.reserve(r.Email, r.Date, r.Time, r.NameID, r.TeacherID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	m := autocert.Manager{
		Prompt:     autocert.AcceptTOS,
		HostPolicy: autocert.HostWhitelist(domain),
		Cache:      autocert.DirCache("/tmp/.cache"),
	}

	go http.ListenAndServe(":http", m.HTTPHandler(nil))
	s := &http.Server{
		Addr: ":https",
		TLSConfig: &tls.Config{
			GetCertificate: m.GetCertificate,
		},
		Handler: r,
	}
	log.Fatal(s.ListenAndServeTLS("", ""))
}

type user struct {
	request    *gorequest.SuperAgent
	email      string
	confidence confidence
}

type confidence struct {
	pass     string
	pmtRefNo string
}

func userExists(email string) bool {
	_, ok := confidenceMap[email]
	return ok
}

func newUser(email string) (*user, error) {
	c, ok := confidenceMap[email]
	if !ok {
		return nil, fmt.Errorf("user:%v not found", email)
	}

	r, err := initSession()
	if err != nil {
		return nil, err
	}

	return &user{
		request:    r,
		email:      email,
		confidence: c,
	}, nil
}

func (u *user) login() (success bool, err error) {
	// login
	_, loginBody, errs := u.request.Post(mindbody+"/Login").
		Query(studioID).
		Query("isLibAsync=true").
		Query("isJson=true").
		Set("User-Agent", userAgent).
		Type("form").
		Send(u.loginData()).
		End()
	if errs != nil {
		return false, errs[0]
	}

	// parse login response
	loginResMap := map[string]interface{}{}
	json.Unmarshal([]byte(loginBody), &loginResMap)
	j := loginResMap["json"].(map[string]interface{})
	return j["success"].(bool), nil
}

func (u *user) loginData() string {
	date := time.Now().Local().Format("2006-01-02")
	loginDataMap := map[string]string{
		"date":                date,
		"classid":             "0",
		"requiredtxtUserName": u.email,
		"requiredtxtPassword": u.confidence.pass,
	}
	loginDataBytes, _ := json.Marshal(loginDataMap)
	// fmt.Println("login data:", string(loginDataBytes))
	return string(loginDataBytes)
}

func (u *user) reserve(date, classID string) error {
	resp, body, errs := u.request.Post(mindbody+"/ASP/res_deb.asp").
		Query(studioID).
		Query("classDate="+date).
		Query("classID="+classID).
		Query("pmtRefNo="+u.confidence.pmtRefNo).
		Query("courseid=&clsLoc=1&typeGroupID=1&recurring=false&wlID=").
		Set("Accept", "application/json").
		Set("Content-Type", "application/json").
		Set("User-Agent", userAgent).
		Type("form").
		Send("").
		End()
	if errs != nil {
		return errs[0]
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("u.reserve falied status:%+v body:%+v", resp.Status, string(body))
	}
	cantReserveStr := "Scheduling is currently closed."
	if strings.Contains(string(body), cantReserveStr) {
		return fmt.Errorf("can't reserve now body:%+v", string(body))
	}

	return nil
}

func initSession() (*gorequest.SuperAgent, error) {
	r := gorequest.New()
	_, _, errs := r.Get(mindbody+"/ASP/home.asp").
		Query(studioID).
		Set("Accept", "application/json").
		Set("Content-Type", "application/json").
		Set("User-Agent", userAgent).
		End()
	if errs != nil {
		return nil, errs[0]
	}
	return r, nil
}

func printCookies(r *gorequest.SuperAgent) {
	if !logCookies {
		return
	}
	log.Println("==== cookies of", r.Url, "====")
	domain, _ := url.Parse(r.Url)
	cookies := r.Client.Jar.Cookies(domain)
	for _, c := range cookies {
		log.Printf("Name:%s Value:%s Expires:%+v MaxAge:%v HttpOnly:%v\n", c.Name, c.Value, c.Expires, c.MaxAge, c.HttpOnly)
	}
}
