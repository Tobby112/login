package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/parnurzeal/gorequest"
)

const (
	userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/69.0.3497.100 Safari/537.36"
)

var (
	logCookies = false
)

func main() {
	email := ""
	pass := ""
	site := 100
	u := newUser(email, pass, site)
	success, err := u.login()
	if err != nil {
		fmt.Println("login error:", err)
		return
	}
	if !success {
		fmt.Println(email, "login failed")
		return
	}
	fmt.Println(email, "login successfully")
}

type user struct {
	request *gorequest.SuperAgent
	email   string
	pass    string
	site    int
}

func newUser(email, pass string, site int) *user {
	return &user{
		request: gorequest.New(),
		email:   email,
		pass:    pass,
		site:    site,
	}
}

func (u *user) login() (success bool, err error) {
	url := "https://clients.mindbodyonline.com"
	studioid := fmt.Sprintf("studioid=%v", u.site)

	// init session
	_, _, errs := u.request.Get(url+"/ASP/home.asp").
		Query(studioid).
		Set("Accept", "application/json").
		Set("Content-Type", "application/json").
		Set("User-Agent", userAgent).
		End()
	if errs != nil {
		return false, errs[0]
	}
	printCookies(u.request)

	// login
	_, loginBody, errs := u.request.Post(url+"/Login").
		Query(studioid).
		Query("isLibAsync=true").
		Query("isJson=true").
		Set("User-Agent", userAgent).
		Type("form").
		Send(u.loginData()).
		End()
	if errs != nil {
		return false, errs[0]
	}
	printCookies(u.request)

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
		"requiredtxtPassword": u.pass,
	}
	loginDataBytes, _ := json.Marshal(loginDataMap)
	// fmt.Println("login data:", string(loginDataBytes))
	return string(loginDataBytes)
}

func printCookies(r *gorequest.SuperAgent) {
	if !logCookies {
		return
	}
	fmt.Println("==== cookies of", r.Url, "====")
	domain, _ := url.Parse(r.Url)
	cookies := r.Client.Jar.Cookies(domain)
	for _, c := range cookies {
		fmt.Printf("Name:%s Value:%s Expires:%+v MaxAge:%v HttpOnly:%v\n", c.Name, c.Value, c.Expires, c.MaxAge, c.HttpOnly)
	}
}
