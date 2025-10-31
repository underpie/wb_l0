package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	_ "github.com/lib/pq"
	stan "github.com/nats-io/stan.go"
)

type Order struct {
	OrderUID string          `json:"order_uid"`
	Raw      json.RawMessage `json:"-"`
}

var (
	db       *sql.DB
	cache    = make(map[string]json.RawMessage)
	cacheMux sync.RWMutex
)

func initDB(dsn string) (*sql.DB, error) {
	d, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}

	if err := d.Ping(); err != nil {
		return nil, err
	}
	return d, nil
}

func loadCacheFromDB() error {
	rows, err := db.Query("SELECT order_uid, data FROM orders")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var raw []byte
		if err := rows.Scan(&id, &raw); err != nil {
			return err
		}
		cacheMux.Lock()
		cache[id] = json.RawMessage(raw)
		cacheMux.Unlock()
	}
	return rows.Err()
}

func saveToDB(id string, raw json.RawMessage) error {
	_, err := db.Exec(`
INSERT INTO orders(order_uid, data, created_at)
VALUES($1, $2, NOW())
ON CONFLICT (order_uid) DO UPDATE SET data = EXCLUDED.data, created_at = NOW()`,
		id, raw,
	)
	return err
}

func validateOrder(data []byte) error {
	var obj map[string]interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return fmt.Errorf("invalid JSON: %v", err)
	}

	// проверка обязательных полей
	if v, ok := obj["order_uid"].(string); !ok || v == "" {
		return errors.New("missing or invalid order_uid")
	}
	if _, ok := obj["delivery"].(map[string]interface{}); !ok {
		return errors.New("missing or invalid delivery field")
	}
	if _, ok := obj["payment"].(map[string]interface{}); !ok {
		return errors.New("missing or invalid payment field")
	}
	if items, ok := obj["items"].([]interface{}); !ok || len(items) == 0 {
		return errors.New("missing or empty items array")
	}

	return nil
}

func upsertCache(id string, raw json.RawMessage) {
	cacheMux.Lock()
	cache[id] = raw
	cacheMux.Unlock()
}

func getFromCache(id string) (json.RawMessage, bool) {
	cacheMux.RLock()
	defer cacheMux.RUnlock()
	v, ok := cache[id]
	return v, ok
}

func orderHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Path[len("/order/"):]
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	if data, ok := getFromCache(id); ok {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		return
	}

	var raw []byte
	err := db.QueryRow("SELECT data FROM orders WHERE order_uid = $1", id).Scan(&raw)
	if err == sql.ErrNoRows {
		http.Error(w, "not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(raw)
	upsertCache(id, json.RawMessage(raw))
}

func allOrdersHandler(w http.ResponseWriter, r *http.Request) {
	var all []map[string]interface{}
	for id, data := range cache {
		all = append(all, map[string]interface{}{
			"order_uid": id,
			"data":      data,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(all)
}

func subscribeNATS(clusterID, clientID, subject string) (stan.Conn, error) {
	sc, err := stan.Connect(clusterID, clientID, stan.NatsURL(stan.DefaultNatsURL))
	if err != nil {
		return nil, err
	}
	_, err = sc.Subscribe(subject, func(m *stan.Msg) {
		log.Printf("received message %d", m.Sequence)

		// проверка JSON структуры
		if err := validateOrder(m.Data); err != nil {
			log.Printf("invalid order skipped: %v", err)
			return
		}

		var tmp map[string]interface{}
		json.Unmarshal(m.Data, &tmp)
		id := tmp["order_uid"].(string)

		upsertCache(id, json.RawMessage(m.Data))
		if err := saveToDB(id, json.RawMessage(m.Data)); err != nil {
			log.Printf("save to db error: %v", err)
		} else {
			log.Printf("order %s saved successfully", id)
		}
	}, stan.DurableName("durable-orders"))
	if err != nil {
		sc.Close()
		return nil, err
	}
	return sc, nil
}

func ensureInitialData() {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM orders").Scan(&count)
	if err != nil {
		log.Printf("check db count error: %v", err)
		return
	}

	if count == 0 {
		log.Println("Database empty — inserting initial order from model.json...")
		data, err := os.ReadFile("model.json")
		if err != nil {
			log.Printf("cannot read model.json: %v", err)
			return
		}

		var tmp map[string]interface{}
		if err := json.Unmarshal(data, &tmp); err != nil {
			log.Printf("invalid JSON in model.json: %v", err)
			return
		}

		id, ok := tmp["order_uid"].(string)
		if !ok || id == "" {
			log.Println("no order_uid field in model.json")
			return
		}

		// сохранить в БД и в кэш
		if err := saveToDB(id, data); err != nil {
			log.Printf("insert initial order error: %v", err)
		} else {
			upsertCache(id, data)
			log.Printf("Initial order inserted successfully: %s", id)
		}
	} else {
		log.Printf("Database already contains %d orders", count)
	}
}

func main() {

	dsn := os.Getenv("DSN")
	if dsn == "" {
		dsn = "postgres://postgres:2212@localhost:5432/wb_demo?sslmode=disable"
	}
	clusterID := getenv("NATS_CLUSTER", "test-cluster")
	clientID := getenv("NATS_CLIENT", fmt.Sprintf("svc-%d", time.Now().Unix()))
	subject := getenv("NATS_SUBJECT", "orders")

	var err error
	db, err = initDB(dsn)
	if err != nil {
		log.Fatalf("db init: %v", err)
	}
	// кеш из дб
	if err := loadCacheFromDB(); err != nil {
		log.Printf("load cache error: %v", err)
	}
	ensureInitialData()

	sc, err := subscribeNATS(clusterID, clientID, subject)
	if err != nil {
		log.Printf("nats subscribe error: %v (service will still run)", err)
	} else {
		defer sc.Close()
	}

	http.HandleFunc("/order/", orderHandler)
	srv := &http.Server{Addr: ":8080"}
	http.Handle("/", http.FileServer(http.Dir("web")))
	http.HandleFunc("/order/all", allOrdersHandler)

	go func() {
		log.Printf("http server listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	log.Println("shutting down")
	srv.Close()
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
