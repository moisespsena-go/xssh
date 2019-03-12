package server

import (
	"database/sql"
	"fmt"
	"strings"
)

type LoadBalancer struct {
	Ap, Service string
	PublicAddr  *string
	MaxCount    int

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
	stmt, err := s.DB.Prepare("INSERT INTO load_balancers (ap, service, max_count, public_addr) VALUES (?, ?, ?, ?)")
	if err != nil {
		return fmt.Errorf("DB Prepare failed: %v", err)
	}

	defer stmt.Close()

	if _, err := stmt.Exec(ap, service, maxCount, publicAddr); err != nil {
		return fmt.Errorf("DB Exec failed: %v", err)
	}
	return nil
}

func (s *LoadBalancers) Remove(ap string, name ...string) (removed int64, err error) {
	if len(name) == 0 {
		return
	}

	sqls := "DELETE FROM load_balancers WHERE ap = ? AND service IN "
	sqls += "(?" + strings.Repeat(",?", len(name)-1) + ")"

	var stmt *sql.Stmt

	if stmt, err = s.DB.Prepare(sqls); err != nil {
		return 0, fmt.Errorf("DB Prepare failed: %v", err)
	}

	defer stmt.Close()

	var args = []interface{}{ap}
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

func (s *Users) SetMaxCount(value int, ap string, name ...string) (removed int64, err error) {
	if len(name) == 0 {
		return
	}

	sqls := "UPDATE load_balancers SET max_count = ? WHERE ap = ? AND name IN "
	sqls += "(?" + strings.Repeat(",?", len(name)-1) + ")"

	var stmt *sql.Stmt

	if stmt, err = s.DB.Prepare(sqls); err != nil {
		return 0, fmt.Errorf("DB Prepare failed: %v", err)
	}

	defer stmt.Close()

	var args = []interface{}{value, ap}
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

func (s *Users) SetPublicAddr(ap, name, value string) (removed int64, err error) {
	sqls := "UPDATE load_balancers SET public_addr = ? WHERE ap = ? AND name = ? "

	var stmt *sql.Stmt

	if stmt, err = s.DB.Prepare(sqls); err != nil {
		return 0, fmt.Errorf("DB Prepare failed: %v", err)
	}

	defer stmt.Close()

	if result, err := stmt.Exec(ap, name, value); err != nil {
		return 0, fmt.Errorf("DB Exec failed: %v", err)
	} else if removed, err = result.RowsAffected(); err != nil {
		err = fmt.Errorf("DB Get Affcted Rows failed: %v", err)
		return 0, err
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

	rows, err := s.DB.Query("SELECT ap, service, max_count, public_addr FROM load_balancers WHERE "+
		strings.Join(where, " AND ")+" ORDER BY ap, service ASC", args...)
	if err != nil {
		return fmt.Errorf("DB Prepare failed: %v", err)
	}

	for i := 1; rows.Next(); i++ {
		var lb LoadBalancer
		if err = rows.Scan(&lb.Ap, &lb.Service, &lb.MaxCount, &lb.PublicAddr); err != nil {
			return fmt.Errorf("Scan Load Balancer %d failed: %v", i, err)
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
