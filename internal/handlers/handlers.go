package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"oms/internal/models"
	"oms/internal/service"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	orderService *service.OrderService
}

func NewHandler(orderService *service.OrderService) *Handler {
	return &Handler{orderService: orderService}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.RootHandler)
	r.Get("/health", h.HealthCheck)
	r.Get("/health", h.HealthCheck)
	r.Get("/app", h.ServeFrontend)

	r.Route("/api", func(r chi.Router) {
		r.Get("/products", h.ListProducts)
		r.Post("/products", h.CreateProduct)

		r.Get("/orders", h.ListOrders)
		r.Post("/orders", h.CreateOrder)
		r.Get("/orders/{id}", h.GetOrder)
		r.Patch("/orders/{id}/status", h.UpdateOrderStatus)
	})
}

func (h *Handler) RootHandler(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, http.StatusOK, map[string]interface{}{
		"service": "Order Management System API",
		"status":  "running",
		"documentation": map[string]string{
			"health_check":   "GET /health",
			"list_products":  "GET /api/products",
			"create_product": "POST /api/products",
			"list_orders":    "GET /api/orders",
			"get_order":      "GET /api/orders/{id}",
			"create_order":   "POST /api/orders",
			"update_status":  "PATCH /api/orders/{id}/status",
		},
		"message": "Welcome to the Order Management System API! Visit /api/products or /health to get started.",
	})
}

func (h *Handler) ServeFrontend(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "web/index.html")
}

func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	sendJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Message: "Order Management System API is healthy",
	})
}

func (h *Handler) ListProducts(w http.ResponseWriter, r *http.Request) {
	products, err := h.orderService.ListProducts(r.Context())
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Data:    products,
	})
}

func (h *Handler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var req models.CreateProductRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	product, err := h.orderService.CreateProduct(r.Context(), req)
	if err != nil {
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	sendJSON(w, http.StatusCreated, models.APIResponse{
		Success: true,
		Message: "Product created successfully",
		Data:    product,
	})
}

func (h *Handler) ListOrders(w http.ResponseWriter, r *http.Request) {
	orders, err := h.orderService.ListOrders(r.Context())
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	sendJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Data:    orders,
	})
}

func (h *Handler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	var req models.CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	order, err := h.orderService.CreateOrder(r.Context(), req)
	if err != nil {
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	sendJSON(w, http.StatusCreated, models.APIResponse{
		Success: true,
		Message: "Order created successfully",
		Data:    order,
	})
}

func (h *Handler) GetOrder(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		sendError(w, http.StatusBadRequest, "Invalid order ID")
		return
	}

	order, err := h.orderService.GetOrder(r.Context(), id)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if order == nil {
		sendError(w, http.StatusNotFound, "Order not found")
		return
	}

	sendJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Data:    order,
	})
}

func (h *Handler) UpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		sendError(w, http.StatusBadRequest, "Invalid order ID")
		return
	}

	var req models.UpdateOrderStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendError(w, http.StatusBadRequest, "Invalid JSON payload")
		return
	}

	order, err := h.orderService.UpdateOrderStatus(r.Context(), id, req.Status)
	if err != nil {
		sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	sendJSON(w, http.StatusOK, models.APIResponse{
		Success: true,
		Message: "Order status updated successfully",
		Data:    order,
	})
}

func sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func sendError(w http.ResponseWriter, status int, message string) {
	sendJSON(w, status, models.APIResponse{
		Success: false,
		Error:   message,
	})
}
