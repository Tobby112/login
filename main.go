package main

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/parnurzeal/gorequest"
)

const (
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

type classReservation struct {
	Email     string
	Date      string // classDateFormat
	Time      string
	NameID    string // cid1764796689
	TeacherID string // bio100000157
}

func main() {
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
		classes, err := getClasses(date)
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
		u, err := newUser(r.Email)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		success, err := u.login()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if !success {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if err := u.reserve(r.Date, r.Time, r.NameID, r.TeacherID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	r.Run("127.0.0.1:8080")
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

func (u *user) reserve(date, time, nameID, teacherID string) error {
	cs, err := getClasses(date)
	if err != nil {
		return err
	}
	c, found := classes(cs).find(date, time, nameID, teacherID)
	if !found {
		return fmt.Errorf("class of date:%s time:%s, nameID:%s, teacherID:%s not fonud", date, time, nameID, teacherID)
	}
	_, _, errs := u.request.Post(mindbody+"/ASP/res_deb.asp").
		Query(studioID).
		Query("classID="+c.ID).
		Query("classDate="+date).
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

// NOTE unique key is `Date+NameID+TeacherID`
type class struct {
	Index     int
	ID        string
	Date      string // classDateFormat
	Time      string
	Name      string
	NameID    string // cid1764796689
	Teacher   string
	TeacherID string // bio100000157
	Location  string
	Duration  string
}

type classes []*class

func (cs classes) find(date, time, nameID, teacherID string) (*class, bool) {
	for _, c := range cs {
		if c.Date == date &&
			c.Time == time &&
			c.NameID == nameID &&
			c.TeacherID == teacherID {
			return c, true
		}
	}
	return nil, false
}

func getClasses(date string) ([]*class, error) {
	r, err := initSession()
	if err != nil {
		fmt.Println("goquery.NewDocumentFromReader error:", err)
		return nil, err
	}

	// get classes
	_, body, errs := r.Get(mindbody+"/classic/mainclass").
		Query(studioID).
		Query("date="+date).
		Query("tg=&vt=&lvl=&stype=&view=&trn=0&page=&catid=&prodid=&classid=0&prodGroupId=&sSU=&optForwardingLink=&qParam=&justloggedin=&nLgIn=&pMode=0&loc=1").
		Set("Accept", "application/json").
		Set("Content-Type", "application/json").
		Set("User-Agent", userAgent).
		End()
	if errs != nil {
		return nil, errs[0]
	}
	// parse classSchedule table in the html body to classes
	classes := []*class{}
	dom, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil {
		fmt.Println("goquery.NewDocumentFromReader error:", err)
		return nil, err
	}
	// get schedule table
	dom.Find("table#classSchedule-mainTable").Each(func(_ int, selection *goquery.Selection) {
		// get table rows
		selection.Find("tr").Each(func(indextr int, rowhtml *goquery.Selection) {
			// skip idx-0:header and idx-1:date
			if indextr < 2 {
				return
			}
			// get all table data of a row
			c := &class{
				Date:  date,
				Index: len(classes),
			}
			rowhtml.Find("td").Each(func(indextd int, tablecell *goquery.Selection) {
				d := spaceMap((html.UnescapeString(tablecell.Text())))
				switch indextd {
				case 0: // time
					c.Time = d
				case 1: // sign up button and reserved/open count
					onclick := tablecell.Find("input").AttrOr("onclick", "")
					i := strings.Index(onclick, "/ASP/res_a.asp?")
					if i == -1 { // not found resa
						break
					}
					queries, _ := url.ParseQuery(onclick[i:])
					classIDs, ok := queries["classId"]
					if !ok {
						break
					}
					c.ID = classIDs[0]
				case 2: // class desc
					c.Name = d
					c.NameID = tablecell.Find("a").AttrOr("name", "")
				case 3: // teacher
					c.Teacher = d
					c.TeacherID = tablecell.Find("a").AttrOr("name", "")
				case 4: // assist
				case 5: // location
					c.Location = d
				case 6: // duraction
					c.Duration = d
				}

			})
			classes = append(classes, c)
		})
	})
	return classes, nil
}

// cutout all spaces in the string
func spaceMap(str string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, str)
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
