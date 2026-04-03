package example

import (
	"context"
	"net/http"
)

type HandlerFunc = func(context.Context, http.ResponseWriter, *http.Request)
type MiddlewareFunc func(next HandlerFunc) HandlerFunc

type VerifyAuthFunc func(context.Context, *http.Request, AccessPolicy) (context.Context, error)

type MuxOptions struct {
	MaxRequestBodySize *int
}

func ApplyMiddlewares(h HandlerFunc, middlewares ...MiddlewareFunc) http.HandlerFunc {
	for _, m := range middlewares {
		h = m(h)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		h(r.Context(), w, r)
	}
}

type ServerHandler interface {
	GetLibraryV1(context.Context) (*Library, error)
	GetLibraryBookV1(context.Context, *GetBookReq) (*Book, error)
	PostLibraryBookCheckoutV1(context.Context, *CheckoutBookReq) error
}

func CreateMux(h ServerHandler, verifyAuth VerifyAuthFunc, options *MuxOptions, middlewares ...MiddlewareFunc) *http.ServeMux {
	if verifyAuth == nil {
		verifyAuth = func(ctx context.Context, _ *http.Request, _ AccessPolicy) (context.Context, error) {
			return ctx, nil
		}
	}
	if options == nil {
		options = &MuxOptions{}
	}
	m := http.NewServeMux()
	m.HandleFunc("GET /v1/library", ApplyMiddlewares(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		ctx, err := verifyAuth(ctx, r, AccessPolicy{})
		if err != nil {
			HandleReqErr(ctx, err, r, w)
			return
		}
		res, err := h.GetLibraryV1(ctx)
		Respond(ctx, r, w, res, err)
	}, middlewares...))
	m.HandleFunc("GET /v1/library/book", ApplyMiddlewares(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		ctx, err := verifyAuth(ctx, r, AccessPolicy{})
		if err != nil {
			HandleReqErr(ctx, err, r, w)
			return
		}
		req, err := decodeWithMaxBodySize(r, options.MaxRequestBodySize, DecodeGetBookReq)
		if err != nil {
			HandleReqErr(ctx, err, r, w)
			return
		}
		res, err := h.GetLibraryBookV1(ctx, req)
		Respond(ctx, r, w, res, err)
	}, middlewares...))
	m.HandleFunc("POST /v1/library/book-checkout", ApplyMiddlewares(func(ctx context.Context, w http.ResponseWriter, r *http.Request) {
		ctx, err := verifyAuth(ctx, r, AccessPolicy{})
		if err != nil {
			HandleReqErr(ctx, err, r, w)
			return
		}
		req, err := decodeWithMaxBodySize(r, options.MaxRequestBodySize, DecodeCheckoutBookReq)
		if err != nil {
			HandleReqErr(ctx, err, r, w)
			return
		}
		err = h.PostLibraryBookCheckoutV1(ctx, req)
		if err != nil {
			HandleReqErr(ctx, err, r, w)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}, middlewares...))
	return m
}
