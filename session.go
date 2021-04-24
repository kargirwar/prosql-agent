/* One session corresponds to one tab in the browser. If the tab is duplicated
they will share the session. Each session can execute multiple queries. Each
query will be associated with a cursor. A cursor can be queried untill all
rows have been read*/
/* There is one goroutine corresponding to each session. Similarly there
is one goroutine per cursor */

package main

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/dchest/uniuri"
	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
)

type DSN struct {
	User string
	Pass string
	Host string
	Port string
	Db   string
}

func (dsn *DSN) String() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", dsn.User, dsn.Pass, dsn.Host, dsn.Port, dsn.Db)
}

//a utility function to load multiple DSNs from files. Mostly used for testing
func parseDSN() []*DSN {
	var dsn []*DSN
	for i := 1; ; i++ {
		f := ".dsn" + strconv.Itoa(i)
		err := godotenv.Overload(f)
		if err != nil {
			break
		}
		dsn = append(dsn, &DSN{
			User: os.Getenv("USR"),
			Pass: os.Getenv("PASS"),
			Host: os.Getenv("HOST"),
			Port: os.Getenv("PORT"),
			Db:   os.Getenv("DB"),
		})
	}

	return dsn
}

//==============================================================//
//          session structs and methods
//==============================================================//
type session struct {
	id          string
	db          string
	pool        *sql.DB
	accessTime  time.Time
	cursorStore *cursors
	mutex       sync.Mutex
}

func (s *session) setAccessTime() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.accessTime = time.Now()
}

func (s *session) getAccessTime() time.Time {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return s.accessTime
}

func (s *session) cleanup() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	log.Printf("%s: Handling CMD_CLEANUP\n", s.id)

	keys := s.cursorStore.getKeys()
	for _, k := range keys {
		log.Printf("%s: Cleaning up cursor: %s\n", s.id, k)
		c, err := s.cursorStore.get(k)
		if err != nil {
			log.Printf("%s: Skipping: %s\n", s.id, k)
			continue
		}
		c.cancel()

		log.Printf("%s: Cleanup done for cursor: %s\n", s.id, k)
		s.cursorStore.clear(k)
		log.Printf("%s: Clear done for cursor: %s\n", s.id, k)
	}

	log.Printf("%s: Done CMD_CLEANUP\n", s.id)
}

//==============================================================//
//          session structs and methods end
//==============================================================//

type sessions struct {
	store map[string]*session
	mutex sync.Mutex
}

func (ps *sessions) set(sid string, s *session) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	ps.store[sid] = s
}

func (ps *sessions) get(sid string) (*session, error) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	s, present := ps.store[sid]
	if !present {
		return nil, errors.New(ERR_INVALID_SESSION_ID)
	}

	return s, nil
}

func (ps *sessions) getKeys() []string {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	keys := make([]string, len(ps.store))

	i := 0
	for k := range ps.store {
		keys[i] = k
		i++
	}

	return keys
}

func (ps *sessions) clear(k string) {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	s, present := ps.store[k]
	if present {
		if s.pool != nil {
			s.pool.Close()
		}

		delete(ps.store, k)
	}
}

//==============================================================//
//          globals
//==============================================================//
var sessionStore *sessions
var ticker *time.Ticker

func init() {
	store := make(map[string]*session)
	sessionStore = &sessions{
		store: store,
	}
	ticker = time.NewTicker(SESSION_CLEANUP_INTERVAL)
	go cleanupSessions()
}

//for all sessions whose accesstime is older than SESSION_CLEANUP_INTERVAL,
//clean up active cursors if any and then delete the session itself
func cleanupSessions() {
	for {
		select {
		case <-ticker.C:
			log.Printf("Starting session cleanup")
			keys := sessionStore.getKeys()
			for _, k := range keys {
				log.Printf("%s: Checking session\n", k)
				s, err := sessionStore.get(k)
				if err != nil {
					log.Printf("%s: Skipping session\n", k)
					continue
				}

				now := time.Now()
				if now.Sub(s.getAccessTime()) > SESSION_CLEANUP_INTERVAL {
					log.Printf("%s: Cleaning up session\n", k)
					s.cleanup()
					log.Printf("%s: Cleanup done\n", k)
					sessionStore.clear(k)
					log.Printf("%s: Clear done\n", k)
				}
			}

			log.Printf("Done session cleanup")
		}
	}
}

//==============================================================//
//         External Interface
//==============================================================//

func NewSession(dbtype string, dsn *DSN) (string, error) {
	pool, err := sql.Open(dbtype, dsn.String())

	if err != nil {
		return "", err
	}

	var s session
	s.pool = pool
	s.db = dsn.Db
	s.accessTime = time.Now()
	s.id = uniuri.New()
	s.cursorStore = NewCursorStore()

	sessionStore.set(s.id, &s)
	return s.id, nil
}

//execute a query and create a cursor for the results
//results must be retrieved by calling fetch later with the cursor id
func Execute(sid string, query string) (string, error) {
	s, err := sessionStore.get(sid)
	if err != nil {
		return "", err
	}

	log.Printf("%s: Handling CMD_EXECUTE for: %s\n", s.id, query)

	c, err := NewCursor(s.pool, s.db)
	if err != nil {
		return "", err
	}

	s.cursorStore.set(c.id, c)
	c.execute(s, query)

	log.Printf("%s: Done CMD_EXECUTE for: %s\n", s.id, query)

	return c.id, nil
}

//fetch n rows from session sid using cursor cid
func Fetch(sid string, cid string, n int) (*[][]string, bool, error) {
	s, err := sessionStore.get(sid)
	if err != nil {
		return nil, false, err
	}

	c, err := s.cursorStore.get(cid)
	if err != nil {
		return nil, false, err
	}

	return c.fetch(n)
}

//cancel a running query
func Cancel(sid string, cid string) error {
	s, err := sessionStore.get(sid)
	if err != nil {
		return err
	}

	c, err := s.cursorStore.get(cid)
	if err != nil {
		return err
	}

	c.cancel()

	return nil
}

//==============================================================//
//         External Interface End
//==============================================================//
