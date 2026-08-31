package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"oms/internal/models"
)

type OrderRepository interface {
	BeginTx(ctx context.Context) (*sql.Tx, error)
	Create(ctx context.Context, tx *sql.Tx, order *models.Order) error
	CreateItem(ctx context.Context, tx *sql.Tx, item *models.OrderItem) error
	GetAll(ctx context.Context) ([]models.Order, error)
	GetByID(ctx context.Context, id int) (*models.Order, error)
	UpdateStatus(ctx context.Context, id int, status models.OrderStatus) error
	GetOrderItems(ctx context.Context, orderID int) ([]models.OrderItem, error)
}

// PostgresOrderRepository handles PostgreSQL storage
type PostgresOrderRepository struct {
	db *sql.DB
}

func NewPostgresOrderRepository(db *sql.DB) *PostgresOrderRepository {
	return &PostgresOrderRepository{db: db}
}

func (r *PostgresOrderRepository) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return r.db.BeginTx(ctx, nil)
}

func (r *PostgresOrderRepository) Create(ctx context.Context, tx *sql.Tx, order *models.Order) error {
	query := `
		INSERT INTO orders (customer_name, customer_email, total_amount, status)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at
	`
	if tx != nil {
		return tx.QueryRowContext(ctx, query, order.CustomerName, order.CustomerEmail, order.TotalAmount, order.Status).
			Scan(&order.ID, &order.CreatedAt, &order.UpdatedAt)
	}
	return r.db.QueryRowContext(ctx, query, order.CustomerName, order.CustomerEmail, order.TotalAmount, order.Status).
		Scan(&order.ID, &order.CreatedAt, &order.UpdatedAt)
}

func (r *PostgresOrderRepository) CreateItem(ctx context.Context, tx *sql.Tx, item *models.OrderItem) error {
	query := `
		INSERT INTO order_items (order_id, product_id, quantity, unit_price, subtotal)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at
	`
	var createdAt sql.NullTime
	if tx != nil {
		return tx.QueryRowContext(ctx, query, item.OrderID, item.ProductID, item.Quantity, item.UnitPrice, item.Subtotal).
			Scan(&item.ID, &createdAt)
	}
	return r.db.QueryRowContext(ctx, query, item.OrderID, item.ProductID, item.Quantity, item.UnitPrice, item.Subtotal).
		Scan(&item.ID, &createdAt)
}

func (r *PostgresOrderRepository) GetAll(ctx context.Context) ([]models.Order, error) {
	query := `
		SELECT id, customer_name, customer_email, total_amount, status, created_at, updated_at
		FROM orders
		ORDER BY id DESC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error querying orders: %w", err)
	}
	defer rows.Close()

	var orders []models.Order
	for rows.Next() {
		var o models.Order
		if err := rows.Scan(&o.ID, &o.CustomerName, &o.CustomerEmail, &o.TotalAmount, &o.Status, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		orders = append(orders, o)
	}
	return orders, nil
}

func (r *PostgresOrderRepository) GetByID(ctx context.Context, id int) (*models.Order, error) {
	query := `
		SELECT id, customer_name, customer_email, total_amount, status, created_at, updated_at
		FROM orders WHERE id = $1
	`
	var o models.Order
	err := r.db.QueryRowContext(ctx, query, id).Scan(&o.ID, &o.CustomerName, &o.CustomerEmail, &o.TotalAmount, &o.Status, &o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	itemsQuery := `
		SELECT oi.id, oi.order_id, oi.product_id, p.name as product_name, oi.quantity, oi.unit_price, oi.subtotal
		FROM order_items oi
		JOIN products p ON oi.product_id = p.id
		WHERE oi.order_id = $1
	`
	itemRows, err := r.db.QueryContext(ctx, itemsQuery, id)
	if err != nil {
		return nil, fmt.Errorf("error querying order items: %w", err)
	}
	defer itemRows.Close()

	for itemRows.Next() {
		var item models.OrderItem
		if err := itemRows.Scan(&item.ID, &item.OrderID, &item.ProductID, &item.ProductName, &item.Quantity, &item.UnitPrice, &item.Subtotal); err != nil {
			return nil, err
		}
		o.Items = append(o.Items, item)
	}

	return &o, nil
}

func (r *PostgresOrderRepository) UpdateStatus(ctx context.Context, id int, status models.OrderStatus) error {
	query := `UPDATE orders SET status = $1, updated_at = NOW() WHERE id = $2`
	res, err := r.db.ExecContext(ctx, query, status, id)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("order ID %d not found", id)
	}
	return nil
}

func (r *PostgresOrderRepository) GetOrderItems(ctx context.Context, orderID int) ([]models.OrderItem, error) {
	query := `SELECT id, order_id, product_id, quantity, unit_price, subtotal FROM order_items WHERE order_id = $1`
	rows, err := r.db.QueryContext(ctx, query, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.OrderItem
	for rows.Next() {
		var item models.OrderItem
		if err := rows.Scan(&item.ID, &item.OrderID, &item.ProductID, &item.Quantity, &item.UnitPrice, &item.Subtotal); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// MemoryOrderRepository handles in-memory storage fallback
type MemoryOrderRepository struct {
	mu         sync.RWMutex
	nextID     int
	nextItemID int
	orders     map[int]*models.Order
}

func NewMemoryOrderRepository() *MemoryOrderRepository {
	return &MemoryOrderRepository{
		nextID:     1,
		nextItemID: 1,
		orders:     make(map[int]*models.Order),
	}
}

func (r *MemoryOrderRepository) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return nil, nil
}

func (r *MemoryOrderRepository) Create(ctx context.Context, tx *sql.Tx, order *models.Order) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	order.ID = r.nextID
	r.nextID++
	now := time.Now()
	order.CreatedAt = now
	order.UpdatedAt = now
	oCopy := *order
	r.orders[order.ID] = &oCopy
	return nil
}

func (r *MemoryOrderRepository) CreateItem(ctx context.Context, tx *sql.Tx, item *models.OrderItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item.ID = r.nextItemID
	r.nextItemID++
	if o, ok := r.orders[item.OrderID]; ok {
		o.Items = append(o.Items, *item)
	}
	return nil
}

func (r *MemoryOrderRepository) GetAll(ctx context.Context) ([]models.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []models.Order
	for i := r.nextID - 1; i >= 1; i-- {
		if o, ok := r.orders[i]; ok {
			list = append(list, *o)
		}
	}
	return list, nil
}

func (r *MemoryOrderRepository) GetByID(ctx context.Context, id int) (*models.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if o, ok := r.orders[id]; ok {
		oCopy := *o
		return &oCopy, nil
	}
	return nil, nil
}

func (r *MemoryOrderRepository) UpdateStatus(ctx context.Context, id int, status models.OrderStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if o, ok := r.orders[id]; ok {
		o.Status = status
		o.UpdatedAt = time.Now()
		return nil
	}
	return fmt.Errorf("order ID %d not found", id)
}

func (r *MemoryOrderRepository) GetOrderItems(ctx context.Context, orderID int) ([]models.OrderItem, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if o, ok := r.orders[orderID]; ok {
		return o.Items, nil
	}
	return nil, nil
}
