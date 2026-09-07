package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/efauncodes/equipment-manager-backend/db"
	"github.com/efauncodes/equipment-manager-backend/domain"
	"github.com/efauncodes/equipment-manager-backend/repository"
	"github.com/efauncodes/equipment-manager-backend/service"
)

const defaultHTTPAddr = ":8080"

func main() {
	database, err := db.Open("")
	if err != nil {
		log.Fatal(err)
	}
	defer database.Close()
	store := repository.NewStore(database)
	svc := service.New(store)
	if err := seedConfiguredAdmins(context.Background(), svc, store, os.Getenv("ADMIN_EMAILS")); err != nil {
		log.Fatal(err)
	}
	log.Printf("sqlite database ready at %s", db.ConfiguredPath())

	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = defaultHTTPAddr
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           newHandlerWithReadiness(svc, database.PingContext),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(shutdown)

	go func() {
		<-shutdown
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
		}
	}()

	log.Printf("backend listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func rootHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"service": "equipment-manager-backend",
		"status":  "ok",
	})
}

func seedConfiguredAdmins(ctx context.Context, svc *service.Service, store *repository.Store, raw string) error {
	for _, value := range strings.Split(raw, ",") {
		email := domain.NormalizeEmail(value)
		if email == "" {
			continue
		}
		user, err := store.Users().FindByEmail(ctx, email)
		if err == nil {
			if user.Role != domain.RoleAdmin {
				return errors.New("ADMIN_EMAILS contains a non-admin user")
			}
			continue
		}
		if !errors.Is(err, domain.ErrNotFound) {
			return err
		}
		name := strings.Split(email, "@")[0]
		if _, err := svc.CreateUser(ctx, email, name, domain.RoleAdmin, true); err != nil {
			return err
		}
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("writing response failed: %v", err)
	}
}
