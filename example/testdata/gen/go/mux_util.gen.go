package example

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
)

func Respond(ctx context.Context, r *http.Request, w http.ResponseWriter, res Encodable, resultErr error) {
	if resultErr != nil {
		HandleReqErr(ctx, resultErr, r, w)
		return
	}
	if res != nil {
		w.Header().Set("Content-Type", "application/protobuf")
		RespondWithStatus(ctx, w, res.Encode(), http.StatusOK)
		return
	}
	RespondWithStatus(ctx, w, nil, http.StatusOK)
}

func RespondWithStatus(ctx context.Context, w http.ResponseWriter, b []byte, code int) {
	w.WriteHeader(code)
	if len(b) == 0 {
		return
	}
	if _, err := w.Write(b); err != nil {
		slog.ErrorContext(ctx, fmt.Sprintf("writing response body: %v", err))
	}
}

func HandleReqErr(ctx context.Context, err error, r *http.Request, w http.ResponseWriter) {
	if err != nil && len(err.Error()) > 0 {
		slog.ErrorContext(ctx, fmt.Sprintf("%v err: %v", r.URL.Path, err.Error()))
	}
	var httpErr ApiErr
	if !errors.As(err, &httpErr) {
		var httpErrPtr *ApiErr
		if errors.As(err, &httpErrPtr) && httpErrPtr != nil {
			httpErr = *httpErrPtr
		} else {
			httpErr = ApiErr{DisplayErr: "Unknown server error", Code: http.StatusInternalServerError}
		}
	}
	w.Header().Set("Content-Type", "application/protobuf")
	RespondWithStatus(ctx, w, httpErr.Encode(), int(httpErr.Code))
}

func decodeBody[T any](r *http.Request, decode func([]byte) (*T, error)) (*T, error) {
	b, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	return decode(b)
}

func NewApiErr(displayErr string, internalErr string, code int32) ApiErr {
	return ApiErr{DisplayErr: displayErr, InternalErr: internalErr, Code: code}
}

func (e ApiErr) Error() string {
	if e.InternalErr != "" {
		return e.InternalErr
	}
	return e.DisplayErr
}
