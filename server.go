package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

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
	stackTrace := string(debug.Stack())
	msg := fmt.Sprintf("Error: %s\n%s", err, stackTrace)

	log.Println(msg)
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

func changeStatus(w http.ResponseWriter, r *http.Request, status string) {
	vars := mux.Vars(r)
	idOrder, ok := vars["id_order"]
	if !ok {
		err := fmt.Errorf("order id parameter is not found")
		serverError(w, err, http.StatusBadRequest)
		return
	}

	_, err := database.Exec("UPDATE orders SET status = $1 WHERE id_order = $2", status, idOrder)
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

func returnOrderList(w http.ResponseWriter, status string) {
	rows, err := database.Query("SELECT * FROM orders WHERE status = $1", status)
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

	orderDetailsList := []OrderDetails{}

	for rows.Next() {
		orderDetail := OrderDetails{}

		err := rows.Scan(&orderDetail.ID, &orderDetail.Name, &orderDetail.Number, &orderDetail.Total)
		if err != nil {
			serverError(w, err, http.StatusInternalServerError)
			return
		}

		orderDetailsList = append(orderDetailsList, orderDetail)
	}

	type OrderResult struct {
		Order            Order          `json:"order"`
		OrderDetailsList []OrderDetails `json:"order_details_list"`
	}

	result := OrderResult{Order: order, OrderDetailsList: orderDetailsList}

	jsonEncodeResponse(w, result)
}

func OrderIsReady(w http.ResponseWriter, r *http.Request) {
	changeStatus(w, r, "ready")
}

func OrderIsCompleted(w http.ResponseWriter, r *http.Request) {
	changeStatus(w, r, "completed")
}

func ListOfReceivedOrders(w http.ResponseWriter, r *http.Request) {
	returnOrderList(w, "received")
}

func ListOfReadyOrders(w http.ResponseWriter, r *http.Request) {
	returnOrderList(w, "ready")
}

func main() {
	host := os.Getenv("host")
	port := os.Getenv("port")
	user := os.Getenv("user")
	password := os.Getenv("password")
	dbname := os.Getenv("dbname")

	dataSourceName := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s", host, port, user, password, dbname)

	db, err := sql.Open("postgres", dataSourceName)
	if err != nil {
		panic(err)
	}

	database = db

	defer db.Close()

	router := mux.NewRouter()
	router.HandleFunc("/menu", LookAtMenu).Methods("GET")
	router.HandleFunc("/create_order", CreateOrder).Methods("POST")
	router.HandleFunc("/order/{id_order:[0-9]+}", ViewTheOrder).Methods("GET")
	router.HandleFunc("/ready/{id_order:[0-9]+}", OrderIsReady).Methods("PUT")
	router.HandleFunc("/completed/{id_order:[0-9]+}", OrderIsCompleted).Methods("PUT")
	router.HandleFunc("/received_orders", ListOfReceivedOrders).Methods("GET")
	router.HandleFunc("/ready_orders", ListOfReadyOrders).Methods("GET")

	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	startServerFunc := func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}
	go startServerFunc()

	log.Print("Server Started")
	log.Printf("pid: %d\n", os.Getpid())

	msg := <-done
	log.Printf("Stop server command: '%s'", msg)
	log.Print("Server Stopped")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	closeFunc := func() {
		cancel()
	}
	defer closeFunc()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server Shutdown Failed:%+v", err)
	}
	log.Print("Server Exited Properly")
}
