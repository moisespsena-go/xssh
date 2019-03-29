package server

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"log"
)

type DB struct {
	DbName string
	*sql.DB
}

func NewDB(dbName string) *DB {
	return &DB{DbName: dbName}
}

func (s *DB) Init() *DB {
	db, err := sql.Open("sqlite3", s.DbName)
	if err != nil {
		log.Fatal(err)
	}

	sqlStmt := `
create table if not exists users (
	name VARCHAR(50) NOT NULL PRIMARY KEY, 
	update_key bool bool NOT NULL DEFAULT false, 
	is_ap bool NOT NULL DEFAULT false, 
	pub_key text
);

create table if not exists user_ap (
	user VARCHAR(50) NOT NULL, 
	ap VARCHAR(50) NOT NULL, 
	PRIMARY KEY (user, ap)
);

create table if not exists load_balancers (
	ap VARCHAR(50) NOT NULL, 
	service VARCHAR(50) NOT NULL, 
	max_count INT NOT NULL DEFAULT 2, 
	public_addr VARCHAR(255),
	unix_socket BOOL NOT NULL DEFAULT false,
	http_host VARCHAR(255),
	http_path VARCHAR(255) NOT NULL DEFAULT '',
	http_auth_enabled BOOL NOT NULL DEFAULT false,
	http_users TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (ap, service), 
	UNIQUE (public_addr),
	UNIQUE (http_host, http_path)
);
`
	_, err = db.Exec(sqlStmt)
	if err != nil {
		log.Fatalf("%q: %s\n", err, sqlStmt)
	}
	s.DB = db
	return s
}

func (s *DB) Close() error {
	if s.DB != nil {
		return s.DB.Close()
	}
	return nil
}

type HttpUsers struct {
	users map[string]string
}

func (a *HttpUsers) Empty() bool {
	return a.users == nil || len(a.users) == 0
}

func (a *HttpUsers) Set(user, password string) *HttpUsers {
	if a.users == nil {
		a.users = map[string]string{}
	}
	a.users[user] = password
	return a
}

func (a *HttpUsers) Remove(user ...string) *HttpUsers {
	if a.users == nil {
		return a
	}
	for _, user := range user {
		if _, ok := a.users[user]; ok {
			delete(a.users, user)
		}
	}
	return a
}

func (a *HttpUsers) Value() (v driver.Value, err error) {
	if a.users == nil || len(a.users) == 0 {
		return "", nil
	}
	var data []byte
	if data, err = json.MarshalIndent(a.users, "", "  "); err != nil {
		return
	}
	v = data
	return
}

func (a *HttpUsers) Scan(src interface{}) (err error) {
	a.users = map[string]string{}
	if src != nil {
		switch t := src.(type) {
		case []byte:
			if t != nil && len(t) > 0 {
				return json.Unmarshal(t, &a.users)
			}
		case string:
			if t != "" {
				return json.Unmarshal([]byte(t), &a.users)
			}
		}
	}
	return nil
}

func (a *HttpUsers) Match(user, password string) bool {
	if a.users == nil {
		return false
	}
	if pwd, ok := a.users[user]; !ok || password != pwd {
		return false
	}
	return true
}
