package server

import (
	"database/sql"
	"log"
)

type DB struct {
	DbName string
	*sql.DB
}

func NewDB(dbName string) *DB {
	return &DB{DbName: dbName}
}

func (s *DB) Init() {
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
	public_addr VARCHAR(255) NOT NULL, 
	PRIMARY KEY (ap, service), 
	UNIQUE (public_addr)
);
`
	_, err = db.Exec(sqlStmt)
	if err != nil {
		log.Fatalf("%q: %s\n", err, sqlStmt)
	}
	s.DB = db
}

func (s *DB) Close() error {
	if s.DB != nil {
		return s.DB.Close()
	}
	return nil
}
