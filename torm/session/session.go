package session

import (
	"database/sql"
	"reflect"
	"strings"

	"betterrest/torm/log"

	_ "github.com/lib/pq"
)

// New creates a instance of Session
func Q(db *sql.DB) *Session {
	return &Session{
		db: db,
	}
}

type Session struct {
	db      *sql.DB
	sql     strings.Builder
	sqlVars []interface{}
}

func (s *Session) DB() *sql.DB {
	return s.db
}

// Clear initialize the state of a session
func (s *Session) Clear() {
	s.sql.Reset()
	s.sqlVars = nil
}

func (s *Session) Exec(query string, args ...interface{}) (result sql.Result, err error) {
	defer s.Clear()
	log.Info(s.sql.String(), s.sqlVars)
	if result, err = s.DB().Exec(s.sql.String(), s.sqlVars...); err != nil {
		log.Error(err)
	}
	return
}

// Raw appends sql and sqlVars
func (s *Session) Raw(sql string, values ...interface{}) *Session {
	s.sql.WriteString(sql)
	s.sql.WriteString(" ")
	s.sqlVars = append(s.sqlVars, values...)
	return s
}

// func foo() {
// 	var (
// 		id   int
// 		name string
// 	)
// 	rows, err := db.Query("select id, name from users where id = ?", 1)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	defer rows.Close()
// 	for rows.Next() {
// 		err := rows.Scan(&id, &name)
// 		if err != nil {
// 			log.Fatal(err)
// 		}
// 		log.Println(id, name)
// 	}
// 	err = rows.Err()
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// }

// Find gets all eligible records
func (s *Session) Find(values interface{}) error {
	destSlice := reflect.Indirect(reflect.ValueOf(values))
	destType := destSlice.Type().Elem()

	table := s.Model(reflect.New(destType).Elem().Interface()).RefTable()

	rows, err := s.Raw(sql, vars...).QueryRows()
	if err != nil {
		return err
	}

	for rows.Next() {
		dest := reflect.New(destType).Elem()
		var values []interface{}
		for _, name := range table.FieldNames {
			values = append(values, dest.FieldByName(name).Addr().Interface())
		}
		if err := rows.Scan(values...); err != nil {
			return err
		}
		destSlice.Set(reflect.Append(destSlice, dest))
	}
	return rows.Close()
}

// QueryRow gets a record from db
func (s *Session) QueryRow() *sql.Row {
	defer s.Clear()
	log.Info(s.sql.String(), s.sqlVars)
	return s.DB().QueryRow(s.sql.String(), s.sqlVars...)
}

// QueryRows gets a list of records from db
func (s *Session) QueryRows() (rows *sql.Rows, err error) {
	defer s.Clear()
	log.Info(s.sql.String(), s.sqlVars)
	if rows, err = s.DB().Query(s.sql.String(), s.sqlVars...); err != nil {
		log.Error(err)
	}
	return
}

// func main() {
// 	db, err := sql.Open("postgres", "user=astaxie password=astaxie dbname=test sslmode=disable")
// 	checkErr(err)

// 	//插入資料
// 	stmt, err := db.Prepare("INSERT INTO userinfo(username,department,created) VALUES($1,$2,$3) RETURNING uid")
// 	checkErr(err)

// 	res, err := stmt.Exec("astaxie", "研發部門", "2012-12-09")
// 	checkErr(err)

// 	//pg 不支援這個函式，因為他沒有類似 MySQL 的自增 ID
// 	// id, err := res.LastInsertId()
// 	// checkErr(err)
// 	// fmt.Println(id)

// 	var lastInsertId int
// 	err = db.QueryRow("INSERT INTO userinfo(username,departname,created) VALUES($1,$2,$3) returning uid;", "astaxie", "研發部門", "2012-12-09").Scan(&lastInsertId)
// 	checkErr(err)
// 	fmt.Println("最後插入 id =", lastInsertId)

// 	//更新資料
// 	stmt, err = db.Prepare("update userinfo set username=$1 where uid=$2")
// 	checkErr(err)

// 	res, err = stmt.Exec("astaxieupdate", 1)
// 	checkErr(err)

// 	affect, err := res.RowsAffected()
// 	checkErr(err)

// 	fmt.Println(affect)

// 	//查詢資料
// 	rows, err := db.Query("SELECT * FROM userinfo")
// 	checkErr(err)

// 	for rows.Next() {
// 		var uid int
// 		var username string
// 		var department string
// 		var created string
// 		err = rows.Scan(&uid, &username, &department, &created)
// 		checkErr(err)
// 		fmt.Println(uid)
// 		fmt.Println(username)
// 		fmt.Println(department)
// 		fmt.Println(created)
// 	}

// 	//刪除資料
// 	stmt, err = db.Prepare("delete from userinfo where uid=$1")
// 	checkErr(err)

// 	res, err = stmt.Exec(1)
// 	checkErr(err)

// 	affect, err = res.RowsAffected()
// 	checkErr(err)

// 	fmt.Println(affect)

// 	db.Close()

// }

// func checkErr(err error) {
// 	if err != nil {
// 		panic(err)
// 	}
// }
