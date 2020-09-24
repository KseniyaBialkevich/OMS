package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"net/http"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

type Menu struct {
	ID_menu int    `json:"id_menu"`
	Name    string `json:"name"`
	Price   int    `json:"price"`
}

type Orders struct {
	ID_order int    `json:"id_order"`
	Status   string `json:"status"`
}

type OrdersToMenu struct {
	ID_order int `json:"id_order"`
	ID_menu  int `json:"id_menu"`
	Number   int `json:"number"`
}

var database *sql.DB

func CreateOrder(w http.ResponseWriter, r *http.Request) {
	type OrderData struct {
		ID_menu int `json:"id_menu"`
		Number  int `json:"number"`
	}

	orderList := make([]OrderData, 0)

	err := json.NewDecoder(r.Body).Decode(&orderList)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tx, err := database.Begin()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	var id_order int

	err = tx.QueryRow("INSERT INTO orders (status) VALUES('recd') RETURNING id_order").Scan(&id_order)
	if err != nil {
		tx.Rollback()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	for _, order := range orderList {
		_, err = database.Exec("INSERT INTO orders_to_menu (id_order, id_menu, number) VALUES ($1, $2, $3)", id_order, order.ID_menu, order.Number)
		if err != nil {
			tx.Rollback()
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	w.Write([]byte("OK"))
}

func main() {

	db, err := sql.Open("postgres", "user=kspsql password=pass1111 dbname=oms_db sslmode=disable")
	if err != nil {
		panic(err)
	}

	database = db

	defer db.Close()

	router := mux.NewRouter()
	router.HandleFunc("/create_order", CreateOrder)

	http.Handle("/", router)

	fmt.Println("Server is listening...")

	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Println("HTTP Server Error - ", err)
	}
}
