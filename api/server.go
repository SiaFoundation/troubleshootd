package api

import (
	"context"
	"net/http"
	"runtime"
	"time"

	"go.sia.tech/jape"
	"go.sia.tech/troubleshootd/build"
	"go.sia.tech/troubleshootd/troubleshoot"
)

// A Troubleshooter is an interface that defines the methods for testing a host.
type Troubleshooter interface {
	TestHost(ctx context.Context, host troubleshoot.Host) (troubleshoot.Result, error)
}

type (
	server struct {
		t Troubleshooter
	}
)

func (s *server) handleGETState(jc jape.Context) {
	jc.Encode(StateResponse{
		Version:   build.Version(),
		Commit:    build.Commit(),
		OS:        runtime.GOOS,
		BuildTime: build.Time(),
	})
}

func (s *server) handlePOSTTroubleshoot(jc jape.Context) {
	var req troubleshoot.Host
	if jc.Decode(&req) != nil {
		return
	}

	ctx, cancel := context.WithTimeout(jc.Request.Context(), 30*time.Second)
	defer cancel()

	resp, err := s.t.TestHost(ctx, req)
	if err != nil {
		jc.Error(err, http.StatusInternalServerError)
		return
	}
	jc.Encode(resp)
}

// NewHandler returns a new HTTP handler for the API.
func NewHandler(t Troubleshooter) http.Handler {
	s := &server{
		t: t,
	}
	return jape.Mux(map[string]jape.Handler{
		"GET /state":         s.handleGETState,
		"POST /troubleshoot": s.handlePOSTTroubleshoot,
	})
}
