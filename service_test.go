package main

import (
	"encoding/json"
	"os"
	"testing"
)

// Проверка: валидный заказ проходит валидацию
func TestValidateOrder_Valid(t *testing.T) {
	data, err := os.ReadFile("model.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := validateOrder(data); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

// Проверка: пустой JSON не проходит
func TestValidateOrder_Invalid(t *testing.T) {
	bad := []byte(`{"foo":123}`)
	if err := validateOrder(bad); err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

// Проверка: восстановление кеша из БД
func TestLoadCacheFromDB(t *testing.T) {
	// создаём временную БД в памяти
	os.Setenv("DSN", "postgres://postgres:2212@localhost:5432/wb_demo?sslmode=disable")

	var err error
	db, err = initDB(os.Getenv("DSN"))
	if err != nil {
		t.Skip("PostgreSQL недоступен, тест пропущен")
	}
	defer db.Close()

	// вставляем тестовые данные
	raw := json.RawMessage(`{"order_uid":"test_order","delivery":{},"payment":{},"items":[{}]}`)
	if err := saveToDB("test_order", raw); err != nil {
		t.Fatal(err)
	}

	// очищаем кеш, потом восстанавливаем
	cache = make(map[string]json.RawMessage)
	if err := loadCacheFromDB(); err != nil {
		t.Fatal(err)
	}
	if _, ok := cache["test_order"]; !ok {
		t.Fatal("ожидалось, что заказ появится в кеше")
	}
}
