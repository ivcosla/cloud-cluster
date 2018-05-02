package cluster

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

type HTTPServer struct {
	srv      *Server
	listener net.Listener
	mux      *http.ServeMux
}

func NewHttpServer(srv *Server) (*HTTPServer, error) {
	srv.logger.Printf("[INFO]: Http server %s", srv.config.HTTPAddr.String())

	ln, err := net.Listen("tcp", srv.config.HTTPAddr.String())
	if err != nil {
		return nil, err
	}

	mux := http.NewServeMux()

	h := &HTTPServer{
		srv:      srv,
		listener: ln,
		mux:      mux,
	}

	h.registerEndpoints()

	go http.Serve(ln, mux)

	return h, nil
}

func (h *HTTPServer) registerEndpoints() {
	h.mux.HandleFunc("/", h.wrap(h.Index))
	h.mux.HandleFunc("/v1/status/peers", h.wrap(h.StatusPeersRequest))
}

func (h *HTTPServer) wrap(handler func(resp http.ResponseWriter, req *http.Request) (interface{}, error)) http.HandlerFunc {
	return func(resp http.ResponseWriter, req *http.Request) {
		reqURL := req.URL.String()

		start := time.Now()
		defer func() {
			h.srv.logger.Printf("[DEBUG] http: Request %v (%v)", reqURL, time.Now().Sub(start))
		}()

		handleErr := func(err error) {
			resp.WriteHeader(http.StatusInternalServerError)
			resp.Write([]byte(err.Error()))
		}

		obj, err := handler(resp, req)
		if err != nil {
			h.srv.logger.Printf("[ERR] http: Request %v, error: %v", reqURL, err)
			handleErr(err)
			return
		}

		if obj == nil {
			return
		}

		buf, err := json.Marshal(obj)
		if err != nil {
			handleErr(err)
			return
		}

		resp.Header().Set("Content-Type", "application/json")
		resp.Write(buf)
	}
}

func (s *HTTPServer) Index(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, fmt.Errorf("")
	}

	return "index", nil
}

func (s *HTTPServer) StatusPeersRequest(resp http.ResponseWriter, req *http.Request) (interface{}, error) {
	if req.Method != "GET" {
		return nil, fmt.Errorf("")
	}

	var peers []string
	if err := s.srv.endpoints.Status.Peers(nil, &peers); err != nil {
		return nil, err
	}

	return peers, nil
}
