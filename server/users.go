package server

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

var ErrStopIteration = errors.New("stop iteration")

type User struct {
	Name      string
	IsAp      bool
	UpdateKey bool
}

func (u User) String() (s string) {
	s += u.Name

	var flags []string

	if u.IsAp {
		flags = append(flags, "AP")
	}

	if u.UpdateKey {
		flags = append(flags, "UPDATE_KEY")
	}

	if len(flags) > 0 {
		s += " [" + strings.Join(flags, ",") + "]"
	}

	return s
}

type Users struct {
	DB *DB
}

func NewUsers(db *DB) *Users {
	users := &Users{DB: db}
	return users
}

func (s *Users) Add(name string, isAp, updateKey bool) (err error) {
	_, err = s.DB.Exec("INSERT INTO users (name, is_ap, update_key) VALUES (?, ?, ?)", name, isAp, updateKey)
	if err != nil {
		return fmt.Errorf("DB Exec failed: %v", err)
	}
	return nil
}

func (s *Users) Remove(name ...string) (removed int64, err error) {
	if len(name) == 0 {
		return
	}

	sqls := "DELETE FROM users WHERE name IN "
	sqls += "(?" + strings.Repeat(",?", len(name)-1) + ")"

	var stmt *sql.Stmt

	if stmt, err = s.DB.Prepare(sqls); err != nil {
		return 0, fmt.Errorf("DB Prepare failed: %v", err)
	}

	defer stmt.Close()

	var namesi []interface{}
	for _, name := range name {
		namesi = append(namesi, name)
	}

	if result, err := stmt.Exec(namesi...); err != nil {
		return 0, fmt.Errorf("DB Exec failed: %v", err)
	} else if removed, err = result.RowsAffected(); err != nil {
		err = fmt.Errorf("DB Get Affcted Rows failed: %v", err)
		return 0, err
	}
	return
}

func (s *Users) SetUpdateKeyFlag(value bool, name ...string) (removed int64, err error) {
	if len(name) == 0 {
		return
	}

	sqls := "UPDATE users SET update_key = ? WHERE name IN "
	sqls += "(?" + strings.Repeat(",?", len(name)-1) + ")"

	var stmt *sql.Stmt

	if stmt, err = s.DB.Prepare(sqls); err != nil {
		return 0, fmt.Errorf("DB Prepare failed: %v", err)
	}

	defer stmt.Close()

	var args = []interface{}{value}
	for _, name := range name {
		args = append(args, name)
	}

	if result, err := stmt.Exec(args...); err != nil {
		return 0, fmt.Errorf("DB Exec failed: %v", err)
	} else if removed, err = result.RowsAffected(); err != nil {
		err = fmt.Errorf("DB Get Affcted Rows failed: %v", err)
		return 0, err
	}
	return
}

func (s *Users) List(isAp bool, cb func(i int, u *User) error, nameMatch ...string) (err error) {
	var (
		where = "is_ap"
		args  = []interface{}{}
	)

	if !isAp {
		where = "NOT " + where
	}

	if len(nameMatch) > 0 && nameMatch[0] != "" {
		args = append(args, "%"+nameMatch[0]+"%")
		where = " AND name ILIKE ?"
	}
	rows, err := s.DB.Query("SELECT name, is_ap, update_key FROM users WHERE "+where+" ORDER BY name ASC", args...)
	if err != nil {
		return fmt.Errorf("DB Prepare failed: %v", err)
	}

	defer rows.Close()

	for i := 1; rows.Next(); i++ {
		var u User
		if err = rows.Scan(&u.Name, &u.IsAp, &u.UpdateKey); err != nil {
			return fmt.Errorf("Scan user %d failed: %v", i, err)
		}
		if err = cb(i, &u); err != nil {
			if err == ErrStopIteration {
				return nil
			}
			return err
		}
	}

	return nil
}

func (s *Users) CheckUser(user, key string) (err error, ok, isAp bool) {
	var (
		updateKey bool
		isAptPtr  *bool
		pubKey    *string
		rows      *sql.Rows
	)

	func() {
		rows, err = s.DB.Query("select update_key, pub_key, is_ap from users where name = ?", user)
		if err != nil {
			return
		}

		defer rows.Close()

		if !rows.Next() {
			err = fmt.Errorf("User %q not found", user)
			return
		}

		if err = rows.Scan(&updateKey, &pubKey, &isAptPtr); err != nil {
			return
		}
	}()

	if err != nil {
		return
	}

	if pubKey == nil || *pubKey != key {
		if updateKey {
			_, err = s.DB.Exec("update users set pub_key = ?, update_key = false where name = ?", key, user)
			if err != nil {
				log.Printf("update %q pub_key failed: %v", user, err)
				return
			}
			ok = true
		}
	} else {
		ok = true
	}

	if ok && isAptPtr != nil {
		isAp = *isAptPtr
	}

	return
}

func (s *Users) IsAp(user string) (ok bool, err error) {
	var (
		rows *sql.Rows
	)

	rows, err = s.DB.Query("select is_ap from users where name = ?", user)
	if err != nil {
		return
	}

	defer rows.Close()

	if !rows.Next() {
		err = fmt.Errorf("User %q not found", user)
		return
	}

	var nbool *bool

	if err = rows.Scan(&nbool); err != nil {
		return
	}

	if nbool != nil {
		ok = *nbool
	}

	return
}
