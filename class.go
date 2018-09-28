package main

import (
	"database/sql"
	"flag"
	"fmt"
	"html"
	"log"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"
	"github.com/jmoiron/sqlx"
)

var (
	mysqlEndpoint = flag.String("mysqlEndpoint", "", "mysql connection information")
)

type impl struct {
	db *sqlx.DB
}

// new creates the class service
func New() *impl {
	log.Println("connecting mysql:", *mysqlEndpoint)
	db, err := sql.Open("mysql", *mysqlEndpoint)
	if err != nil {
		panic(err)
	}
	if err := db.Ping(); err != nil {
		panic(err)
	}
	im := &impl{
		db: sqlx.NewDb(db, "mysql"),
	}
	go reserver(im)
	return im
}

// NOTE unique key is `Date+Time+NameID+TeacherID`
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

type Reservation struct {
	ID        int64  `db:"id"`
	Email     string `db:"email"`
	Date      string `db:"date"`
	Time      string `db:"time"`
	NameID    string `db:"nameID"`
	TeacherID string `db:"teacherID"`
	IsDone    bool   `db:"isDone"`
}

func (r *Reservation) String() string {
	return fmt.Sprintf("%+v", *r)
}

func reserver(im *impl) {
	reserve := func(email, date, classID string) bool {
		var err error
		defer func() {
			if err != nil {
				log.Println("error", err)
			}
		}()
		var u *user
		u, err = newUser(email)
		if err != nil {
			return false
		}

		success, err := u.login()
		if !success {
			return false
		}
		err = u.reserve(date, classID)
		if err != nil {
			return false
		}
		return true
	}
	log.Println("start reserver")
	loc, _ := time.LoadLocation("Asia/Taipei")
	ticker := time.NewTicker(2 * time.Second)
	for range ticker.C {
		t := time.Now().In(loc)
		if t.Hour() <= 9 || t.Hour() >= 21 {
			continue
		}
		date := t.AddDate(0, 0, 7).Format(classDateFormat)
		rs, _ := im.getReservationsByDate(date)
		if len(rs) == 0 {
			continue
		}

		// get class
		cs, err := im.getClasses(date)
		if err != nil {
			log.Printf("im.getClasses date:%+v, error:%+v\n", date, err)
			continue
		}

		for _, r := range rs {
			log.Println("processing reservation", r)
			c, found := classes(cs).find(r.Date, r.Time, r.NameID, r.TeacherID)
			if !found || c.ID == "" {
				log.Printf("class of date:%s time:%s, nameID:%s, teacherID:%s not fonud\n", r.Date, r.Time, r.NameID, r.TeacherID)
				continue
			}

			if success := reserve(r.Email, r.Date, c.ID); success {
				log.Println("reserve success", r)
				im.reserveDone(r.ID)
			} else {
				log.Println("reserve failed", r)
			}
		}
	}
}

func (im *impl) getReservationsByDate(date string) ([]*Reservation, error) {
	rs := []*Reservation{}
	selectStmt := `SELECT id,email,date,time,nameID,teacherID,isDone FROM ClassReservation WHERE date=? AND isDone=? ORDER BY id asc`
	if err := im.db.Select(&rs, selectStmt, date, false); err != nil {
		log.Println("db.Select error", err, selectStmt, date)
		return nil, err
	}
	return rs, nil
}

func (im *impl) reserve(email, date, time, nameID, teacherID string) error {
	r := &Reservation{
		Email:     email,
		Date:      date,
		Time:      time,
		NameID:    nameID,
		TeacherID: teacherID,
		IsDone:    false,
	}
	log.Println("reserve a class", r)
	insertStmt := `INSERT INTO ClassReservation (email,date,time,nameID,teacherID,isDone) VALUES (:email,:date,:time,:nameID,:teacherID,:isDone)`
	_, err := im.db.NamedExec(insertStmt, r)
	if err != nil {
		log.Println("db.NamedExec error", err, insertStmt, r)
		return err
	}
	return nil
}

func (im *impl) reserveDone(id int64) error {
	log.Println("reserve done", id)
	updateStmt := `UPDATE ClassReservation SET isDone=:isDone WHERE id=:id`
	_, err := im.db.NamedExec(updateStmt, map[string]interface{}{
		"isDone": true,
		"id":     id,
	})
	if err != nil {
		log.Println("db.NamedExec error", err, updateStmt)
		return err
	}
	return nil
}

func (im *impl) cancel(email, date, time, nameID, teacherID string) error {
	return fmt.Errorf("not implemnt")
}

func (im *impl) getClasses(date string) ([]*class, error) {
	r, err := initSession()
	if err != nil {
		log.Println("goquery.NewDocumentFromReader error:", err)
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
		log.Println("goquery.NewDocumentFromReader error:", err)
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
				d := trimSpace((html.UnescapeString(tablecell.Text())))
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
func trimSpace(str string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, str)
}
