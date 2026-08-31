package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"oms/internal/models"
)

type ProductRepository interface {
	GetAll(ctx context.Context) ([]models.Product, error)
	GetByID(ctx context.Context, id int) (*models.Product, error)
	GetByIDTx(ctx context.Context, tx *sql.Tx, id int) (*models.Product, error)
	Create(ctx context.Context, p *models.Product) error
	DeductStock(ctx context.Context, tx *sql.Tx, productID, qty int) error
	RestoreStock(ctx context.Context, tx *sql.Tx, productID, qty int) error
}

// PostgresProductRepository handles PostgreSQL storage
type PostgresProductRepository struct {
	db *sql.DB
}

func NewPostgresProductRepository(db *sql.DB) *PostgresProductRepository {
	return &PostgresProductRepository{db: db}
}

func (r *PostgresProductRepository) GetAll(ctx context.Context) ([]models.Product, error) {
	query := `SELECT id, name, description, price, stock, created_at, updated_at FROM products ORDER BY id ASC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("error querying products: %w", err)
	}
	defer rows.Close()

	var products []models.Product
	for rows.Next() {
		var p models.Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.Stock, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating products: %w", err)
	}
	return products, nil
}

func (r *PostgresProductRepository) GetByID(ctx context.Context, id int) (*models.Product, error) {
	query := `SELECT id, name, description, price, stock, created_at, updated_at FROM products WHERE id = $1`
	var p models.Product
	err := r.db.QueryRowContext(ctx, query, id).Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.Stock, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (r *PostgresProductRepository) GetByIDTx(ctx context.Context, tx *sql.Tx, id int) (*models.Product, error) {
	query := `SELECT id, name, description, price, stock, created_at, updated_at FROM products WHERE id = $1 FOR UPDATE`
	var p models.Product
	err := tx.QueryRowContext(ctx, query, id).Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.Stock, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (r *PostgresProductRepository) Create(ctx context.Context, p *models.Product) error {
	query := `INSERT INTO products (name, description, price, stock) VALUES ($1, $2, $3, $4) RETURNING id, created_at, updated_at`
	return r.db.QueryRowContext(ctx, query, p.Name, p.Description, p.Price, p.Stock).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

func (r *PostgresProductRepository) DeductStock(ctx context.Context, tx *sql.Tx, productID, qty int) error {
	query := `UPDATE products SET stock = stock - $1, updated_at = NOW() WHERE id = $2 AND stock >= $1`
	var res sql.Result
	var err error
	if tx != nil {
		res, err = tx.ExecContext(ctx, query, qty, productID)
	} else {
		res, err = r.db.ExecContext(ctx, query, qty, productID)
	}
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return fmt.Errorf("insufficient stock for product ID %d", productID)
	}
	return nil
}

func (r *PostgresProductRepository) RestoreStock(ctx context.Context, tx *sql.Tx, productID, qty int) error {
	query := `UPDATE products SET stock = stock + $1, updated_at = NOW() WHERE id = $2`
	var err error
	if tx != nil {
		_, err = tx.ExecContext(ctx, query, qty, productID)
	} else {
		_, err = r.db.ExecContext(ctx, query, qty, productID)
	}
	return err
}

// MemoryProductRepository handles in-memory storage fallback
type MemoryProductRepository struct {
	mu       sync.RWMutex
	nextID   int
	products map[int]*models.Product
}

func NewMemoryProductRepository() *MemoryProductRepository {
	repo := &MemoryProductRepository{
		nextID:   1,
		products: make(map[int]*models.Product),
	}
	// Add initial seed data
	now := time.Now()
	seeds := []models.Product{
		{ID: 1, Name: "Banana (per kg)", Description: "Fresh and ripe bananas", Price: 12.90, Stock: 100, CreatedAt: now, UpdatedAt: now},
		{ID: 2, Name: "Grapes (per box)", Description: "Fresh and juicy grapes", Price: 15.50, Stock: 30, CreatedAt: now, UpdatedAt: now},
		{ID: 3, Name: "Apple (per kg)", Description: "Fresh and crisp apples", Price: 8.90, Stock: 50, CreatedAt: now, UpdatedAt: now},
		{ID: 4, Name: "Pineapple (per kg)", Description: "Fresh and juicy pineapples", Price: 7.90, Stock: 40, CreatedAt: now, UpdatedAt: now},
	}
	for _, p := range seeds {
		pCopy := p
		repo.products[p.ID] = &pCopy
		if p.ID >= repo.nextID {
			repo.nextID = p.ID + 1
		}
	}
	return repo
}

func (r *MemoryProductRepository) GetAll(ctx context.Context) ([]models.Product, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var list []models.Product
	for i := 1; i < r.nextID; i++ {
		if p, ok := r.products[i]; ok {
			list = append(list, *p)
		}
	}
	return list, nil
}

func (r *MemoryProductRepository) GetByID(ctx context.Context, id int) (*models.Product, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if p, ok := r.products[id]; ok {
		pCopy := *p
		return &pCopy, nil
	}
	return nil, nil
}

func (r *MemoryProductRepository) GetByIDTx(ctx context.Context, tx *sql.Tx, id int) (*models.Product, error) {
	return r.GetByID(ctx, id)
}

func (r *MemoryProductRepository) Create(ctx context.Context, p *models.Product) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p.ID = r.nextID
	r.nextID++
	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now
	pCopy := *p
	r.products[p.ID] = &pCopy
	return nil
}

func (r *MemoryProductRepository) DeductStock(ctx context.Context, tx *sql.Tx, productID, qty int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.products[productID]
	if !ok {
		return fmt.Errorf("product ID %d not found", productID)
	}
	if p.Stock < qty {
		return fmt.Errorf("insufficient stock for product ID %d", productID)
	}
	p.Stock -= qty
	p.UpdatedAt = time.Now()
	return nil
}

func (r *MemoryProductRepository) RestoreStock(ctx context.Context, tx *sql.Tx, productID, qty int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.products[productID]
	if !ok {
		return fmt.Errorf("product ID %d not found", productID)
	}
	p.Stock += qty
	p.UpdatedAt = time.Now()
	return nil
}
