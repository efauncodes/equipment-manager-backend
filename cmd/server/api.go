package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/efauncodes/equipment-manager-backend/domain"
	"github.com/efauncodes/equipment-manager-backend/service"
)

var (
	errUnauthorized = errors.New("unauthorized")
	errForbidden    = errors.New("forbidden")
)

const (
	corsAllowedOrigin  = "https://equipment.sentient-octopus.dev"
	corsAllowedMethods = "GET, POST, PATCH, OPTIONS"
	corsAllowedHeaders = "Authorization, Content-Type, X-Request-ID"
)

type readinessProbe func(context.Context) error

type apiServer struct {
	svc          *service.Service
	development  bool
	exposeTokens bool
	readiness    readinessProbe
}

type responseMeta struct {
	RequestID string `json:"request_id"`
	LocalTime string `json:"local_time"`
}

type successResponse struct {
	Data any          `json:"data"`
	Meta responseMeta `json:"meta"`
}

type errorResponse struct {
	Error apiError     `json:"error"`
	Meta  responseMeta `json:"meta"`
}

type apiError struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

type userResponse struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	IsActive    bool   `json:"is_active"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type equipmentResponse struct {
	ID           string  `json:"id"`
	SerialNumber string  `json:"serial_number"`
	Type         string  `json:"type"`
	Size         string  `json:"size"`
	PurchaseDate *string `json:"purchase_date,omitempty"`
	Manufacturer string  `json:"manufacturer"`
	Status       string  `json:"status"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}

type issuanceResponse struct {
	ID                 string  `json:"id"`
	EquipmentID        string  `json:"equipment_id"`
	MemberID           string  `json:"member_id"`
	IssuedByUserID     string  `json:"issued_by_user_id"`
	IssuedAt           *string `json:"issued_at,omitempty"`
	ConfirmationStatus string  `json:"confirmation_status"`
	MemberConfirmedAt  *string `json:"member_confirmed_at,omitempty"`
	ReturnedAt         *string `json:"returned_at,omitempty"`
	ClosedAt           *string `json:"closed_at,omitempty"`
	ClosureReason      *string `json:"closure_reason,omitempty"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
}

type returnResponse struct {
	ID                 string  `json:"id"`
	IssuanceID         string  `json:"issuance_id"`
	InitiatedByUserID  string  `json:"initiated_by_user_id"`
	ReturnedAt         *string `json:"returned_at,omitempty"`
	ConfirmationStatus string  `json:"confirmation_status"`
	MemberConfirmedAt  *string `json:"member_confirmed_at,omitempty"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
}

type historyResponse struct {
	ID              string  `json:"id"`
	EquipmentID     string  `json:"equipment_id"`
	EventType       string  `json:"event_type"`
	FromStatus      *string `json:"from_status,omitempty"`
	ToStatus        *string `json:"to_status,omitempty"`
	IssuanceID      *string `json:"issuance_id,omitempty"`
	ReturnID        *string `json:"return_id,omitempty"`
	ChangedByUserID string  `json:"changed_by_user_id"`
	OccurredAt      string  `json:"occurred_at"`
	Note            *string `json:"note,omitempty"`
}

type adminMagicLinkRequest struct {
	Email string `json:"email"`
}
type consumeRequest struct {
	Token string `json:"token"`
}
type createMemberRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	IsActive    *bool  `json:"is_active,omitempty"`
}
type updateMemberRequest struct {
	Email       *string `json:"email,omitempty"`
	DisplayName *string `json:"display_name,omitempty"`
	IsActive    *bool   `json:"is_active,omitempty"`
}
type createEquipmentRequest struct {
	SerialNumber string  `json:"serial_number"`
	Type         string  `json:"type"`
	Size         string  `json:"size"`
	PurchaseDate *string `json:"purchase_date,omitempty"`
	Manufacturer string  `json:"manufacturer"`
}
type updateEquipmentRequest struct {
	SerialNumber *string `json:"serial_number,omitempty"`
	Type         *string `json:"type,omitempty"`
	Size         *string `json:"size,omitempty"`
	PurchaseDate *string `json:"purchase_date,omitempty"`
	Manufacturer *string `json:"manufacturer,omitempty"`
}
type issueRequest struct {
	EquipmentID string `json:"equipment_id"`
	MemberID    string `json:"member_id"`
}
type returnRequest struct {
	IssuanceID string `json:"issuance_id"`
}
type noteRequest struct {
	Note string `json:"note,omitempty"`
}

func newHandler(services ...*service.Service) http.Handler {
	var svc *service.Service
	if len(services) > 0 {
		svc = services[0]
	}
	return newHandlerWithReadiness(svc, nil)

}

func newHandlerWithConfig(svc *service.Service, development, exposeTokens bool) http.Handler {
	return newHandlerWithConfigAndReadiness(svc, development, exposeTokens, nil)
}

func newHandlerWithReadiness(svc *service.Service, readiness readinessProbe) http.Handler {
	return newHandlerWithConfigAndReadiness(svc, strings.EqualFold(os.Getenv("APP_ENV"), "development"), strings.EqualFold(os.Getenv("DEV_EXPOSE_TOKENS"), "true"), readiness)
}

func newHandlerWithConfigAndReadiness(svc *service.Service, development, exposeTokens bool, readiness readinessProbe) http.Handler {
	a := &apiServer{svc: svc, development: development, exposeTokens: exposeTokens, readiness: readiness}
	return a.handler()
}

func (a *apiServer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler)
	mux.HandleFunc("/readyz", a.readinessHandler)
	mux.HandleFunc("/", a.dispatch)
	return a.cors(mux)
}

func (a *apiServer) readinessHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if a.readiness == nil || a.readiness(r.Context()) != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *apiServer) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/") {
			next.ServeHTTP(w, r)
			return
		}

		origin := r.Header.Get("Origin")
		if origin == "" {
			next.ServeHTTP(w, r)
			return
		}
		if origin != corsAllowedOrigin || r.Header.Get("Cookie") != "" {
			a.corsDenied(w, r)
			return
		}

		if r.Method == http.MethodOptions {
			if !corsMethodAllowed(r.Header.Get("Access-Control-Request-Method")) || !corsHeadersAllowed(r.Header.Get("Access-Control-Request-Headers")) {
				a.corsDenied(w, r)
				return
			}
			setCORSHeaders(w, origin)
			w.Header().Set("Access-Control-Allow-Methods", corsAllowedMethods)
			w.Header().Set("Access-Control-Allow-Headers", corsAllowedHeaders)
			w.Header().Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		setCORSHeaders(w, origin)
		next.ServeHTTP(w, r)
	})
}

func (a *apiServer) corsDenied(w http.ResponseWriter, r *http.Request) {
	a.writeErrorStatus(w, r, http.StatusForbidden, apiError{Code: "FORBIDDEN", Message: "cors request denied"})
}

func setCORSHeaders(w http.ResponseWriter, origin string) {
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Vary", "Origin")
}

func corsMethodAllowed(method string) bool {
	switch strings.TrimSpace(method) {
	case http.MethodGet, http.MethodPost, http.MethodPatch, http.MethodOptions:
		return true
	default:
		return false
	}
}

func corsHeadersAllowed(value string) bool {
	if strings.TrimSpace(value) == "" {
		return true
	}
	for _, header := range strings.Split(value, ",") {
		switch strings.ToLower(strings.TrimSpace(header)) {
		case "authorization", "content-type", "x-request-id":
		default:
			return false
		}
	}
	return true
}

func (a *apiServer) dispatch(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" {
		rootHandler(w, r)
		return
	}
	if a.svc == nil {
		a.writeError(w, r, errors.New("service unavailable"))
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/api/v1/") {
		a.writeErrorStatus(w, r, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "resource not found"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/")
	switch {
	case path == "auth/admin/magic-links":
		a.adminMagicLink(w, r)
	case path == "auth/consume":
		a.consume(w, r)
	case path == "auth/logout":
		a.logout(w, r)
	case path == "me":
		a.me(w, r)
	case path == "members" || strings.HasPrefix(path, "members/"):
		a.members(w, r, strings.TrimPrefix(path, "members"))
	case path == "equipment" || strings.HasPrefix(path, "equipment/"):
		a.equipment(w, r, strings.TrimPrefix(path, "equipment"))
	case path == "issuances" || strings.HasPrefix(path, "issuances/"):
		a.issuances(w, r, strings.TrimPrefix(path, "issuances"))
	case path == "returns" || strings.HasPrefix(path, "returns/"):
		a.returns(w, r, strings.TrimPrefix(path, "returns"))
	default:
		a.writeErrorStatus(w, r, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "resource not found"})
	}
}

func (a *apiServer) adminMagicLink(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		a.methodNotAllowed(w, r)
		return
	}
	var input adminMagicLinkRequest
	if !a.decode(w, r, &input) {
		return
	}
	if strings.TrimSpace(input.Email) == "" {
		a.writeError(w, r, fmt.Errorf("%w: email is required", domain.ErrInvalid))
		return
	}
	user, err := a.svc.FindUserByEmail(r.Context(), input.Email)
	if errors.Is(err, domain.ErrNotFound) || errors.Is(err, domain.ErrInactive) || (err == nil && user.Role != domain.RoleAdmin) {
		a.writeSuccessStatus(w, r, http.StatusAccepted, map[string]any{"accepted": true})
		return
	}
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	link, err := a.svc.CreateLoginLink(r.Context(), user.ID)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	data := map[string]any{"accepted": true, "expires_at": link.Link.ExpiresAt}
	if a.development && a.exposeTokens {
		data["token"] = link.RawToken
	}
	a.writeSuccessStatus(w, r, http.StatusAccepted, data)
}

func (a *apiServer) consume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		a.methodNotAllowed(w, r)
		return
	}
	var input consumeRequest
	if !a.decode(w, r, &input) {
		return
	}
	if strings.TrimSpace(input.Token) == "" {
		a.writeError(w, r, fmt.Errorf("%w: token is required", domain.ErrInvalid))
		return
	}
	session, loginErr := a.svc.RedeemLoginLink(r.Context(), input.Token)
	if loginErr == nil {
		a.writeSuccess(w, r, map[string]any{"session_token": session.RawToken, "user": toUser(session.User)})
		return
	}
	event, confirmErr := a.svc.RedeemConfirmationLink(r.Context(), input.Token)
	if confirmErr == nil {
		a.writeSuccess(w, r, map[string]any{"event": event})
		return
	}
	if !errors.Is(loginErr, domain.ErrConflict) {
		a.writeError(w, r, loginErr)
		return
	}
	a.writeError(w, r, confirmErr)
}

func (a *apiServer) me(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.methodNotAllowed(w, r)
		return
	}
	user, _, ok := a.authenticated(w, r)
	if ok {
		a.writeSuccess(w, r, toUser(user))
	}
}

func (a *apiServer) logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		a.methodNotAllowed(w, r)
		return
	}
	_, raw, ok := a.authenticated(w, r)
	if !ok {
		return
	}
	if err := a.svc.RevokeSession(r.Context(), raw); err != nil {
		a.writeError(w, r, err)
		return
	}
	a.writeSuccess(w, r, map[string]bool{"logged_out": true})
}

func (a *apiServer) members(w http.ResponseWriter, r *http.Request, suffix string) {
	actor, _, ok := a.requireAdmin(w, r)
	if !ok {
		return
	}
	parts := splitSuffix(suffix)
	if len(parts) == 0 {
		switch r.Method {
		case http.MethodGet:
			items, err := a.svc.ListMembers(r.Context())
			if err != nil {
				a.writeError(w, r, err)
				return
			}
			a.writeSuccess(w, r, userList(items))
		case http.MethodPost:
			var input createMemberRequest
			if !a.decode(w, r, &input) {
				return
			}
			active := true
			if input.IsActive != nil {
				active = *input.IsActive
			}
			user, err := a.svc.CreateUser(r.Context(), input.Email, input.DisplayName, domain.RoleMember, active)
			if err != nil {
				a.writeError(w, r, err)
				return
			}
			a.writeSuccessStatus(w, r, http.StatusCreated, toUser(user))
		default:
			a.methodNotAllowed(w, r)
		}
		return
	}
	if len(parts) != 1 {
		a.writeErrorStatus(w, r, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "member not found"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		user, err := a.svc.GetUser(r.Context(), parts[0])
		if err != nil {
			a.writeError(w, r, err)
			return
		}
		if user.Role != domain.RoleMember {
			a.writeError(w, r, domain.ErrNotFound)
			return
		}
		a.writeSuccess(w, r, toUser(user))
	case http.MethodPatch:
		var input updateMemberRequest
		if !a.decode(w, r, &input) {
			return
		}
		user, err := a.svc.UpdateMember(r.Context(), actor.ID, parts[0], service.UpdateMemberInput{Email: input.Email, DisplayName: input.DisplayName, IsActive: input.IsActive})
		if err != nil {
			a.writeError(w, r, err)
			return
		}
		a.writeSuccess(w, r, toUser(user))
	default:
		a.methodNotAllowed(w, r)
	}
}

func (a *apiServer) equipment(w http.ResponseWriter, r *http.Request, suffix string) {
	actor, _, ok := a.requireAdmin(w, r)
	if !ok {
		return
	}
	parts := splitSuffix(suffix)
	if len(parts) == 0 {
		switch r.Method {
		case http.MethodGet:
			includeWrittenOff, err := optionalBool(r.URL.Query().Get("include_written_off"))
			if err != nil {
				a.writeError(w, r, err)
				return
			}
			items, err := a.svc.ListEquipment(r.Context(), includeWrittenOff)
			if err != nil {
				a.writeError(w, r, err)
				return
			}
			a.writeSuccess(w, r, equipmentList(items))
		case http.MethodPost:
			var input createEquipmentRequest
			if !a.decode(w, r, &input) {
				return
			}
			item, err := a.svc.CreateEquipment(r.Context(), actor.ID, service.CreateEquipmentInput{SerialNumber: input.SerialNumber, Type: input.Type, Size: input.Size, PurchaseDate: input.PurchaseDate, Manufacturer: input.Manufacturer})
			if err != nil {
				a.writeError(w, r, err)
				return
			}
			a.writeSuccessStatus(w, r, http.StatusCreated, toEquipment(item))
		default:
			a.methodNotAllowed(w, r)
		}
		return
	}
	if len(parts) == 2 {
		switch parts[1] {
		case "history":
			if r.Method != http.MethodGet {
				a.methodNotAllowed(w, r)
				return
			}
			if _, err := a.svc.GetEquipment(r.Context(), parts[0]); err != nil {
				a.writeError(w, r, err)
				return
			}
			history, err := a.svc.History(r.Context(), parts[0])
			if err != nil {
				a.writeError(w, r, err)
				return
			}
			result := make([]historyResponse, 0, len(history))
			for _, item := range history {
				result = append(result, toHistory(item))
			}
			a.writeSuccess(w, r, result)
		case "write-off":
			if r.Method != http.MethodPost {
				a.methodNotAllowed(w, r)
				return
			}
			var input noteRequest
			if !a.decode(w, r, &input) {
				return
			}
			item, err := a.svc.WriteOff(r.Context(), parts[0], actor.ID, input.Note)
			if err != nil {
				a.writeError(w, r, err)
				return
			}
			a.writeSuccess(w, r, toEquipment(item))
		default:
			a.writeErrorStatus(w, r, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "resource not found"})
		}
		return
	}
	if len(parts) != 1 {
		a.writeErrorStatus(w, r, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "equipment not found"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		item, err := a.svc.GetEquipment(r.Context(), parts[0])
		if err != nil {
			a.writeError(w, r, err)
			return
		}
		a.writeSuccess(w, r, toEquipment(item))
	case http.MethodPatch:
		var input updateEquipmentRequest
		if !a.decode(w, r, &input) {
			return
		}
		var purchaseDate **string
		if input.PurchaseDate != nil {
			purchaseDate = &input.PurchaseDate
		}
		item, err := a.svc.UpdateEquipment(r.Context(), actor.ID, parts[0], service.UpdateEquipmentInput{SerialNumber: input.SerialNumber, Type: input.Type, Size: input.Size, PurchaseDate: purchaseDate, Manufacturer: input.Manufacturer})
		if err != nil {
			a.writeError(w, r, err)
			return
		}
		a.writeSuccess(w, r, toEquipment(item))
	default:
		a.methodNotAllowed(w, r)
	}
}

func (a *apiServer) issuances(w http.ResponseWriter, r *http.Request, suffix string) {
	parts := splitSuffix(suffix)
	if len(parts) == 0 {
		actor, _, ok := a.requireAdmin(w, r)
		if !ok {
			return
		}
		switch r.Method {
		case http.MethodGet:
			items, err := a.svc.ListIssuances(r.Context())
			if err != nil {
				a.writeError(w, r, err)
				return
			}
			result := make([]issuanceResponse, 0, len(items))
			for _, item := range items {
				result = append(result, toIssuance(item))
			}
			a.writeSuccess(w, r, result)
		case http.MethodPost:
			var input issueRequest
			if !a.decode(w, r, &input) {
				return
			}
			item, err := a.svc.Issue(r.Context(), input.EquipmentID, input.MemberID, actor.ID)
			if err != nil {
				a.writeError(w, r, err)
				return
			}
			link, err := a.svc.CreateIssuanceConfirmationLink(r.Context(), item.ID)
			if err != nil {
				a.writeError(w, r, err)
				return
			}
			data := map[string]any{"issuance": toIssuance(item), "expires_at": link.Link.ExpiresAt}
			if a.development && a.exposeTokens {
				data["confirmation_token"] = link.RawToken
			}
			a.writeSuccessStatus(w, r, http.StatusCreated, data)
		default:
			a.methodNotAllowed(w, r)
		}
		return
	}
	if len(parts) != 2 || parts[1] != "confirm" || r.Method != http.MethodPost {
		a.writeErrorStatus(w, r, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "resource not found"})
		return
	}
	var input consumeRequest
	if !a.decodeOptional(w, r, &input) {
		return
	}
	if input.Token != "" {
		if _, err := a.svc.RedeemConfirmationLinkFor(r.Context(), input.Token, parts[0], ""); err != nil {
			a.writeError(w, r, err)
			return
		}
		item, err := a.svc.GetIssuance(r.Context(), parts[0])
		if err != nil {
			a.writeError(w, r, err)
			return
		}
		a.writeSuccess(w, r, toIssuance(item))
		return
	}
	member, _, ok := a.requireMember(w, r)
	if !ok {
		return
	}
	item, err := a.svc.ConfirmIssuance(r.Context(), parts[0], member.ID)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	a.writeSuccess(w, r, toIssuance(item))
}

func (a *apiServer) returns(w http.ResponseWriter, r *http.Request, suffix string) {
	parts := splitSuffix(suffix)
	if len(parts) == 0 {
		actor, _, ok := a.requireAdmin(w, r)
		if !ok {
			return
		}
		switch r.Method {
		case http.MethodGet:
			items, err := a.svc.ListReturns(r.Context())
			if err != nil {
				a.writeError(w, r, err)
				return
			}
			result := make([]returnResponse, 0, len(items))
			for _, item := range items {
				result = append(result, toReturn(item))
			}
			a.writeSuccess(w, r, result)
		case http.MethodPost:
			var input returnRequest
			if !a.decode(w, r, &input) {
				return
			}
			item, err := a.svc.StartReturn(r.Context(), input.IssuanceID, actor.ID)
			if err != nil {
				a.writeError(w, r, err)
				return
			}
			link, err := a.svc.CreateReturnConfirmationLink(r.Context(), item.ID)
			if err != nil {
				a.writeError(w, r, err)
				return
			}
			data := map[string]any{"return": toReturn(item), "expires_at": link.Link.ExpiresAt}
			if a.development && a.exposeTokens {
				data["confirmation_token"] = link.RawToken
			}
			a.writeSuccessStatus(w, r, http.StatusCreated, data)
		default:
			a.methodNotAllowed(w, r)
		}
		return
	}
	if len(parts) != 2 || parts[1] != "confirm" || r.Method != http.MethodPost {
		a.writeErrorStatus(w, r, http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "resource not found"})
		return
	}
	var input consumeRequest
	if !a.decodeOptional(w, r, &input) {
		return
	}
	if input.Token != "" {
		if _, err := a.svc.RedeemConfirmationLinkFor(r.Context(), input.Token, "", parts[0]); err != nil {
			a.writeError(w, r, err)
			return
		}
		item, err := a.svc.GetReturn(r.Context(), parts[0])
		if err != nil {
			a.writeError(w, r, err)
			return
		}
		a.writeSuccess(w, r, toReturn(item))
		return
	}
	member, _, ok := a.requireMember(w, r)
	if !ok {
		return
	}
	item, err := a.svc.ConfirmReturn(r.Context(), parts[0], member.ID)
	if err != nil {
		a.writeError(w, r, err)
		return
	}
	a.writeSuccess(w, r, toReturn(item))
}

func (a *apiServer) authenticated(w http.ResponseWriter, r *http.Request) (domain.User, string, bool) {
	raw := bearerToken(r)
	if raw == "" {
		a.writeError(w, r, errUnauthorized)
		return domain.User{}, "", false
	}
	user, err := a.svc.ValidateSession(r.Context(), raw)
	if err != nil {
		a.writeError(w, r, errUnauthorized)
		return domain.User{}, "", false
	}
	return user, raw, true
}

func (a *apiServer) requireAdmin(w http.ResponseWriter, r *http.Request) (domain.User, string, bool) {
	user, raw, ok := a.authenticated(w, r)
	if !ok {
		return domain.User{}, "", false
	}
	if user.Role != domain.RoleAdmin {
		a.writeError(w, r, errForbidden)
		return domain.User{}, "", false
	}
	return user, raw, true
}

func (a *apiServer) requireMember(w http.ResponseWriter, r *http.Request) (domain.User, string, bool) {
	user, raw, ok := a.authenticated(w, r)
	if !ok {
		return domain.User{}, "", false
	}
	if user.Role != domain.RoleMember {
		a.writeError(w, r, errForbidden)
		return domain.User{}, "", false
	}
	return user, raw, true
}

func bearerToken(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(value) < 8 || !strings.EqualFold(value[:7], "Bearer ") {
		return ""
	}
	return strings.TrimSpace(value[7:])
}

func (a *apiServer) decode(w http.ResponseWriter, r *http.Request, target any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := decoder.Decode(target); err != nil {
		a.writeError(w, r, fmt.Errorf("%w: invalid JSON body", domain.ErrInvalid))
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		a.writeError(w, r, fmt.Errorf("%w: request body must contain one JSON value", domain.ErrInvalid))
		return false
	}
	return true
}

func (a *apiServer) decodeOptional(w http.ResponseWriter, r *http.Request, target any) bool {
	if r.Body == http.NoBody || r.ContentLength == 0 {
		return true
	}
	return a.decode(w, r, target)
}

func (a *apiServer) methodNotAllowed(w http.ResponseWriter, r *http.Request) {
	a.writeErrorStatus(w, r, http.StatusMethodNotAllowed, apiError{Code: "METHOD_NOT_ALLOWED", Message: "method not allowed"})
}

func (a *apiServer) writeSuccess(w http.ResponseWriter, r *http.Request, data any) {
	a.writeSuccessStatus(w, r, http.StatusOK, data)
}

func (a *apiServer) writeSuccessStatus(w http.ResponseWriter, r *http.Request, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(successResponse{Data: data, Meta: a.meta(r)})
}

func (a *apiServer) writeError(w http.ResponseWriter, r *http.Request, err error) {
	status, value := classifyError(err)
	a.writeErrorStatus(w, r, status, value)
}

func (a *apiServer) writeErrorStatus(w http.ResponseWriter, r *http.Request, status int, value apiError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{Error: value, Meta: a.meta(r)})
}

func (a *apiServer) meta(r *http.Request) responseMeta {
	requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
	if requestID == "" || len(requestID) > 128 {
		requestID = newRequestID()
	}
	return responseMeta{RequestID: requestID, LocalTime: time.Now().In(apiLocation()).Format(service.TimestampLayout)}
}

func classifyError(err error) (int, apiError) {
	switch {
	case errors.Is(err, errUnauthorized):
		return http.StatusUnauthorized, apiError{Code: "UNAUTHORIZED", Message: "authentication required"}
	case errors.Is(err, errForbidden):
		return http.StatusForbidden, apiError{Code: "FORBIDDEN", Message: "insufficient permissions"}
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound, apiError{Code: "NOT_FOUND", Message: "resource not found"}
	case errors.Is(err, domain.ErrInvalid):
		return http.StatusBadRequest, apiError{Code: "VALIDATION_ERROR", Message: "request is invalid"}
	case errors.Is(err, domain.ErrExpired):
		return http.StatusUnauthorized, apiError{Code: "TOKEN_EXPIRED", Message: "token expired"}
	case errors.Is(err, domain.ErrUsed):
		return http.StatusConflict, apiError{Code: "TOKEN_USED", Message: "token already used"}
	case errors.Is(err, domain.ErrUnavailable):
		return http.StatusConflict, apiError{Code: "EQUIPMENT_UNAVAILABLE", Message: "equipment is unavailable"}
	case errors.Is(err, domain.ErrConflict), errors.Is(err, domain.ErrAlreadyClosed):
		return http.StatusConflict, apiError{Code: "CONFLICT", Message: "request conflicts with current state"}
	case errors.Is(err, domain.ErrInactive):
		return http.StatusForbidden, apiError{Code: "INACTIVE_USER", Message: "user is inactive"}
	default:
		return http.StatusInternalServerError, apiError{Code: "INTERNAL_ERROR", Message: "internal server error"}
	}
}

func splitSuffix(value string) []string {
	value = strings.Trim(value, "/")
	if value == "" {
		return nil
	}
	parts := strings.Split(value, "/")
	for _, part := range parts {
		if part == "" {
			return nil
		}
	}
	return parts
}

func optionalBool(value string) (bool, error) {
	if value == "" {
		return false, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%w: query parameter must be true or false", domain.ErrInvalid)
	}
	return parsed, nil
}

func apiLocation() *time.Location {
	location, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		return time.FixedZone("Europe/Berlin", 3600)
	}
	return location
}

func newRequestID() string {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return fmt.Sprintf("request-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(value)
}

func toUser(value domain.User) userResponse {
	return userResponse{ID: value.ID, Email: value.Email, DisplayName: value.DisplayName, Role: value.Role, IsActive: value.IsActive, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

func userList(values []domain.User) []userResponse {
	result := make([]userResponse, 0, len(values))
	for _, value := range values {
		result = append(result, toUser(value))
	}
	return result
}

func toEquipment(value domain.Equipment) equipmentResponse {
	return equipmentResponse{ID: value.ID, SerialNumber: value.SerialNumber, Type: value.Type, Size: value.Size, PurchaseDate: value.PurchaseDate, Manufacturer: value.Manufacturer, Status: value.Status, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

func equipmentList(values []domain.Equipment) []equipmentResponse {
	result := make([]equipmentResponse, 0, len(values))
	for _, value := range values {
		result = append(result, toEquipment(value))
	}
	return result
}

func toIssuance(value domain.Issuance) issuanceResponse {
	return issuanceResponse{ID: value.ID, EquipmentID: value.EquipmentID, MemberID: value.MemberID, IssuedByUserID: value.IssuedByUserID, IssuedAt: value.IssuedAt, ConfirmationStatus: value.ConfirmationStatus, MemberConfirmedAt: value.MemberConfirmedAt, ReturnedAt: value.ReturnedAt, ClosedAt: value.ClosedAt, ClosureReason: value.ClosureReason, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

func toReturn(value domain.Return) returnResponse {
	return returnResponse{ID: value.ID, IssuanceID: value.IssuanceID, InitiatedByUserID: value.InitiatedByUserID, ReturnedAt: value.ReturnedAt, ConfirmationStatus: value.ConfirmationStatus, MemberConfirmedAt: value.MemberConfirmedAt, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

func toHistory(value domain.History) historyResponse {
	return historyResponse{ID: value.ID, EquipmentID: value.EquipmentID, EventType: value.EventType, FromStatus: value.FromStatus, ToStatus: value.ToStatus, IssuanceID: value.IssuanceID, ReturnID: value.ReturnID, ChangedByUserID: value.ChangedByUserID, OccurredAt: value.OccurredAt, Note: value.Note}
}
