package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
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
		fmt.Print("Choose: \n(0 / Default) Local PostgreSQL\n(1) Supabase PostgreSQL\n")

		var input int
		_, err = fmt.Scan(&input)
		if err != nil {
			fmt.Println("Defaulting to Local PostgreSQL")
			connectionType = 0
		} else {
			connectionType = input
		}
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
	fmt.Println("\nusers table OK")

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS pastes (
			id SERIAL PRIMARY KEY,
			userid INT NOT NULL,
			content TEXT NOT NULL
		)
	`)
	checkErr(err)
	fmt.Println("pastes table OK")
	fmt.Println("")

	interaction := -1

	for interaction != 0 && interaction != 1 {
		fmt.Print("Choose: \n(0 / Default) localhost:8080 endpoints\n(1) CLI interaction\n")

		var input int
		_, err = fmt.Scan(&input)
		if err != nil {
			fmt.Println("Defaulting to localhost:8080")
			interaction = 0
		} else {
			interaction = input
		}
	}

	if interaction == 0 {
		fmt.Println("\nAs a reminder, you've made a Postman collection to quickly send HTTP requests.")
		fmt.Println("")

		router := gin.Default()
		router.Use(dbMiddleware(db))

		// following u/mcvoid1's URL vs Query vs JSON Parameters advice
		// https://www.reddit.com/r/golang/comments/10huint/comment/j5b2tqv/?utm_source=share&utm_medium=web3x&utm_name=web3xcss&utm_term=1&utm_content=share_button
		router.GET("/selectUsers", funcNameMiddleware("selectUsers"), ginExecuteSQL)
		router.GET("/selectPastes", funcNameMiddleware("selectPastes"), ginExecuteSQL)
		router.GET("/readPaste/:pasteid", funcNameMiddleware("readPaste"), ginExecuteSQL)

		router.POST("/createUser", funcNameMiddleware("createUser"), ginExecuteSQL)
		router.POST("/createPaste", funcNameMiddleware("createPaste"), ginExecuteSQL)

		router.DELETE("/deletePaste/:pasteid", funcNameMiddleware("deletePaste"), ginExecuteSQL)
		router.PUT("/updatePaste/:pasteid", funcNameMiddleware("updatePaste"), ginExecuteSQL)

		router.Run("localhost:8080")
	} else {
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

}

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func dbMiddleware(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("db", db)
		c.Next()
	}
}

func funcNameMiddleware(funcName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("funcName", funcName)
		c.Next()
	}
}

func ginExecuteSQL(c *gin.Context) {
	db, exists := c.Get("db")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection not found"})
		return
	}

	dbConn, ok := db.(*sql.DB)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database connection not of type database"})
		return
	}

	funcName, exists := c.Get("funcName")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Function name not found"})
		return
	}

	var rows *sql.Rows
	var err error
	switch funcName {
	case "selectUsers":
		rows, err = selectUsers(dbConn)
	case "selectPastes":
		rows, err = selectPastes(dbConn)
	case "readPaste":
		pasteidStr := c.Param("pasteid")
		pasteid, err := strconv.Atoi(pasteidStr)
		checkErr(err)

		rows, err = readPaste(dbConn, pasteid)
	case "createUser":
		type RequestBody struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		}

		var requestBody RequestBody
		if err := c.BindJSON(&requestBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		_, err = createUser(dbConn, requestBody.Name, requestBody.Email)
	case "createPaste":
		type RequestBody struct {
			UserID  int    `json:"userid"`
			Content string `json:"content"`
		}

		var requestBody RequestBody
		if err := c.BindJSON(&requestBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		_, err = createPaste(dbConn, requestBody.UserID, requestBody.Content)
	case "deletePaste":
		pasteidStr := c.Param("pasteid")
		pasteid, err := strconv.Atoi(pasteidStr)
		checkErr(err)

		_, err = deletePaste(dbConn, pasteid)
	case "updatePaste":
		pasteidStr := c.Param("pasteid")
		pasteid, err := strconv.Atoi(pasteidStr)
		checkErr(err)

		type RequestBody struct {
			Content string `json:"content"`
		}

		var requestBody RequestBody

		if err := c.BindJSON(&requestBody); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
			return
		}

		_, err = updatePaste(dbConn, pasteid, requestBody.Content)
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Received a nonexistent funcName for executing SQL"})
		return
	}

	checkErr(err)

	if rows != nil {
		rowsJSON := rowsToJSON(rows)

		var result []map[string]interface{}
		err = json.Unmarshal(rowsJSON, &result)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse JSON: " + err.Error()})
			return
		}

		c.JSON(http.StatusOK, result)
	} else {
		c.JSON(http.StatusOK, "No rows to return.")
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

func readPaste(db *sql.DB, pasteid int) (*sql.Rows, error) {
	rows, err := db.Query(`
		SELECT * FROM pastes
		WHERE id=$1
	`, pasteid)

	return rows, err
}

func deletePaste(db *sql.DB, pasteid int) (*sql.Rows, error) {
	rows, err := db.Query(`
		DELETE FROM pastes
		WHERE id=$1
	`, pasteid)

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
