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
	ID    int    `json:"id_menu"`
	Name  string `json:"name"`
	Price int    `json:"price"`
}

type Order struct {
	ID        int    `json:"id_order"`
	Status    string `json:"status"`
	TotalCost int    `json:"total_cost"`
}

type OrderToMenu struct {
	IdOrder int `json:"id_order"`
	IdMenu  int `json:"id_menu"`
	Number  int `json:"number"`
}

var database *sql.DB

func serverError(w http.ResponseWriter, err error, statusCode int) {
	log.Println(err)
	http.Error(w, err.Error(), statusCode)
}

func jsonEncodeResponse(w http.ResponseWriter, obj interface{}) {
	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(obj)
	if err != nil {
		serverError(w, err, http.StatusInternalServerError)
		return
	}
}

func LookAtMenu(w http.ResponseWriter, r *http.Request) {
	rows, err := database.Query("SELECT * FROM menu")
	if err != nil {
		serverError(w, err, http.StatusInternalServerError)
		return
	}

	defer rows.Close()

	menuList := []Menu{}

	for rows.Next() {
		menuItem := Menu{}

		err := rows.Scan(&menuItem.ID, &menuItem.Name, &menuItem.Price)
		if err != nil {
			serverError(w, err, http.StatusInternalServerError)
			return
		}

		menuList = append(menuList, menuItem)
	}

	jsonEncodeResponse(w, menuList)
}

func CreateOrder(w http.ResponseWriter, r *http.Request) {
	type OrderData struct {
		ID     int `json:"id_menu"`
		Number int `json:"number"`
	}

	orderList := []OrderData{}

	err := json.NewDecoder(r.Body).Decode(&orderList)
	if err != nil {
		serverError(w, err, http.StatusBadRequest)
		return
	}

	tx, err := database.Begin()
	if err != nil {
		serverError(w, err, http.StatusInternalServerError)
		return
	}

	var idOrder int

	err = tx.QueryRow("INSERT INTO orders (status) VALUES('received') RETURNING id_order").Scan(&idOrder)
	if err != nil {
		tx.Rollback()
		serverError(w, err, http.StatusInternalServerError)
		return
	}

	for _, order := range orderList {
		_, err = tx.Exec("INSERT INTO orders_to_menu (id_order, id_menu, number) VALUES ($1, $2, $3)", idOrder, order.ID, order.Number)
		if err != nil {
			tx.Rollback()
			serverError(w, err, http.StatusInternalServerError)
			return
		}
	}

	row := tx.QueryRow(
		`SELECT SUM(otm.number * m.price) 
		FROM orders_to_menu AS otm JOIN menu AS m ON otm.id_menu = m.id_menu 
		WHERE otm.id_order = $1`,
		idOrder)

	var totalCost int

	err = row.Scan(&totalCost)
	if err != nil {
		tx.Rollback()
		serverError(w, err, http.StatusInternalServerError)
		return
	}

	_, err = tx.Exec("UPDATE orders SET total_cost = $1 WHERE id_order = $2", totalCost, idOrder)
	if err != nil {
		tx.Rollback()
		serverError(w, err, http.StatusInternalServerError)
		return
	}

	err = tx.Commit()
	if err != nil {
		serverError(w, err, http.StatusInternalServerError)
		return
	}

	row = database.QueryRow("SELECT * FROM orders WHERE id_order = $1", idOrder)

	order := Order{}

	err = row.Scan(&order.ID, &order.Status, &order.TotalCost)
	if err != nil {
		serverError(w, err, http.StatusInternalServerError)
		return
	}

	jsonEncodeResponse(w, order)
}

func ViewTheOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idOrder, ok := vars["id_order"]
	if !ok {
		err := fmt.Errorf("order id parameter is not found")
		serverError(w, err, http.StatusBadRequest)
		return
	}

	type OrderDetails struct {
		ID     int    `json:"id_menu"`
		Name   string `json:"name"`
		Number int    `json:"number"`
		Total  int    `json:"total"`
	}

	row := database.QueryRow("SELECT * FROM orders WHERE id_order = $1", idOrder)

	order := Order{}

	err := row.Scan(&order.ID, &order.Status, &order.TotalCost)
	if err != nil {
		serverError(w, err, http.StatusInternalServerError)
		return
	}

	rows, err := database.Query(
		`SELECT m.id_menu, m.name, otm.number, SUM(otm.number * m.price) AS total 
		FROM menu AS m JOIN orders_to_menu AS otm ON m.id_menu = otm.id_menu 
		WHERE otm.id_order = $1
		GROUP BY m.id_menu, m.name, otm.number`,
		idOrder)
	if err != nil {
		serverError(w, err, http.StatusInternalServerError)
		return
	}

	defer rows.Close()

	orderDetailList := []OrderDetails{}

	for rows.Next() {
		orderDetail := OrderDetails{}

		err := rows.Scan(&orderDetail.ID, &orderDetail.Name, &orderDetail.Number, &orderDetail.Total)
		if err != nil {
			serverError(w, err, http.StatusInternalServerError)
			return
		}

		orderDetailList = append(orderDetailList, orderDetail)
	}

	type OrderResult struct {
		Order           Order          `json:"order"`
		OrderDetailList []OrderDetails `json:"order_detail_list"`
	}

	result := OrderResult{Order: order, OrderDetailList: orderDetailList}

	jsonEncodeResponse(w, result)
}

func OrderIsReady(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idOrder, ok := vars["id_order"]
	if !ok {
		err := fmt.Errorf("order id parameter is not found")
		serverError(w, err, http.StatusBadRequest)
		return
	}

	_, err := database.Exec("UPDATE orders SET status = 'ready' WHERE id_order = $1", idOrder)
	if err != nil {
		serverError(w, err, http.StatusInternalServerError)
		return
	}

	row := database.QueryRow("SELECT * FROM orders WHERE id_order = $1", idOrder)

	order := Order{}

	err = row.Scan(&order.ID, &order.Status, &order.TotalCost)
	if err != nil {
		serverError(w, err, http.StatusInternalServerError)
		return
	}

	jsonEncodeResponse(w, order)
}

func OrderIsCompleted(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idOrder, ok := vars["id_order"]
	if !ok {
		err := fmt.Errorf("order id parameter is not found")
		serverError(w, err, http.StatusBadRequest)
		return
	}

	_, err := database.Exec("UPDATE orders SET status = 'completed' WHERE id_order = $1", idOrder)
	if err != nil {
		serverError(w, err, http.StatusInternalServerError)
		return
	}

	row := database.QueryRow("SELECT * FROM orders WHERE id_order = $1", idOrder)

	order := Order{}

	err = row.Scan(&order.ID, &order.Status, &order.TotalCost)
	if err != nil {
		serverError(w, err, http.StatusInternalServerError)
		return
	}

	jsonEncodeResponse(w, order)
}

func ListOrdersPendingProcessing(w http.ResponseWriter, r *http.Request) {
	rows, err := database.Query("SELECT * FROM orders WHERE status = 'received'")
	if err != nil {
		serverError(w, err, http.StatusInternalServerError)
		return
	}

	defer rows.Close()

	ordersList := []Order{}

	for rows.Next() {
		order := Order{}

		err := rows.Scan(&order.ID, &order.Status, &order.TotalCost)
		if err != nil {
			serverError(w, err, http.StatusInternalServerError)
			return
		}
		ordersList = append(ordersList, order)
	}

	jsonEncodeResponse(w, ordersList)
}

func ListOrdersPendingIssuance(w http.ResponseWriter, r *http.Request) {
	rows, err := database.Query("SELECT * FROM orders WHERE status = 'ready'")
	if err != nil {
		serverError(w, err, http.StatusInternalServerError)
		return
	}

	defer rows.Close()

	ordersList := []Order{}

	for rows.Next() {
		order := Order{}

		err := rows.Scan(&order.ID, &order.Status, &order.TotalCost)
		if err != nil {
			serverError(w, err, http.StatusInternalServerError)
			return
		}
		ordersList = append(ordersList, order)
	}

	jsonEncodeResponse(w, ordersList)
}

func main() {
	db, err := sql.Open("postgres", "user=kspsql password=pass1111 dbname=oms_db sslmode=disable")
	if err != nil {
		panic(err)
	}

	database = db

	defer db.Close()

	router := mux.NewRouter()
	router.HandleFunc("/menu", LookAtMenu)
	router.HandleFunc("/create_order", CreateOrder)
	router.HandleFunc("/order/{id_order:[0-9]+}", ViewTheOrder)
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
