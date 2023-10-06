package shop

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"golang.org/x/exp/slog"
)

// Service
type Service interface {
	GetProduct(ctx context.Context, id string) (Product, error)
	ListProducts(ctx context.Context, f ProductFilter) ([]Product, error)
}

// Product defines a single product, represented by its unique ID.
type Product struct{}

// ProductFilter
type ProductFilter struct{}

// Handler
type Handler struct {
	service Service
	http.Handler
	logger *slog.Logger
}

// NewHandler returns a http.Handler to parse API requests and send JSON responses using data obtained from the underlying Service implementation.
func NewHandler(s Service, l *slog.Logger) *Handler {
	h := Handler{
		service: s,
		logger:  l,
	}

	router := mux.NewRouter()
	router.HandleFunc("/products", h.ListProducts).Methods(http.MethodGet)
	router.HandleFunc("/products/{productID}", h.GetProduct).Methods(http.MethodGet)
	h.Handler = router

	return &h
}

func idFromRequest(r *http.Request) string {
	return mux.Vars(r)["productID"]
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
func (h *Handler) ListProducts(w http.ResponseWriter, r *http.Request) {
	var f ProductFilter
	decoder := schema.NewDecoder()
	decoder.IgnoreUnknownKeys(true)

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

var (
	ErrNotFound = errors.New("shop: not found")
)
