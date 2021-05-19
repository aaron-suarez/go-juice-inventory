package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

var db *sql.DB
var once sync.Once

type juice struct {
	Id         int64     `json:"id"`
	Name       string    `json:"name"`
	Expiration time.Time `json:"expiration"`
}

func main() {
	fmt.Println("Starting up!")
	db := getDbInstance()

	defer db.Close()

	err := db.Ping()

	// TODO: remove when the db doesn't need to be initialized from scratch by setUpDb()
	if err != nil {
		fmt.Println("Waiting for DB to come online...")
		time.Sleep(2 * time.Second)
		err = db.Ping()
	}
	CheckError(err)

	// TODO: remove this too once there is a stable production database
	setUpDb(db)

	startServer()
}

/*
 * GET /
 */
func HomeHandler(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintf(w, "Welcome to the Juice Inventory\n")
}

/*
 * DELETE /products/{id}
 */
func DeleteHandler(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	fmt.Println("Successfully deleted", vars["id"])
	fmt.Fprintf(w, "Successfully deleted %s\n", vars["id"])
}

/*
 * GET /products
 */
func StockDisplayHandler(w http.ResponseWriter, req *http.Request) {
	rows, err := db.Query("SELECT * FROM juice LIMIT 200;")
	CheckError(err)

	defer rows.Close()

	var juiceSlice []juice

	for rows.Next() {
		var (
			id         int64
			name       string
			expiration time.Time
		)
		err := rows.Scan(&id, &name, &expiration)
		CheckError(err)
		juiceSlice = append(juiceSlice, juice{Id: id, Name: name, Expiration: expiration})
	}
	prettyJSON, err := json.MarshalIndent(juiceSlice, "", "    ")
	CheckError(err)

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, "%s\n", prettyJSON)
}

/*
 * Utility Functions
 */

func getDbInstance() *sql.DB {
	if db == nil {
		once.Do(
			func() {
				fmt.Println("Creating a single instance of database")
				host := os.Getenv("POSTGRES_HOST")
				port, e := strconv.Atoi(os.Getenv("POSTGRES_PORT"))
				user := os.Getenv("POSTGRES_USER")
				password := os.Getenv("POSTGRES_PASSWORD")
				dbname := os.Getenv("POSTGRES_DB")

				if e != nil {
					fmt.Println("Invalid port, using 5432 instead")
					port = 5432
				}
				psqlconn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", host, port, user, password, dbname)
				fmt.Println(psqlconn)

				temp, err := sql.Open("postgres", psqlconn)
				db = temp
				CheckError(err)
			})
	}
	return db
}

func setUpDb(db *sql.DB) {
	createSchema := `CREATE SCHEMA IF NOT EXISTS inventory;`
	createTable := `CREATE TABLE IF NOT EXISTS juice (
		id          int GENERATED BY DEFAULT AS IDENTITY PRIMARY KEY,
		name        varchar NOT NULL,
		expiration  date
	);`

	_, err := db.Exec(createSchema)
	CheckError(err)
	_, err = db.Exec(createTable)
	CheckError(err)

	populateData(db)

	fmt.Println("Sucessfully set up database")
}

func populateData(db *sql.DB) {
	var count int

	err := db.QueryRow("SELECT COUNT(*) FROM juice").Scan(&count)
	CheckError(err)

	// Only need to populate if it's not already populated
	if count > 0 {
		fmt.Println("Sucessfully inserted rows into the juice table")
		return
	}

	insertJuices := `INSERT INTO juice (name, expiration) VALUES `
	juiceNames, err := os.Open("src/juices.txt")
	CheckError(err)
	defer juiceNames.Close()

	scnr := bufio.NewScanner(juiceNames)
	scnr.Split(bufio.ScanLines)

	for scnr.Scan() {
		insertJuices = insertJuices + "('" + scnr.Text() + "', '" + randate().Format("Jan-02-06") + "'),\n"
	}
	insertJuices = insertJuices[:len(insertJuices)-2]
	insertJuices = insertJuices + ";"

	_, err = db.Exec(insertJuices)
	CheckError(err)
	fmt.Println("Sucessfully inserted rows into the juice table")
}

// Generates a random date somewhere between now and 1 year from now
func randate() time.Time {
	var delta int64 = 31557600

	sec := rand.Int63n(delta) + time.Now().Unix()
	return time.Unix(sec, 0)
}

func startServer() {
	fmt.Println("Starting server...")
	r := mux.NewRouter()
	r.HandleFunc("/", HomeHandler)
	r.HandleFunc("/products/{id}", DeleteHandler).Methods("DELETE")
	r.HandleFunc("/products", StockDisplayHandler).Methods("GET")

	http.Handle("/", r)
	http.ListenAndServe(":8090", r)
}

func CheckError(err error) {
	if err != nil {
		panic(err)
	}
}
