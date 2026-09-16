// Command stakewars-simnet-player runs the production StakeWars session stack
// without a graphical window. It exists for isolated simnet acceptance tests:
// the process connects with the same mTLS credentials, accepts the same
// dashboard invitations, persists the same game state, and sends the same BR
// turn messages as the desktop game.
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/karamble/dcrstakewars/internal/bridgeconn"
	"github.com/karamble/dcrstakewars/internal/session"
	"github.com/karamble/dcrstakewars/pkg/replay"
	"github.com/karamble/dcrstakewars/pkg/sim"
)

type status struct {
	Connected       bool   `json:"connected"`
	Status          string `json:"status"`
	Error           string `json:"error,omitempty"`
	Match           string `json:"match,omitempty"`
	Phase           string `json:"phase,omitempty"`
	Mine            uint8  `json:"mine"`
	CanFund         bool   `json:"canFund"`
	WorldAgreed     bool   `json:"worldAgreed"`
	Height          uint32 `json:"height"`
	Turn            uint32 `json:"turn,omitempty"`
	ActiveSeat      uint8  `json:"activeSeat,omitempty"`
	Winner          int8   `json:"winner,omitempty"`
	HeadHash        string `json:"headHash,omitempty"`
	Settlement      string `json:"settlement,omitempty"`
	FinancialTables int    `json:"financialTables"`
}

func snapshot(c *session.Controller) status {
	v := c.Snapshot()
	s := status{
		Connected:       v.Connected,
		Status:          v.Status,
		Error:           v.Error,
		Match:           v.Match,
		Phase:           v.Phase,
		Mine:            v.Mine,
		CanFund:         v.CanFund,
		WorldAgreed:     v.WorldAgreed,
		Height:          v.Height,
		Settlement:      v.Settlement,
		FinancialTables: len(v.Tables),
		Winner:          -1,
	}
	if v.Head != nil {
		s.Turn = v.Head.Turn
		s.ActiveSeat = v.Head.ActiveSeat()
		s.Winner = v.Head.Winner
		hash := replay.Hash(v.Head)
		s.HeadHash = hex.EncodeToString(hash[:])
	}
	return s
}

func turnFrom(head *sim.State, mode string) (*sim.State, []sim.Input, error) {
	if head == nil {
		return nil, nil, errors.New("match has no agreed head")
	}
	if head.Phase == sim.Ended {
		return nil, nil, errors.New("match already ended")
	}
	after := head.Clone()
	var inputs []sim.Input
	acted := false
	for tick := 0; tick < 20000 && after.Turn == head.Turn && after.Phase != sim.Ended; tick++ {
		var next []sim.Input
		if after.Phase == sim.Playing && !acted {
			acted = true
			kind, param := sim.Surrender, int32(0)
			if mode == "fire" {
				kind, param = sim.Fire, 650
			}
			next = []sim.Input{{Tick: after.Tick, Seat: after.ActiveSeat(), Kind: kind, Param: param}}
			inputs = append(inputs, next...)
		}
		if _, err := sim.Step(after, next); err != nil {
			return nil, nil, err
		}
	}
	if after.Turn == head.Turn && after.Phase != sim.Ended {
		return nil, nil, errors.New("turn did not end within deterministic step limit")
	}
	return after, inputs, nil
}

func writeJSON(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}

func run() error {
	var dataDir, bridgePath, listen string
	flag.StringVar(&dataDir, "appdata", "", "isolated StakeWars data directory")
	flag.StringVar(&bridgePath, "bridge-config", "", "bridge.json path (default: APPDATA/bridge.json)")
	flag.StringVar(&listen, "listen", "127.0.0.1:0", "loopback control address")
	flag.Parse()
	if strings.TrimSpace(dataDir) == "" {
		return errors.New("-appdata is required")
	}
	var err error
	dataDir, err = filepath.Abs(dataDir)
	if err != nil {
		return err
	}
	if bridgePath == "" {
		bridgePath = filepath.Join(dataDir, "bridge.json")
	}
	cfg, err := bridgeconn.Load(bridgePath)
	if err != nil {
		return err
	}
	if err = cfg.Validate(); err != nil {
		return err
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	controller, err := session.Open(ctx, cfg, dataDir)
	if err != nil {
		return err
	}
	controllerDone := make(chan error, 1)
	go func() { controllerDone <- controller.Run(ctx) }()

	listener, err := net.Listen("tcp", listen)
	if err != nil {
		cancel()
		<-controllerDone
		return err
	}
	host, _, splitErr := net.SplitHostPort(listener.Addr().String())
	ip := net.ParseIP(strings.Trim(host, "[]"))
	if splitErr != nil || ip == nil || !ip.IsLoopback() {
		_ = listener.Close()
		cancel()
		<-controllerDone
		return errors.New("control listener must be loopback")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, snapshot(controller))
	})
	mux.HandleFunc("POST /fund", func(w http.ResponseWriter, _ *http.Request) {
		if err := controller.Action("fund"); err != nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]bool{"queued": true})
	})
	mux.HandleFunc("POST /advance", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Mode string `json:"mode"`
		}
		if r.Body != nil {
			_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1024)).Decode(&req)
		}
		if req.Mode == "" {
			req.Mode = "surrender"
		}
		if req.Mode != "fire" && req.Mode != "surrender" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "mode must be fire or surrender"})
			return
		}
		v := controller.Snapshot()
		if v.Head == nil || v.Match == "" {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "match is not ready"})
			return
		}
		if v.Head.ActiveSeat() != v.Mine {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "not this player's turn"})
			return
		}
		after, inputs, err := turnFrom(v.Head, req.Mode)
		if err == nil {
			err = controller.Submit(v.Match, after, inputs)
		}
		if err != nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]any{"queued": true, "turn": v.Head.Turn, "mode": req.Mode})
	})

	server := &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	serverDone := make(chan error, 1)
	go func() { serverDone <- server.Serve(listener) }()
	fmt.Printf("STAKEWARS_SIMNET_PLAYER=%s\n", listener.Addr())

	select {
	case err = <-controllerDone:
		cancel()
		_ = server.Close()
		if err != nil && !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	case err = <-serverDone:
		cancel()
		<-controllerDone
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = server.Shutdown(shutdownCtx)
		<-controllerDone
		return nil
	}
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
