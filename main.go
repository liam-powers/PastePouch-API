package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
	// for registering postgres as a driver for the sql connection
	_ "github.com/lib/pq"
)

// TABLES:
// users
// userid, name, email
//
// pastes
// id, userid, content

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Error loading .env")
	}

	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASS")

	connectionType := -1
	for connectionType != 0 && connectionType != 1 {
		fmt.Print("Choose: \n(0) Local PostgreSQL\n(1) Supabase PostgreSQL\n")
		fmt.Scan(&connectionType)
		connectionType = int(connectionType)
	}

	var connectionString string
	if connectionType == 0 {
		connectionString = "user=liam dbname=pastepouch sslmode=disable"
	} else {
		connectionString = fmt.Sprintf("user=%s password=%s host=aws-0-us-east-1.pooler.supabase.com port=6543 dbname=postgres", dbUser, dbPass)
	}

	db, err := sql.Open("postgres", connectionString)
	checkErr(err)
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			name TEXT,
			email TEXT UNIQUE NOT NULL
		)
	`)
	checkErr(err)

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS pastes (
			id SERIAL PRIMARY KEY,
			userid INT NOT NULL,
			content TEXT NOT NULL
		)
	`)
	checkErr(err)

	for true {
		var val int
		fmt.Print(`
(1) Display all users entries
(2) Display all pastes entries
(3) Create a new user
(4) Create a new paste
(5) Read a paste
(6) Delete a paste
(7) Update a paste
`)
		fmt.Scan(&val)

		switch val {
		case 1:
			rows, err := selectUsers(db)
			checkErr(err)
			json := rowsToJSON(rows)
			fmt.Println(string(json))
		case 2:
			rows, err := selectPastes(db)
			checkErr(err)
			json := rowsToJSON(rows)
			fmt.Println(string(json))
		case 3:
			var name string
			var email string
			fmt.Print("createUser> Enter name: ")
			fmt.Scan(&name)

			fmt.Print("createUser> Enter email: ")
			fmt.Scan(&email)

			_, err := createUser(db, name, email)
			checkErr(err)
		case 4:
			var userid int
			fmt.Print("createPaste> Enter userid: ")
			fmt.Scan(&userid)

			fmt.Print("createPaste> Enter content: ")
			in := bufio.NewReader(os.Stdin)
			content, err := in.ReadString('\n')
			checkErr(err)
			// removing newline character from end (shows up as a + in pSQL)
			content = strings.TrimSpace(content)

			fmt.Println("createPaste> Your content was: ", content)

			_, err = createPaste(db, userid, content)
			checkErr(err)
		case 5:
			var pasteid int
			fmt.Print("readPaste> Enter pasteid: ")
			fmt.Scan(&pasteid)

			rows, err := readPaste(db, pasteid)
			checkErr(err)
			json := rowsToJSON(rows)
			fmt.Println(string(json))
		case 6:
			var pasteid int
			fmt.Print("deletePaste> Enter pasteid: ")
			fmt.Scan(&pasteid)

			rows, err := deletePaste(db, pasteid)
			checkErr(err)
			json := rowsToJSON(rows)
			fmt.Println(string(json))
		case 7:
			var pasteid int
			fmt.Print("updatePaste> Enter pasteid: ")
			fmt.Scan(&pasteid)

			fmt.Print("updatePaste> Enter content: ")
			in := bufio.NewReader(os.Stdin)
			content, err := in.ReadString('\n')
			checkErr(err)
			// removing newline character from end (shows up as a + in pSQL)
			content = strings.TrimSpace(content)

			fmt.Println("updatePaste> Your content was: ", content)

			rows, err := updatePaste(db, pasteid, content)
			checkErr(err)
			json := rowsToJSON(rows)
			fmt.Println(string(json))
		}
	}
}

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func createUser(db *sql.DB, name string, email string) (sql.Result, error) {
	res, err := db.Exec(
		`INSERT INTO users (name, email)
		VALUES ($1, $2)`, name, email)

	return res, err
}

func createPaste(db *sql.DB, userid int, content string) (sql.Result, error) {
	res, err := db.Exec(
		`INSERT INTO pastes (userid, content)
		VALUES ($1, $2)`, userid, content)

	return res, err
}

func selectUsers(db *sql.DB) (*sql.Rows, error) {
	rows, err := db.Query(`
		SELECT * FROM users
	`)

	return rows, err
}

func selectPastes(db *sql.DB) (*sql.Rows, error) {
	rows, err := db.Query(`
		SELECT * FROM pastes
	`)

	return rows, err
}

func readPaste(db *sql.DB, pasteId int) (*sql.Rows, error) {
	rows, err := db.Query(`
		SELECT * FROM pastes
		WHERE id=$1
	`, pasteId)

	return rows, err
}

func deletePaste(db *sql.DB, pasteId int) (*sql.Rows, error) {
	rows, err := db.Query(`
		DELETE FROM pastes
		WHERE id=$1
	`, pasteId)

	return rows, err
}

func updatePaste(db *sql.DB, pasteid int, newContent string) (*sql.Rows, error) {
	rows, err := db.Query(`
		UPDATE pastes
		SET content=$1
		WHERE id=$2
	`, newContent, pasteid)

	return rows, err
}

func rowsToJSON(rows *sql.Rows) []byte {
	columnTypes, err := rows.ColumnTypes()
	checkErr(err)

	count := len(columnTypes)
	finalRows := []interface{}{}

	for rows.Next() {
		scanArgs := make([]interface{}, count)

		for i, v := range columnTypes {
			switch v.DatabaseTypeName() {
			case "VARCHAR", "TEXT", "UUID", "TIMESTAMP":
				scanArgs[i] = new(sql.NullString)
				break
			case "BOOL":
				scanArgs[i] = new(sql.NullBool)
				break
			case "INT4":
				scanArgs[i] = new(sql.NullInt64)
				break
			default:
				scanArgs[i] = new(sql.NullString)
			}
		}

		err := rows.Scan(scanArgs...)
		checkErr(err)

		masterData := map[string]interface{}{}

		for i, v := range columnTypes {

			if z, ok := (scanArgs[i]).(*sql.NullBool); ok {
				masterData[v.Name()] = z.Bool
				continue
			}

			if z, ok := (scanArgs[i]).(*sql.NullString); ok {
				masterData[v.Name()] = z.String
				continue
			}

			if z, ok := (scanArgs[i]).(*sql.NullInt64); ok {
				masterData[v.Name()] = z.Int64
				continue
			}

			if z, ok := (scanArgs[i]).(*sql.NullFloat64); ok {
				masterData[v.Name()] = z.Float64
				continue
			}

			if z, ok := (scanArgs[i]).(*sql.NullInt32); ok {
				masterData[v.Name()] = z.Int32
				continue
			}

			masterData[v.Name()] = scanArgs[i]
		}

		finalRows = append(finalRows, masterData)
	}

	z, err := json.Marshal(finalRows)
	return z
}
