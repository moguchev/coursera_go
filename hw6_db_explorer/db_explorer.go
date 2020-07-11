package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/go-sql-driver/mysql"
)

type repository struct {
	db *sql.DB
}

func NewDbExplorer(db *sql.DB) (http.Handler, error) {

	r := repository{db: db}

	router := http.NewServeMux()
	router.HandleFunc("/", handler(r))

	return router, nil
}

func handleGetRequest(w http.ResponseWriter, r *http.Request, repo *repository) {
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		fmt.Println(err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	path := r.URL.Path
	if len(path) == 1 {
		result, err := tableList(repo.db)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Write(ResponseToBytes(result, "tables"))
	} else {
		params := strings.Split(path, "/")
		table := params[1]
		switch len(params) {
		case 2:
			result, err := findAllRows(repo.db, query, table)
			if err != nil {
				if sqlError, e := (err).(*mysql.MySQLError); e && sqlError.Number == 1146 {
					w.WriteHeader(http.StatusNotFound)
					w.Write(ResponseError("unknown table"))
				} else {
					w.WriteHeader(http.StatusInternalServerError)
				}
				return
			}
			w.Write(ResponseToBytes(result, "records"))
		case 3:
			id := params[2]
			result, err := findById(repo.db, table, id)
			if err != nil {
				if err.Error() == "record not found" {
					w.WriteHeader(http.StatusNotFound)
					w.Write(ResponseError(err.Error()))
				} else {
					w.WriteHeader(http.StatusInternalServerError)
				}
				return
			}
			w.Write(ResponseToBytes(result, "record"))
		}
	}
}

func handlePostRequest(w http.ResponseWriter, r *http.Request, repo *repository) {
	path := r.URL.Path
	params := strings.Split(path, "/")
	if len(params) < 3 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	table := params[1]
	id := params[2]
	body := r.Body
	result, _, err := createUpdateRow(repo.db, table, body, id)
	if err != nil {
		if strings.Contains(err.Error(), "invalid type") {
			w.WriteHeader(http.StatusBadRequest)
			w.Write(ResponseError(err.Error()))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(ResponseToBytes(result, "updated"))
}

func handlePutRequest(w http.ResponseWriter, r *http.Request, repo *repository) {
	path := r.URL.Path
	params := strings.Split(path, "/")
	if len(params) < 3 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	table := params[1]
	id := params[2]
	body := r.Body
	result, key, err := createUpdateRow(repo.db, table, body, id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(ResponseToBytes(result, key))
}

func handleDeleteRequest(w http.ResponseWriter, r *http.Request, repo *repository) {
	path := r.URL.Path
	params := strings.Split(path, "/")
	if len(params) < 3 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	table := params[1]
	id := params[2]
	result, err := deleteRow(repo.db, table, id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write(ResponseError(err.Error()))
		return
	}
	w.Write(ResponseToBytes(result, "deleted"))
}

func handler(repo repository) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleGetRequest(w, r, &repo)
		case http.MethodPost:
			handlePostRequest(w, r, &repo)
		case http.MethodPut:
			handlePutRequest(w, r, &repo)
		case http.MethodDelete:
			handleDeleteRequest(w, r, &repo)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}
}

func tableList(db *sql.DB) ([]string, error) {

	var tables []string
	rows, ok := db.Query("SHOW TABLES;")
	if ok != nil {
		return nil, ok
	}
	defer rows.Close()

	for rows.Next() {
		tableName := ""
		ok := rows.Scan(&tableName)
		if ok != nil {
			return nil, ok
		}
		tables = append(tables, tableName)
	}

	return tables, nil
}

func findAllRows(db *sql.DB, query url.Values, table string) ([]map[string]interface{}, error) {
	//https://forum.golangbridge.org/t/database-rows-scan-unknown-number-of-columns-json/7378/15
	var objects []map[string]interface{}

	tableName := table

	limit, e := strconv.Atoi(query.Get("limit"))
	if e != nil && limit < 0 {
		return nil, e
	}

	offset, e := strconv.Atoi(query.Get("offset"))
	if e != nil && offset < 0 {
		return nil, e
	}

	var rows *sql.Rows
	var ok error
	if limit == 0 && offset == 0 {
		rows, ok = db.Query(fmt.Sprintf("SELECT * FROM %s", tableName))
	} else {
		rows, ok = db.Query(fmt.Sprintf("SELECT * FROM %s limit ? offset ?", tableName), limit, offset)
	}

	if ok != nil {
		return nil, ok
	}
	defer rows.Close()

	for rows.Next() {
		columnTypes, _ := rows.ColumnTypes()

		values := make([]interface{}, len(columnTypes))
		object := map[string]interface{}{}
		for i, column := range columnTypes {

			v := reflect.New(column.ScanType()).Interface()
			switch v.(type) {
			case *[]uint8:
				v = new(*string)
			case *int32:
				v = new(*int32)
			case *sql.RawBytes:
				v = new(*string)
			default:
				values[i] = v
			}

			object[column.Name()] = v
			values[i] = v
		}

		ok := rows.Scan(values...)
		if ok != nil {
			return nil, ok
		}

		objects = append(objects, object)
	}

	return objects, nil
}

func findById(db *sql.DB, table string, id string) (map[string]interface{}, error) {

	var objects map[string]interface{}

	key, ok := keyField(db, table)
	if ok != nil {
		return nil, ok
	}

	rows, ok := db.Query(fmt.Sprintf("SELECT * FROM %s WHERE %s = ?", table, key), id)
	if ok != nil {
		return nil, ok
	}
	defer rows.Close()

	if rows.Next() {
		columnTypes, _ := rows.ColumnTypes()

		values := make([]interface{}, len(columnTypes))
		object := map[string]interface{}{}
		for i, column := range columnTypes {

			v := reflect.New(column.ScanType()).Interface()
			switch v.(type) {
			case *[]uint8:
				v = new(*string)
			case *int32:
				v = new(*int32)
			case *sql.RawBytes:
				v = new(*string)
			default:
				values[i] = v
			}

			object[column.Name()] = v
			values[i] = v
		}

		ok := rows.Scan(values...)
		if ok != nil {
			return nil, ok
		}

		objects = object

		return objects, nil
	}

	return nil, errors.New("record not found")
}

func createUpdateRow(db *sql.DB, table string, body io.ReadCloser, id string) (int, string, error) {

	rows, ok := ioutil.ReadAll(body)
	if ok != nil {
		return -1, "", ok
	}
	defer body.Close()

	bodyValues := make(map[string]interface{})
	ok = json.Unmarshal(rows, &bodyValues)
	if ok != nil {
		return -1, "", ok
	}

	var key string
	key, ok = validateFields(db, table, &bodyValues, id)
	if ok != nil {
		return -1, "", ok
	}

	// POST
	if id == "" {

		var fields, placeholders string
		var values []interface{}
		for k, v := range bodyValues {

			if k == "id" {
				continue
			}

			if len(fields) > 0 {
				fields += ","
				placeholders += ","
			}
			fields += "`" + k + "`"
			values = append(values, v)
			placeholders += "?"
		}

		result, ok := db.Exec(fmt.Sprintf("INSERT INTO %s (%s) VALUES(%s)", table, fields, placeholders), values...)
		if ok != nil {
			return -1, "", ok
		}

		created, ok := result.LastInsertId()
		if ok != nil {
			return -1, "", ok
		}

		return int(created), key, nil

	}
	// PUT
	var fields string
	var values []interface{}
	for k, v := range bodyValues {

		if k == key {
			m := "field " + key + " have invalid type"
			return -1, "", errors.New(m)
		}

		if len(fields) > 0 {
			fields += ","
		}
		fields += "`" + k + "` = ?"
		values = append(values, v)
	}

	values = append(values, id)

	result, ok := db.Exec(fmt.Sprintf("UPDATE %s SET %s WHERE %s = ?", table, fields, key), values...)
	if ok != nil {
		return -1, "", ok
	}

	updated, ok := result.RowsAffected()
	if ok != nil {
		return -1, "", ok
	}

	return int(updated), key, nil
}

func deleteRow(db *sql.DB, table string, id string) (int, error) {

	key, ok := keyField(db, table)
	if ok != nil {
		return -1, ok
	}

	result, ok := db.Exec(fmt.Sprintf("DELETE FROM %s WHERE %s = ?", table, key), id)
	if ok != nil {
		return -1, ok
	}

	deleted, ok := result.RowsAffected()
	if ok != nil {
		return -1, ok
	}

	return int(deleted), nil
}

func keyField(db *sql.DB, tableName string) (string, error) {
	var objects []map[string]interface{}

	rows, ok := db.Query(fmt.Sprintf("SHOW FULL COLUMNS FROM %s", tableName))
	if ok != nil {
		return "", ok
	}
	defer rows.Close()

	for rows.Next() {
		columnTypes, _ := rows.ColumnTypes()

		values := make([]interface{}, len(columnTypes))
		object := map[string]interface{}{}
		for i, column := range columnTypes {

			v := reflect.New(column.ScanType()).Interface()
			switch v.(type) {
			case *[]uint8:
				v = new(*string)
			case *int32:
				v = new(*int32)
			case *sql.RawBytes:
				v = new(*string)
			default:
				values[i] = v
			}

			object[column.Name()] = v
			values[i] = v
		}

		err := rows.Scan(values...)
		if err != nil {
			return "", err
		}

		objects = append(objects, object)
	}

	var key string
	for _, v := range objects {
		k := **v["Key"].(**string)
		if strings.Contains(k, "PRI") {
			s := **v["Field"].(**string)
			key = s
			break
		}
	}

	return key, nil
}

func validateFields(db *sql.DB, tableName string, values *map[string]interface{}, id string) (string, error) {

	var objects []map[string]interface{}

	rows, ok := db.Query(fmt.Sprintf("SHOW FULL COLUMNS FROM %s", tableName))
	if ok != nil {
		return "", ok
	}
	defer rows.Close()

	for rows.Next() {
		columnTypes, _ := rows.ColumnTypes()

		values := make([]interface{}, len(columnTypes))
		object := map[string]interface{}{}
		for i, column := range columnTypes {

			v := reflect.New(column.ScanType()).Interface()
			switch v.(type) {
			case *[]uint8:
				v = new(*string)
			case *int32:
				v = new(*int32)
			case *sql.RawBytes:
				v = new(*string)
			default:
				values[i] = v
			}

			object[column.Name()] = v
			values[i] = v
		}

		ok := rows.Scan(values...)
		if ok != nil {
			return "", ok
		}

		objects = append(objects, object)
	}

	valid := true
	var field string
	var key string
	columns := make(map[string]interface{})
	defaults := make(map[string]interface{})
	for i, val := range *values {
		for _, v := range objects {
			s := **v["Field"].(**string)
			t := **v["Type"].(**string)
			n := **v["Null"].(**string)
			k := **v["Key"].(**string)
			if strings.Contains(k, "PRI") {
				key = s
			}
			if strings.Contains(n, "NO") {
				defaults[s] = s
			}
			if s == i {
				switch val.(type) {
				case float64:
					valid = strings.Contains(t, "int")
				case int:
					valid = strings.Contains(t, "int")
				case string:
					valid = strings.Contains(t, "varchar") || strings.Contains(t, "text")
				default:
					valid = strings.Contains(n, "YES")
				}
			}
			if !valid {
				field = s
				break
			}
			columns[s] = s
		}
		if !valid {
			break
		}
	}

	unknows := []string{}
	for k := range *values {
		if _, ok := columns[k]; !ok {
			unknows = append(unknows, k)
		}
	}
	for _, v := range unknows {
		delete(*values, v)
	}

	if id == "" {
		for k := range defaults {
			if k == key {
				continue
			}
			if _, ok := (*values)[k]; !ok {
				(*values)[k] = ""
			}
		}
	}

	if valid {
		return key, nil
	}

	e := "field " + field + " have invalid type"
	return "", errors.New(e)
}

func ResponseToBytes(body interface{}, key string) []byte {
	response := make(map[string]interface{})
	responseRows := make(map[string]interface{})
	responseRows[key] = body
	response["response"] = responseRows
	json, _ := json.Marshal(response)
	return json
}

func ResponseError(error string) []byte {
	response := make(map[string]interface{})
	response["error"] = error
	json, _ := json.Marshal(response)
	return json
}
