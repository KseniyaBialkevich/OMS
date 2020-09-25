package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strconv"

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
	ID_order   int    `json:"id_order"`
	Status     string `json:"status"`
	Total_Cost int    `json:"total_cost"`
}

type OrdersToMenu struct {
	ID_order int `json:"id_order"`
	ID_menu  int `json:"id_menu"`
	Number   int `json:"number"`
}

type OrderResponse struct {
	Status string `json:"status"`
	Order  int    `json:"order"`
	Total  int    `json:"total"`
}

type StatusResponse struct {
	ID_order int    `json:"id_order"`
	Status   string `json:"status"`
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
		log.Println(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	tx, err := database.Begin()
	if err != nil {
		serverError(err, w)
		return
	}

	var id_order int

	err = tx.QueryRow("INSERT INTO orders (status) VALUES('recd') RETURNING id_order").Scan(&id_order)
	if err != nil {
		tx.Rollback()
		serverError(err, w)
		return
	}

	for _, order := range orderList {
		_, err = tx.Exec("INSERT INTO orders_to_menu (id_order, id_menu, number) VALUES ($1, $2, $3)", id_order, order.ID_menu, order.Number)
		if err != nil {
			tx.Rollback()
			serverError(err, w)
			return
		}
	}

	sum := tx.QueryRow("SELECT SUM(otm.number * m.price) FROM orders_to_menu AS otm JOIN menu AS m ON otm.id_menu = m.id_menu WHERE otm.id_order = $1", id_order)

	var total_cost int

	err = sum.Scan(&total_cost)
	if err != nil {
		tx.Rollback()
		serverError(err, w)
		return
	}

	_, err = tx.Exec("UPDATE orders SET total_cost = $1 WHERE id_order = $2", total_cost, id_order)
	if err != nil {
		tx.Rollback()
		serverError(err, w)
		return
	}

	err = tx.Commit()
	if err != nil {
		serverError(err, w)
		return
	} else {
		jsonResultEncode(id_order, total_cost, w) // err -?
	}
}

func OrderIsReady(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id_order"]
	id_order, _ := strconv.Atoi(id)

	var status string

	err := database.QueryRow("UPDATE orders SET status='ready' WHERE id_order=$1 RETURNING status", id_order).Scan(&status)
	if err != nil {
		serverError(err, w)
		return
	} else {
		jsonEncode(id_order, status, w) // err -?
	}
}

func OrderIsCompleted(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id_order"]
	id_order, _ := strconv.Atoi(id)

	var status string

	err := database.QueryRow("UPDATE orders SET status='completed' WHERE id_order=$1 RETURNING status", id_order).Scan(&status)
	if err != nil {
		serverError(err, w)
		return
	} else {
		jsonEncode(id_order, status, w) // err - ?
	}
}

func ListOrdersPendingProcessing(w http.ResponseWriter, r *http.Request) {
	rows, err := database.Query("SELECT id_order FROM orders WHERE status='recd'")
	if err != nil {
		serverError(err, w)
		return
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

	jsonListEncode(orders, w) // err -?
}

func ListOrdersPendingIssuance(w http.ResponseWriter, r *http.Request) {
	rows, err := database.Query("SELECT id_order FROM orders WHERE status='ready'")
	if err != nil {
		serverError(err, w)
		return
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

	jsonListEncode(orders, w) // err -?
}

func serverError(err error, w http.ResponseWriter) {
	log.Println(err)
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func jsonResultEncode(id_order int, total_cost int, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")

	response := OrderResponse{
		Status: "OK",
		Order:  id_order,
		Total:  total_cost,
	}

	json.NewEncoder(w).Encode(response)
}

func jsonEncode(id_order int, status string, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")

	response := StatusResponse{
		ID_order: id_order,
		Status:   status,
	}

	json.NewEncoder(w).Encode(response)
}

func jsonListEncode(orders []int, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orders)
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
