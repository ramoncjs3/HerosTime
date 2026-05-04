package admin

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"oldbeggar-refactor/internal/config"
	"oldbeggar-refactor/internal/runner"
)

const (
	sessionCookieName = "oldbeggar_admin_session"
	csrfCookieName    = "oldbeggar_admin_csrf"
	sessionTTL        = 12 * time.Hour
	actionTimeout     = 10 * time.Minute
	eventHistoryLimit = 1000

	maxLoginFailures  = 5
	loginFailureTTL   = 15 * time.Minute
	loginBlockTimeout = 15 * time.Minute
)

type Server struct {
	cfg      *config.Config
	adminCfg config.AdminConfig
	app      *runner.App
	assets   http.FileSystem

	httpServer *http.Server

	sessionMu sync.Mutex
	sessions  map[string]sessionInfo

	loginMu       sync.Mutex
	loginFailures map[string]loginFailure

	actionMu sync.Mutex
	action   actionState
	queue    []queuedAction
}

type sessionInfo struct {
	ExpiresAt time.Time
	CSRFToken string
}

type loginFailure struct {
	Failures    int
	FirstFailed time.Time
	LockedUntil time.Time
}

type actionState struct {
	Name       string
	Running    bool
	StartedAt  time.Time
	FinishedAt time.Time
	Error      string
}

type queuedAction struct {
	Name     string
	Label    string
	Run      func(context.Context) error
	QueuedAt time.Time
}

type statusResponse struct {
	Now           string                 `json:"now"`
	App           appStatus              `json:"app"`
	Notifications notificationStatus     `json:"notifications"`
	Variants      []variantStatus        `json:"variants"`
	Expansion     []expansionStatus      `json:"expansion"`
	State         []serverStateStatus    `json:"state"`
	Events        []serverStateStatus    `json:"events"`
	Runtime       runner.RuntimeSnapshot `json:"runtime"`
	Action        actionStatus           `json:"action"`
}

type appStatus struct {
	Name             string         `json:"name"`
	Timezone         string         `json:"timezone"`
	DryRun           bool           `json:"dry_run"`
	StateFile        string         `json:"state_file"`
	StorageType      string         `json:"storage_type"`
	AdminAddr        string         `json:"admin_addr"`
	QQEnabled        bool           `json:"qq_enabled"`
	WxPusherEnabled  bool           `json:"wxpusher_enabled"`
	Schedules        scheduleStatus `json:"schedules"`
	ConfiguredServer int            `json:"configured_server_count"`
}

type scheduleStatus struct {
	BootstrapOnStart bool   `json:"bootstrap_on_start"`
	RefreshSessions  string `json:"refresh_sessions"`
	CheckShop        string `json:"check_shop"`
}

type notificationStatus struct {
	WatchItems []string `json:"watch_items"`
}

type variantStatus struct {
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	Enabled     bool     `json:"enabled"`
	ServerCount int      `json:"server_count"`
	Servers     []string `json:"servers"`
}

type expansionStatus struct {
	VariantName        string `json:"variant_name"`
	Kind               string `json:"kind"`
	Server             string `json:"server"`
	UsernameConfigured bool   `json:"username_configured"`
	PasswordConfigured bool   `json:"password_configured"`
	WxPusherConfigured bool   `json:"wxpusher_configured"`
	QQConfigured       bool   `json:"qq_configured"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
}

type serverStateStatus struct {
	Key       string   `json:"key"`
	Source    string   `json:"source"`
	Server    string   `json:"server"`
	Date      string   `json:"date"`
	Status    string   `json:"status"`
	Items     []string `json:"items,omitempty"`
	UpdatedAt string   `json:"updated_at"`
	CreatedAt string   `json:"created_at,omitempty"`
}

type actionStatus struct {
	Name       string               `json:"name,omitempty"`
	Running    bool                 `json:"running"`
	StartedAt  string               `json:"started_at,omitempty"`
	FinishedAt string               `json:"finished_at,omitempty"`
	Error      string               `json:"error,omitempty"`
	Queued     []queuedActionStatus `json:"queued,omitempty"`
}

type queuedActionStatus struct {
	Name     string `json:"name"`
	QueuedAt string `json:"queued_at"`
}

func New(cfg *config.Config, app *runner.App) (*Server, error) {
	if cfg == nil {
		return nil, errors.New("admin config is nil")
	}
	if app == nil {
		return nil, errors.New("admin app is nil")
	}
	assets, err := webFileSystem()
	if err != nil {
		return nil, err
	}
	return &Server{
		cfg:      cfg,
		adminCfg: cfg.Admin,
		app:      app,
		assets:   assets,
		sessions: make(map[string]sessionInfo),

		loginFailures: make(map[string]loginFailure),
	}, nil
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.route)

	s.httpServer = &http.Server{
		Addr:              s.adminCfg.Addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("admin shutdown failed: %v", err)
		}
	}()

	go func() {
		log.Printf("admin server started: addr=%s user=%s", s.adminCfg.Addr, s.adminCfg.Username)
		if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("admin server failed: %v", err)
		}
	}()

	return nil
}

func (s *Server) route(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/login":
		s.handleAPILogin(w, r)
		return
	case "/login":
		if r.Method == http.MethodPost {
			s.handleLogin(w, r)
			return
		}
		s.serveIndex(w, r)
		return
	case "/logout":
		if !s.isAuthenticated(r) {
			s.requireAuth(w, r)
			return
		}
		s.handleLogout(w, r)
		return
	default:
		if s.isPublicAsset(r.URL.Path) {
			s.serveStatic(w, r)
			return
		}
		if !s.isAuthenticated(r) {
			s.requireAuth(w, r)
			return
		}
		s.handleAuthed(w, r)
	}
}

func (s *Server) handleAPILogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.validateRequestOrigin(w, r) {
		return
	}
	loginKey := s.loginKey(r)
	if retryAfter := s.loginRetryAfter(loginKey); retryAfter > 0 {
		s.writeLoginRateLimited(w, retryAfter)
		return
	}
	username, password, err := loginCredentials(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"ok":    false,
			"error": "登录请求无效",
		})
		return
	}
	if !s.checkCredentials(username, password) {
		if retryAfter := s.recordLoginFailure(loginKey); retryAfter > 0 {
			s.writeLoginRateLimited(w, retryAfter)
			return
		}
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"ok":    false,
			"error": "账号或密码错误",
		})
		return
	}
	s.clearLoginFailures(loginKey)
	if err := s.issueSession(w, r); err != nil {
		log.Printf("admin session create failed: %v", err)
		writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"ok":    false,
			"error": "登录失败",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if s.isAuthenticated(r) {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		s.serveIndex(w, r)
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		if !s.validateRequestOrigin(w, r) {
			return
		}
		loginKey := s.loginKey(r)
		if retryAfter := s.loginRetryAfter(loginKey); retryAfter > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
			http.Error(w, "too many login attempts", http.StatusTooManyRequests)
			return
		}
		username := r.FormValue("username")
		password := r.FormValue("password")
		if !s.checkCredentials(username, password) {
			if retryAfter := s.recordLoginFailure(loginKey); retryAfter > 0 {
				w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())))
				http.Error(w, "too many login attempts", http.StatusTooManyRequests)
				return
			}
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		s.clearLoginFailures(loginKey)
		if err := s.issueSession(w, r); err != nil {
			log.Printf("admin session create failed: %v", err)
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}
		http.Redirect(w, r, "/", http.StatusFound)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !s.validateWriteRequest(w, r) {
		return
	}
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		s.sessionMu.Lock()
		delete(s.sessions, cookie.Value)
		s.sessionMu.Unlock()
	}
	http.SetCookie(w, expiredCookie(r))
	http.SetCookie(w, expiredCSRFCookie(r))
	http.Redirect(w, r, "/login", http.StatusFound)
}

func (s *Server) handleAuthed(w http.ResponseWriter, r *http.Request) {
	if isWriteMethod(r.Method) && !s.validateWriteRequest(w, r) {
		return
	}
	switch {
	case r.URL.Path == "/" && r.Method == http.MethodGet:
		s.serveIndex(w, r)
	case r.URL.Path == "/api/status" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, s.status())
	case r.URL.Path == "/api/logout" && r.Method == http.MethodPost:
		s.clearSession(w, r)
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
	case r.URL.Path == "/api/expansion" && r.Method == http.MethodPost:
		s.handleExpansionAdd(w, r)
	case r.URL.Path == "/api/expansion/delete" && r.Method == http.MethodPost:
		s.handleExpansionDelete(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/actions/") && r.Method == http.MethodPost:
		name := strings.TrimPrefix(r.URL.Path, "/api/actions/")
		s.handleAction(w, r, name)
	case r.Method == http.MethodGet:
		s.serveStaticOrIndex(w, r)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) handleExpansionAdd(w http.ResponseWriter, r *http.Request) {
	var req config.ExpansionAccountInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "请求无效"})
		return
	}
	record, err := s.cfg.AddExpansionAccount(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	s.app.ApplyExpansionDestinations(record)
	action, actionErr := s.startAction("refresh")
	response := map[string]interface{}{
		"ok":        true,
		"expansion": s.expansionRecordStatus(record),
		"action":    action,
	}
	if actionErr != nil {
		response["warning"] = "新区已保存，当前已有任务运行，稍后请手动刷新登录"
	}
	writeJSON(w, http.StatusCreated, response)
}

func (s *Server) handleExpansionDelete(w http.ResponseWriter, r *http.Request) {
	var req config.ExpansionAccountInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": "请求无效"})
		return
	}
	record, err := s.cfg.RemoveExpansionAccount(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{"ok": false, "error": err.Error()})
		return
	}
	s.app.RemoveExpansionDestinations(record)
	action, actionErr := s.startAction("refresh")
	response := map[string]interface{}{
		"ok":        true,
		"expansion": s.expansionRecordStatus(record),
		"action":    action,
	}
	if actionErr != nil {
		response["warning"] = "新区已移除，当前已有任务运行，稍后请手动刷新登录"
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleAction(w http.ResponseWriter, r *http.Request, name string) {
	status, err := s.startAction(name)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]interface{}{
			"ok":     false,
			"error":  err.Error(),
			"action": status,
		})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]interface{}{
		"ok":     true,
		"action": status,
	})
}

func (s *Server) startAction(name string) (actionStatus, error) {
	run, label, ok := s.actionFunc(name)
	if !ok {
		return s.actionSnapshot(), fmt.Errorf("unknown action %q", name)
	}
	action := queuedAction{Name: name, Label: label, Run: run, QueuedAt: time.Now()}

	s.actionMu.Lock()
	if s.action.Running {
		s.enqueueActionLocked(action)
		status := s.actionStatusLocked()
		s.actionMu.Unlock()
		return status, nil
	}
	s.startActionLocked(action)
	status := s.actionStatusLocked()
	s.actionMu.Unlock()

	go s.runAction(action)

	return status, nil
}

func (s *Server) enqueueActionLocked(action queuedAction) {
	for _, queued := range s.queue {
		if queued.Name == action.Name {
			return
		}
	}
	s.queue = append(s.queue, action)
}

func (s *Server) startActionLocked(action queuedAction) {
	s.action = actionState{
		Name:      action.Label,
		Running:   true,
		StartedAt: time.Now(),
	}
}

func (s *Server) runAction(action queuedAction) {
	ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
	defer cancel()
	err := action.Run(ctx)

	var next queuedAction
	hasNext := false
	s.actionMu.Lock()
	s.action.Running = false
	s.action.FinishedAt = time.Now()
	if err != nil {
		s.action.Error = err.Error()
		log.Printf("admin action failed: name=%s error=%v", action.Label, err)
	} else {
		s.action.Error = ""
		log.Printf("admin action completed: name=%s", action.Label)
	}
	if len(s.queue) > 0 {
		next = s.queue[0]
		s.queue = s.queue[1:]
		s.startActionLocked(next)
		hasNext = true
	}
	s.actionMu.Unlock()

	if hasNext {
		go s.runAction(next)
	}
}

func (s *Server) actionFunc(name string) (func(context.Context) error, string, bool) {
	switch name {
	case "bootstrap":
		return s.app.Bootstrap, "完整检查", true
	case "refresh":
		return s.app.RefreshSessions, "刷新登录", true
	case "check":
		return s.app.CheckShop, "检查商店", true
	default:
		return nil, "", false
	}
}

func (s *Server) status() statusResponse {
	variants := s.variants()
	configuredServerCount := 0
	for _, variant := range variants {
		if variant.Enabled {
			configuredServerCount += variant.ServerCount
		}
	}
	return statusResponse{
		Now: time.Now().Format(time.RFC3339),
		App: appStatus{
			Name:            s.cfg.App.Name,
			Timezone:        s.cfg.App.Timezone,
			DryRun:          s.cfg.App.DryRun,
			StateFile:       s.cfg.App.StateFile,
			StorageType:     s.cfg.Storage.Type,
			AdminAddr:       s.adminCfg.Addr,
			QQEnabled:       strings.TrimSpace(s.cfg.QQ.APIURL) != "",
			WxPusherEnabled: s.cfg.WxPusher.Enabled,
			Schedules: scheduleStatus{
				BootstrapOnStart: s.cfg.Schedules.BootstrapOnStart,
				RefreshSessions:  s.cfg.Schedules.RefreshSessions,
				CheckShop:        s.cfg.Schedules.CheckShop,
			},
			ConfiguredServer: configuredServerCount,
		},
		Notifications: notificationStatus{
			WatchItems: append([]string(nil), s.cfg.Notifications.WatchItems...),
		},
		Variants:  variants,
		Expansion: s.expansionRows(),
		State:     s.stateRows(),
		Events:    s.eventRows(eventHistoryLimit),
		Runtime:   s.app.Snapshot(),
		Action:    s.actionSnapshot(),
	}
}

func (s *Server) variants() []variantStatus {
	variants := s.cfg.VariantsSnapshot()
	out := make([]variantStatus, 0, len(variants))
	for _, variant := range variants {
		enabled := true
		if variant.Enabled != nil {
			enabled = *variant.Enabled
		}
		servers := make([]string, 0, len(variant.Accounts))
		for _, account := range variant.Accounts {
			if strings.TrimSpace(account.Server) != "" {
				servers = append(servers, account.Server)
			}
		}
		sortServers(servers)
		out = append(out, variantStatus{
			Name:        variant.Name,
			Kind:        variant.Kind,
			Enabled:     enabled,
			ServerCount: len(servers),
			Servers:     servers,
		})
	}
	return out
}

func (s *Server) stateRows() []serverStateStatus {
	snapshot := s.app.StateSnapshot()
	rows := make([]serverStateStatus, 0, len(snapshot))
	for key, value := range snapshot {
		source, server := splitStateKey(key)
		rows = append(rows, serverStateStatus{
			Key:       key,
			Source:    source,
			Server:    server,
			Date:      value.Date,
			Status:    value.Status,
			Items:     append([]string(nil), value.Items...),
			UpdatedAt: value.UpdatedAt.Format(time.RFC3339),
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Date != rows[j].Date {
			return rows[i].Date > rows[j].Date
		}
		if rows[i].Server != rows[j].Server {
			return serverLess(rows[i].Server, rows[j].Server)
		}
		return rows[i].Source < rows[j].Source
	})
	return rows
}

func (s *Server) expansionRows() []expansionStatus {
	records := s.cfg.ExpansionSnapshot()
	rows := make([]expansionStatus, 0, len(records))
	for _, record := range records {
		rows = append(rows, s.expansionRecordStatus(record))
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].VariantName != rows[j].VariantName {
			return rows[i].VariantName < rows[j].VariantName
		}
		return serverLess(rows[i].Server, rows[j].Server)
	})
	return rows
}

func (s *Server) expansionRecordStatus(record config.ExpansionRecord) expansionStatus {
	return expansionStatus{
		VariantName:        record.VariantName,
		Kind:               record.Kind,
		Server:             record.Server,
		UsernameConfigured: strings.TrimSpace(record.Username) != "",
		PasswordConfigured: strings.TrimSpace(record.Password) != "",
		WxPusherConfigured: record.WxPusherTopicID > 0,
		QQConfigured:       strings.TrimSpace(record.QQGroupID) != "",
		CreatedAt:          record.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          record.UpdatedAt.Format(time.RFC3339),
	}
}

func (s *Server) eventRows(limit int) []serverStateStatus {
	events := s.app.StateEvents(limit)
	rows := make([]serverStateStatus, 0, len(events))
	for _, event := range events {
		source, server := splitStateKey(event.Key)
		createdAt := event.CreatedAt.Format(time.RFC3339)
		rows = append(rows, serverStateStatus{
			Key:       event.Key,
			Source:    source,
			Server:    server,
			Date:      event.Date,
			Status:    event.Status,
			Items:     append([]string(nil), event.Items...),
			UpdatedAt: createdAt,
			CreatedAt: createdAt,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].CreatedAt != rows[j].CreatedAt {
			return rows[i].CreatedAt > rows[j].CreatedAt
		}
		if rows[i].Date != rows[j].Date {
			return rows[i].Date > rows[j].Date
		}
		if rows[i].Server != rows[j].Server {
			return serverLess(rows[i].Server, rows[j].Server)
		}
		return rows[i].Source < rows[j].Source
	})
	return rows
}

func splitStateKey(key string) (string, string) {
	if source, server, ok := strings.Cut(key, ":"); ok {
		return source, server
	}
	return "oldbeggar", key
}

func (s *Server) actionSnapshot() actionStatus {
	s.actionMu.Lock()
	defer s.actionMu.Unlock()
	return s.actionStatusLocked()
}

func (s *Server) actionStatusLocked() actionStatus {
	status := actionStatus{
		Name:    s.action.Name,
		Running: s.action.Running,
		Error:   s.action.Error,
	}
	if !s.action.StartedAt.IsZero() {
		status.StartedAt = s.action.StartedAt.Format(time.RFC3339)
	}
	if !s.action.FinishedAt.IsZero() {
		status.FinishedAt = s.action.FinishedAt.Format(time.RFC3339)
	}
	if len(s.queue) > 0 {
		status.Queued = make([]queuedActionStatus, 0, len(s.queue))
		for _, item := range s.queue {
			status.Queued = append(status.Queued, queuedActionStatus{
				Name:     item.Label,
				QueuedAt: item.QueuedAt.Format(time.RFC3339),
			})
		}
	}
	return status
}

func (s *Server) isAuthenticated(r *http.Request) bool {
	if username, password, ok := r.BasicAuth(); ok && s.checkCredentials(username, password) {
		return true
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}
	now := time.Now()
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	session, ok := s.sessions[cookie.Value]
	if !ok {
		return false
	}
	if now.After(session.ExpiresAt) {
		delete(s.sessions, cookie.Value)
		return false
	}
	session.ExpiresAt = now.Add(sessionTTL)
	s.sessions[cookie.Value] = session
	return true
}

func (s *Server) requireAuth(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		w.Header().Set("WWW-Authenticate", `Basic realm="oldbeggar admin"`)
		writeJSON(w, http.StatusUnauthorized, map[string]interface{}{
			"ok":    false,
			"error": "unauthorized",
		})
		return
	}
	http.Redirect(w, r, "/login", http.StatusFound)
}

func (s *Server) validateWriteRequest(w http.ResponseWriter, r *http.Request) bool {
	if !s.validateRequestOrigin(w, r) {
		return false
	}
	if !s.validCSRFToken(r) {
		writeJSON(w, http.StatusForbidden, map[string]interface{}{
			"ok":    false,
			"error": "invalid csrf token",
		})
		return false
	}
	return true
}

func (s *Server) validateRequestOrigin(w http.ResponseWriter, r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	referer := strings.TrimSpace(r.Header.Get("Referer"))
	if origin == "" && referer == "" {
		return true
	}
	if origin != "" && s.sameOrigin(origin, r) {
		return true
	}
	if origin == "" && referer != "" && s.sameOrigin(referer, r) {
		return true
	}
	writeJSON(w, http.StatusForbidden, map[string]interface{}{
		"ok":    false,
		"error": "invalid request origin",
	})
	return false
}

func (s *Server) validCSRFToken(r *http.Request) bool {
	if username, password, ok := r.BasicAuth(); ok && s.checkCredentials(username, password) {
		return true
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}
	token := strings.TrimSpace(r.Header.Get("X-CSRF-Token"))
	if token == "" {
		token = strings.TrimSpace(r.Header.Get("X-XSRF-TOKEN"))
	}
	if token == "" {
		return false
	}
	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()
	session, ok := s.sessions[cookie.Value]
	if !ok {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(session.CSRFToken)) == 1
}

func (s *Server) sameOrigin(raw string, r *http.Request) bool {
	if strings.EqualFold(raw, "null") {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return false
	}
	return strings.EqualFold(parsed.Host, requestHost(r)) &&
		strings.EqualFold(parsed.Scheme, requestScheme(r))
}

func requestHost(r *http.Request) string {
	if forwarded := firstForwardedHeader(r.Header.Get("X-Forwarded-Host")); forwarded != "" {
		return forwarded
	}
	return r.Host
}

func requestScheme(r *http.Request) string {
	if forwarded := firstForwardedHeader(r.Header.Get("X-Forwarded-Proto")); forwarded != "" {
		return strings.ToLower(forwarded)
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func firstForwardedHeader(value string) string {
	if first, _, ok := strings.Cut(value, ","); ok {
		return strings.TrimSpace(first)
	}
	return strings.TrimSpace(value)
}

func isWriteMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func (s *Server) loginRetryAfter(key string) time.Duration {
	now := time.Now()
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	failure, ok := s.loginFailures[key]
	if !ok {
		return 0
	}
	if !failure.LockedUntil.IsZero() {
		if now.Before(failure.LockedUntil) {
			return failure.LockedUntil.Sub(now)
		}
		delete(s.loginFailures, key)
		return 0
	}
	if now.Sub(failure.FirstFailed) > loginFailureTTL {
		delete(s.loginFailures, key)
	}
	return 0
}

func (s *Server) recordLoginFailure(key string) time.Duration {
	now := time.Now()
	s.loginMu.Lock()
	defer s.loginMu.Unlock()
	failure := s.loginFailures[key]
	if failure.FirstFailed.IsZero() || now.Sub(failure.FirstFailed) > loginFailureTTL {
		failure = loginFailure{FirstFailed: now}
	}
	failure.Failures++
	if failure.Failures >= maxLoginFailures {
		failure.LockedUntil = now.Add(loginBlockTimeout)
	}
	s.loginFailures[key] = failure
	if failure.LockedUntil.IsZero() {
		return 0
	}
	return failure.LockedUntil.Sub(now)
}

func (s *Server) clearLoginFailures(key string) {
	s.loginMu.Lock()
	delete(s.loginFailures, key)
	s.loginMu.Unlock()
}

func (s *Server) writeLoginRateLimited(w http.ResponseWriter, retryAfter time.Duration) {
	seconds := int(retryAfter.Seconds())
	if seconds < 1 {
		seconds = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(seconds))
	writeJSON(w, http.StatusTooManyRequests, map[string]interface{}{
		"ok":          false,
		"error":       "too many login attempts",
		"retry_after": seconds,
	})
}

func (s *Server) loginKey(r *http.Request) string {
	if forwarded := firstForwardedHeader(r.Header.Get("X-Forwarded-For")); forwarded != "" {
		return forwarded
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}

func loginCredentials(r *http.Request) (string, string, error) {
	if strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "application/json") {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return "", "", err
		}
		return req.Username, req.Password, nil
	}
	if err := r.ParseForm(); err != nil {
		return "", "", err
	}
	return r.FormValue("username"), r.FormValue("password"), nil
}

func (s *Server) checkCredentials(username, password string) bool {
	expectedUser := []byte(s.adminCfg.Username)
	expectedPass := []byte(s.adminCfg.Password)
	gotUser := []byte(username)
	gotPass := []byte(password)
	userOK := subtle.ConstantTimeCompare(gotUser, expectedUser) == 1
	passOK := subtle.ConstantTimeCompare(gotPass, expectedPass) == 1
	return userOK && passOK
}

func (s *Server) issueSession(w http.ResponseWriter, r *http.Request) error {
	token, err := randomToken()
	if err != nil {
		return err
	}
	csrfToken, err := randomToken()
	if err != nil {
		return err
	}
	expiresAt := time.Now().Add(sessionTTL)

	s.sessionMu.Lock()
	s.sessions[token] = sessionInfo{
		ExpiresAt: expiresAt,
		CSRFToken: csrfToken,
	}
	s.sessionMu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(sessionTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecureRequest(r),
	})
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    csrfToken,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   int(sessionTTL.Seconds()),
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecureRequest(r),
	})
	return nil
}

func randomToken() (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func expiredCookie(r *http.Request) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecureRequest(r),
	}
}

func expiredCSRFCookie(r *http.Request) *http.Cookie {
	return &http.Cookie{
		Name:     csrfCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
		SameSite: http.SameSiteLaxMode,
		Secure:   isSecureRequest(r),
	}
}

func (s *Server) clearSession(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookieName); err == nil {
		s.sessionMu.Lock()
		delete(s.sessions, cookie.Value)
		s.sessionMu.Unlock()
	}
	http.SetCookie(w, expiredCookie(r))
	http.SetCookie(w, expiredCSRFCookie(r))
}

func isSecureRequest(r *http.Request) bool {
	return strings.EqualFold(requestScheme(r), "https")
}

func (s *Server) isPublicAsset(urlPath string) bool {
	return strings.HasPrefix(urlPath, "/assets/") || urlPath == "/favicon.ico"
}

func (s *Server) serveStaticOrIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/" || r.URL.Path == "/login" {
		s.serveIndex(w, r)
		return
	}
	clean := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if clean == "." || clean == "" {
		s.serveIndex(w, r)
		return
	}
	file, err := s.assets.Open(clean)
	if err != nil {
		s.serveIndex(w, r)
		return
	}
	_ = file.Close()
	s.serveStatic(w, r)
}

func (s *Server) serveStatic(w http.ResponseWriter, r *http.Request) {
	http.FileServer(s.assets).ServeHTTP(w, r)
}

func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	file, err := s.assets.Open("index.html")
	if err != nil {
		http.Error(w, "admin web assets are missing", http.StatusInternalServerError)
		return
	}
	defer file.Close()
	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, "admin web assets are unreadable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func writeJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func sortServers(servers []string) {
	sort.Slice(servers, func(i, j int) bool {
		return serverLess(servers[i], servers[j])
	})
}

func serverLess(a, b string) bool {
	ap, an, as := serverParts(a)
	bp, bn, bs := serverParts(b)
	if ap != bp {
		return ap < bp
	}
	if an != bn {
		return an < bn
	}
	if as != bs {
		return as < bs
	}
	return a < b
}

func serverParts(code string) (string, int, string) {
	code = strings.TrimSpace(code)
	lastDigit := -1
	for i := len(code) - 1; i >= 0; i-- {
		if code[i] < '0' || code[i] > '9' {
			break
		}
		lastDigit = i
	}
	if lastDigit < 0 {
		return code, 0, ""
	}
	number, err := strconv.Atoi(code[lastDigit:])
	if err != nil {
		return code, 0, ""
	}
	return code[:lastDigit], number, code[lastDigit:]
}
