package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"

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
		requestError(err, w)
	}

	tx, err := database.Begin()
	if err != nil {
		serverError(err, w)
	}

	var id_order int

	err = tx.QueryRow("INSERT INTO orders (status) VALUES('recd') RETURNING id_order").Scan(&id_order)
	if err != nil {
		tx.Rollback()
		serverError(err, w)
	}

	for _, order := range orderList {
		_, err = tx.Exec("INSERT INTO orders_to_menu (id_order, id_menu, number) VALUES ($1, $2, $3)", id_order, order.ID_menu, order.Number)
		if err != nil {
			tx.Rollback()
			serverError(err, w)
		}
	}

	err = tx.Commit()
	if err != nil {
		serverError(err, w)
	}

	id := strconv.Itoa(id_order)

	w.Write([]byte(id + " - OK"))
}

func OrderIsReady(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id_order := vars["id_order"]

	var status string

	err := database.QueryRow("UPDATE orders SET status='ready' WHERE id_order=$1 RETURNING status", id_order).Scan(&status)
	if err != nil {
		serverError(err, w)
	}

	w.Write([]byte("order: " + id_order + "\nstatus: " + status))
}

func OrderIsCompleted(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id_order := vars["id_order"]

	var status string

	err := database.QueryRow("UPDATE orders SET status='completed' WHERE id_order=$1 RETURNING status", id_order).Scan(&status)
	if err != nil {
		serverError(err, w)
	}

	w.Write([]byte("order: " + id_order + "\nstatus: " + status))
}

func ListOrdersPendingProcessing(w http.ResponseWriter, r *http.Request) {
	rows, err := database.Query("SELECT id_order FROM orders WHERE status='recd'")
	if err != nil {
		serverError(err, w)
	}

	defer rows.Close()

	orders := (make([]int, 0))

	for rows.Next() {
		var order int

		err := rows.Scan(&order)
		if err != nil {
			log.Println(err)
			continue
		}
		orders = append(orders, order)
	}

	ordersText := (make([]string, 0))

	for idx, _ := range orders {
		id := orders[idx]
		idTxt := strconv.Itoa(id)
		ordersText = append(ordersText, idTxt)
	}

	result := strings.Join(ordersText, "\n")

	w.Write([]byte("the orders are received:\n" + result))
}

func ListOrdersPendingIssuance(w http.ResponseWriter, r *http.Request) {
	rows, err := database.Query("SELECT id_order FROM orders WHERE status='ready'")
	if err != nil {
		serverError(err, w)
	}

	defer rows.Close()

	orders := (make([]int, 0))

	for rows.Next() {
		var order int

		err := rows.Scan(&order)
		if err != nil {
			log.Println(err)
			continue
		}
		orders = append(orders, order)
	}

	ordersText := (make([]string, 0))

	for idx, _ := range orders {
		id := orders[idx]
		idTxt := strconv.Itoa(id)
		ordersText = append(ordersText, idTxt)
	}

	result := strings.Join(ordersText, "\n")

	w.Write([]byte("the orders are redy:\n" + result))
}

func requestError(err error, w http.ResponseWriter) {
	log.Println(err)
	http.Error(w, err.Error(), http.StatusBadRequest)
	return
}

func serverError(err error, w http.ResponseWriter) {
	log.Println(err)
	http.Error(w, err.Error(), http.StatusInternalServerError)
	return
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
	router.HandleFunc("/ready/{id_order:[0-9]+}", OrderIsReady)
	router.HandleFunc("/completed/{id_order:[0-9]+}", OrderIsCompleted)
	router.HandleFunc("/processing", ListOrdersPendingProcessing)
	router.HandleFunc("/issuance", ListOrdersPendingIssuance)

	http.Handle("/", router)

	fmt.Println("Server is listening...")

	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Println("HTTP Server Error - ", err)
	}
}
