package server

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/go-errors/errors"
)

type LoadBalancer struct {
	Ap, Service string
	PublicAddr  *string
	HttpHost    *string
	HttpPath    string
	MaxCount    int
	UnixSocket  bool

	*Nodes
}

type LoadBalancerFilter struct {
	Ap       string
	Services []string
}

type LoadBalancers struct {
	*DB
}

func NewLoadBalancers(DB *DB) *LoadBalancers {
	return &LoadBalancers{DB: DB}
}

func (s *LoadBalancers) Add(ap, service string, maxCount int, publicAddr string) (err error) {
	_, err = s.DB.Exec("INSERT INTO load_balancers (ap, service, max_count, public_addr) VALUES (?, ?, ?, ?)",
		ap, service, maxCount, publicAddr)
	if err != nil {
		return fmt.Errorf("DB exec failed: %v", err)
	}
	return nil
}

func (s *LoadBalancers) Remove(ap string, name ...string) (removed int64, err error) {
	if len(name) == 0 {
		return
	}

	sqls := "DELETE FROM load_balancers WHERE ap = ? AND service IN "
	sqls += "(?" + strings.Repeat(",?", len(name)-1) + ")"

	var (
		args = []interface{}{ap}
		res  sql.Result
	)
	for _, name := range name {
		args = append(args, name)
	}

	if res, err = s.DB.Exec(sqls, args...); err != nil {
		return 0, fmt.Errorf("DB exec failed: %v", err)
	} else if removed, err = res.RowsAffected(); err != nil {
		return 0, fmt.Errorf("DB get affected rows failed: %v", err)
	}
	return
}

func (s *LoadBalancers) SetHttpHost(ap, name string, value *string) (err error) {
	return s.Set(ap, name, "http_host", value)
}

func (s *LoadBalancers) SetHttpPath(ap, name string, value string) (err error) {
	return s.Set(ap, name, "http_path", value)
}

func (s *LoadBalancers) SetHttpAuthEnabled(ap, name string, value bool) (err error) {
	return s.Set(ap, name, "http_auth_enabled", value)
}

func (s *LoadBalancers) HttpUserAdd(ap, name, username, pasword string) (err error) {
	var users HttpUsers
	if users, _, err = s.GetUsers(ap, name); err != nil {
		return
	}
	return s.Set(ap, name, "http_auth_enabled", users.Set(username, pasword))
}

func (s *LoadBalancers) HttpUserRemove(ap, name string, username ...string) (err error) {
	if len(username) == 0 {
		return
	}
	var users HttpUsers
	if users, _, err = s.GetUsers(ap, name); err != nil {
		return
	}
	return s.Set(ap, name, "http_auth_enabled", users.Remove(username...))
}

func (s *LoadBalancers) SetUnixSocket(ap, name string, value bool) (err error) {
	return s.Set(ap, name, "unix_socket", value)
}

func (s *LoadBalancers) SetMaxCount(ap, name string, value int) (err error) {
	return s.Set(ap, name, "max_xount", value)
}

func (s *LoadBalancers) SetPublicAddr(ap, name, value string) (err error) {
	return s.Set(ap, name, "public_addr", value)
}

func (s *LoadBalancers) Set(ap, name, field string, value interface{}) (err error) {
	sqls := "UPDATE load_balancers SET " + field + " = ? WHERE ap = ? AND name = ? "

	var stmt *sql.Stmt

	if stmt, err = s.DB.Prepare(sqls); err != nil {
		return fmt.Errorf("DB Prepare failed: %v", err)
	}

	defer stmt.Close()

	if result, err := stmt.Exec(ap, name, value); err != nil {
		return fmt.Errorf("DB Exec failed: %v", err)
	} else if af, err := result.RowsAffected(); err != nil {
		err = fmt.Errorf("DB Get Affected Rows failed: %v", err)
		return err
	} else if af == 0 {
		err = errors.New("Recorde not found")
	}
	return
}

func (s *LoadBalancers) List(cb func(i int, lb *LoadBalancer) error, filter *LoadBalancerFilter) (err error) {
	var (
		where []string
		args  = []interface{}{}
	)

	if filter != nil {
		if filter.Ap != "" {
			where = append(where, "ap = ?")
			args = append(args, filter.Ap)
		}

		if len(filter.Services) > 0 {
			where = append(where, "service IN (?"+strings.Repeat(",?", len(filter.Services)-1)+")")
			for _, service := range filter.Services {
				args = append(args, service)
			}
		}
	}

	rows, err := s.DB.Query("SELECT ap, service, max_count, public_addr, http_host, http_path FROM load_balancers WHERE "+
		strings.Join(where, " AND ")+" ORDER BY ap, service ASC", args...)
	if err != nil {
		return fmt.Errorf("DB Query failed: %v", err)
	}

	defer rows.Close()

	for i := 1; rows.Next(); i++ {
		var lb LoadBalancer
		if err = rows.Scan(&lb.Ap, &lb.Service, &lb.MaxCount, &lb.PublicAddr, &lb.HttpHost, &lb.HttpPath); err != nil {
			return fmt.Errorf("Scan Load Balancer %d failed: %v", i, err)
		}
		if lb.HttpPath != "" {
			lb.HttpPath = cleanPth(lb.HttpPath)
		}
		if err = cb(i, &lb); err != nil {
			if err == ErrStopIteration {
				return nil
			}
			return err
		}
	}

	return nil
}

func (s *LoadBalancers) Get(ap, service string) (balancer *LoadBalancer, err error) {
	err = s.List(func(i int, lb *LoadBalancer) error {
		balancer = lb
		return nil
	}, &LoadBalancerFilter{Ap: ap, Services: []string{service}})
	return
}

func (s *LoadBalancers) GetUsers(ap, service string) (users HttpUsers, enabled bool, err error) {
	var rows *sql.Rows
	rows, err = s.DB.Query("SELECT http_auth_enabled, http_users FROM load_balancers WHERE ap = ? AND service = ?", ap, service)
	if err != nil {
		err = fmt.Errorf("DB Query failed: %v", err)
		return
	}

	defer rows.Close()
	if rows.Next() {
		if err = rows.Scan(&enabled, &users); err != nil {
			err = fmt.Errorf("Scan auth failed: %v", err)
			return
		}
	}
	return
}
