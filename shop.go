package shop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
)

// Service
type Service interface {
	CreateProduct(ctx context.Context, p Product) (Product, error)
	GetProduct(ctx context.Context, id string) (Product, error)
	ListProducts(ctx context.Context, f ProductFilter) ([]Product, error)
	UpdateProduct(ctx context.Context, id string, upd ProductUpdate) (Product, error)
	DeleteProduct(ctx context.Context, id string) error
}

// Product defines a single product, represented by its unique ID.
type Product struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Price       Price  `json:"price,omitempty"`
}

type Price float64

func (p Price) String() string {
	return fmt.Sprintf("%.2f", p)
}

// ProductFilter defines the attributes a user can use to filter/limit a subset of results in a request for a list of products.
type ProductFilter struct{}

// ProductUpdate defines the attributes that can be updated in a PATCH request.
type ProductUpdate struct{}

// Handler implements http.Handler and provides an interface for API requests to interact with the ProductService.
type Handler struct {
	service Service
	http.Handler
	logger *slog.Logger
}

// NewHandler returns a new Handler.
func NewHandler(s Service, l *slog.Logger) *Handler {
	h := Handler{
		service: s,
		logger:  l,
	}

	router := mux.NewRouter()
	router.HandleFunc("/products", h.CreateProduct).Methods(http.MethodPost)
	router.HandleFunc("/products", h.ListProducts()).Methods(http.MethodGet)
	router.HandleFunc("/products/{productID}", h.GetProduct).Methods(http.MethodGet)
	router.HandleFunc("/products/{productID}", h.UpdateProduct).Methods(http.MethodPatch)
	router.HandleFunc("/products/{productID}", h.DeleteProduct).Methods(http.MethodDelete)
	h.Handler = router

	return &h
}

func idFromRequest(r *http.Request) string {
	return mux.Vars(r)["productID"]
}

// POST /products
func (h *Handler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	var p Product
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	product, err := h.service.CreateProduct(r.Context(), p)
	if err != nil {
		switch {
		default:
			h.logger.Info("failed to create product", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	url := fmt.Sprintf("/products/%s", product.ID)
	w.Header().Set("Location", url)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(product)
}

// GetProduct will parse and respond to an API request for a specific product defined by its {productID} in the request URL.
// GET /products/{productID}
func (h *Handler) GetProduct(w http.ResponseWriter, r *http.Request) {
	id := idFromRequest(r)

	product, err := h.service.GetProduct(r.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			w.WriteHeader(http.StatusNotFound)
		default:
			h.logger.Info("failed to get product", "id", id, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(product)

}

// ListProducts returns a subset of products defined by the filters in the URL.
// GET /products
func (h *Handler) ListProducts() http.HandlerFunc {
	decoder := schema.NewDecoder()
	decoder.IgnoreUnknownKeys(true)
	return func(w http.ResponseWriter, r *http.Request) {

		var f ProductFilter
		if err := decoder.Decode(&f, r.URL.Query()); err != nil {
			h.logger.Info("failed to decode request", "error", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		products, err := h.service.ListProducts(r.Context(), f)
		if err != nil {
			switch {
			default:
				h.logger.Info("failed to list products", "error", err)
				w.WriteHeader(http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(products)

	}
}

// Update a subset of a products attributes.  The product is identified by its {productID} in the URL.
// PATCH /products/{productID}
func (h *Handler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	id := idFromRequest(r)

	var upd ProductUpdate
	if err := json.NewDecoder(r.Body).Decode(&upd); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	product, err := h.service.UpdateProduct(r.Context(), id, upd)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			w.WriteHeader(http.StatusNotFound)
		default:
			h.logger.Info("failed to update product", "id", id, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(product)
}

// Delete a product identified in the URL.
// DELETE /products/{productID}
func (h *Handler) DeleteProduct(w http.ResponseWriter, r *http.Request) {
	id := idFromRequest(r)

	if err := h.service.DeleteProduct(r.Context(), id); err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			w.WriteHeader(http.StatusNotFound)
		default:
			h.logger.Info("failed to delete product", "id", id, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

var (
	ErrNotFound = errors.New("shop: not found")
)
