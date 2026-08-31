package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"oms/internal/cache"
	"oms/internal/models"
	"oms/internal/repository"
)

type OrderService struct {
	productRepo repository.ProductRepository
	orderRepo   repository.OrderRepository
	redisClient *cache.RedisClient
}

func NewOrderService(
	productRepo repository.ProductRepository,
	orderRepo repository.OrderRepository,
	redisClient *cache.RedisClient,
) *OrderService {
	return &OrderService{
		productRepo: productRepo,
		orderRepo:   orderRepo,
		redisClient: redisClient,
	}
}

func (s *OrderService) ListProducts(ctx context.Context) ([]models.Product, error) {
	if s.redisClient != nil {
		cached, err := s.redisClient.GetCachedProducts(ctx)
		if err == nil && len(cached) > 0 {
			log.Println("Returning products from Redis cache")
			return cached, nil
		}
	}

	products, err := s.productRepo.GetAll(ctx)
	if err != nil {
		return nil, err
	}

	if s.redisClient != nil && len(products) > 0 {
		_ = s.redisClient.SetCachedProducts(ctx, products, 10*time.Minute)
	}

	return products, nil
}

func (s *OrderService) CreateProduct(ctx context.Context, req models.CreateProductRequest) (*models.Product, error) {
	if req.Name == "" || req.Price <= 0 || req.Stock < 0 {
		return nil, errors.New("invalid product data: name, positive price, and non-negative stock are required")
	}

	p := &models.Product{
		Name:        req.Name,
		Description: req.Description,
		Price:       req.Price,
		Stock:       req.Stock,
	}

	if err := s.productRepo.Create(ctx, p); err != nil {
		return nil, err
	}

	if s.redisClient != nil {
		_ = s.redisClient.InvalidateProductCache(ctx)
	}

	return p, nil
}

func (s *OrderService) CreateOrder(ctx context.Context, req models.CreateOrderRequest) (*models.Order, error) {
	if req.CustomerName == "" || req.CustomerEmail == "" {
		return nil, errors.New("customer name and email are required")
	}
	if len(req.Items) == 0 {
		return nil, errors.New("order must contain at least one item")
	}

	// 1. Redis lock if available
	var acquiredLocks []int
	if s.redisClient != nil {
		for _, item := range req.Items {
			locked, err := s.redisClient.AcquireLock(ctx, item.ProductID, 5*time.Second)
			if err != nil || !locked {
				for _, lockedID := range acquiredLocks {
					_ = s.redisClient.ReleaseLock(ctx, lockedID)
				}
				return nil, fmt.Errorf("system busy processing orders for product ID %d, please try again", item.ProductID)
			}
			acquiredLocks = append(acquiredLocks, item.ProductID)
		}
		defer func() {
			for _, lockedID := range acquiredLocks {
				_ = s.redisClient.ReleaseLock(ctx, lockedID)
			}
		}()
	}

	// 2. Start Transaction (if supported by storage)
	tx, err := s.orderRepo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	if tx != nil {
		defer tx.Rollback()
	}

	var totalAmount float64
	var orderItems []models.OrderItem

	// 3. Stock validation & deduction
	for _, itemReq := range req.Items {
		if itemReq.Quantity <= 0 {
			return nil, fmt.Errorf("quantity for product ID %d must be greater than 0", itemReq.ProductID)
		}

		p, err := s.productRepo.GetByID(ctx, itemReq.ProductID)
		if err != nil {
			return nil, fmt.Errorf("error fetching product %d: %w", itemReq.ProductID, err)
		}
		if p == nil {
			return nil, fmt.Errorf("product ID %d not found", itemReq.ProductID)
		}
		if p.Stock < itemReq.Quantity {
			return nil, fmt.Errorf("insufficient stock for '%s' (available: %d, requested: %d)", p.Name, p.Stock, itemReq.Quantity)
		}

		if err := s.productRepo.DeductStock(ctx, tx, itemReq.ProductID, itemReq.Quantity); err != nil {
			return nil, fmt.Errorf("failed to deduct stock for product %d: %w", itemReq.ProductID, err)
		}

		subtotal := p.Price * float64(itemReq.Quantity)
		totalAmount += subtotal

		orderItems = append(orderItems, models.OrderItem{
			ProductID:   itemReq.ProductID,
			ProductName: p.Name,
			Quantity:    itemReq.Quantity,
			UnitPrice:   p.Price,
			Subtotal:    subtotal,
		})
	}

	// 4. Save Order
	order := &models.Order{
		CustomerName:  req.CustomerName,
		CustomerEmail: req.CustomerEmail,
		TotalAmount:   totalAmount,
		Status:        models.StatusPending,
	}

	if err := s.orderRepo.Create(ctx, tx, order); err != nil {
		return nil, fmt.Errorf("failed to create order: %w", err)
	}

	// 5. Save Order Items
	for i := range orderItems {
		orderItems[i].OrderID = order.ID
		if err := s.orderRepo.CreateItem(ctx, tx, &orderItems[i]); err != nil {
			return nil, fmt.Errorf("failed to create order item: %w", err)
		}
	}

	// 6. Commit Transaction if DB tx present
	if tx != nil {
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("failed to commit order transaction: %w", err)
		}
	}

	order.Items = orderItems

	if s.redisClient != nil {
		_ = s.redisClient.InvalidateProductCache(ctx)
	}

	return order, nil
}

func (s *OrderService) ListOrders(ctx context.Context) ([]models.Order, error) {
	return s.orderRepo.GetAll(ctx)
}

func (s *OrderService) GetOrder(ctx context.Context, id int) (*models.Order, error) {
	return s.orderRepo.GetByID(ctx, id)
}

func (s *OrderService) UpdateOrderStatus(ctx context.Context, id int, newStatus models.OrderStatus) (*models.Order, error) {
	if !newStatus.IsValid() {
		return nil, fmt.Errorf("invalid order status: %s", newStatus)
	}

	currentOrder, err := s.orderRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if currentOrder == nil {
		return nil, fmt.Errorf("order ID %d not found", id)
	}

	if currentOrder.Status == newStatus {
		return currentOrder, nil
	}

	if currentOrder.Status == models.StatusCompleted || currentOrder.Status == models.StatusCancelled {
		return nil, fmt.Errorf("cannot change status of an order that is already %s", currentOrder.Status)
	}

	var tx *sql.Tx
	if newStatus == models.StatusCancelled {
		tx, err = s.orderRepo.BeginTx(ctx)
		if err != nil {
			return nil, err
		}
		if tx != nil {
			defer tx.Rollback()
		}

		items, err := s.orderRepo.GetOrderItems(ctx, id)
		if err != nil {
			return nil, err
		}

		for _, item := range items {
			if err := s.productRepo.RestoreStock(ctx, tx, item.ProductID, item.Quantity); err != nil {
				return nil, fmt.Errorf("failed to restore stock for product %d: %w", item.ProductID, err)
			}
		}

		if err := s.orderRepo.UpdateStatus(ctx, id, newStatus); err != nil {
			return nil, err
		}

		if tx != nil {
			if err := tx.Commit(); err != nil {
				return nil, err
			}
		}

		if s.redisClient != nil {
			_ = s.redisClient.InvalidateProductCache(ctx)
		}
	} else {
		if err := s.orderRepo.UpdateStatus(ctx, id, newStatus); err != nil {
			return nil, err
		}
	}

	return s.orderRepo.GetByID(ctx, id)
}
