package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"oms/internal/cache"
	"oms/internal/config"
	"oms/internal/database"
	"oms/internal/handlers"
	"oms/internal/repository"
	"oms/internal/service"
)

func main() {
	cfg := config.Load()
	log.Printf("Starting Order Management System on port %s...", cfg.Port)

	var productRepo repository.ProductRepository
	var orderRepo repository.OrderRepository

	// Connect PostgreSQL (if available)
	db, err := database.Connect(cfg)
	if err == nil && db != nil {
		log.Println("[Storage] Using PostgreSQL Database Engine")
		if err := database.InitSchema(db, "migrations/init.sql"); err != nil {
			log.Printf("Warning: Schema migration notice: %v", err)
		}
		productRepo = repository.NewPostgresProductRepository(db)
		orderRepo = repository.NewPostgresOrderRepository(db)
		defer db.Close()
	} else {
		log.Println("[Storage] PostgreSQL unavailable. Using In-Memory Storage Engine with default seed products.")
		productRepo = repository.NewMemoryProductRepository()
		orderRepo = repository.NewMemoryOrderRepository()
	}

	// Connect Redis (if available)
	redisClient, err := cache.NewRedisClient(cfg)
	if err != nil {
		log.Println("[Cache] Redis unavailable. Operating with standard memory locks.")
	} else {
		log.Println("[Cache] Using Redis Cache & Lock Engine")
	}

	// Initialize Service
	orderService := service.NewOrderService(productRepo, orderRepo, redisClient)

	// Initialize HTTP Router & Middleware
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	handler := handlers.NewHandler(orderService)
	handler.RegisterRoutes(r)

	log.Printf("Order Management System API listening on http://localhost:%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatalf("Server shutdown with error: %v", err)
	}
}
