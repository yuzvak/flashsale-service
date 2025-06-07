package monitoring

import (
	"context"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MetricsServer struct {
	server *http.Server
}

func NewMetricsServer(addr string) *MetricsServer {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return &MetricsServer{
		server: server,
	}
}

func (s *MetricsServer) Start() error {
	return s.server.ListenAndServe()
}

func (s *MetricsServer) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

func WrapHandler(handler http.Handler) http.Handler {
	return NewHTTPMetricsMiddleware(handler)
}

func WrapHandlerFunc(handlerFunc http.HandlerFunc) http.Handler {
	return NewHTTPMetricsMiddleware(handlerFunc)
}

func RegisterMetricsEndpoint(mux *http.ServeMux) {
	mux.Handle("/metrics", promhttp.Handler())
}
