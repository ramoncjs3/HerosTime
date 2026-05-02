package runner

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"oldbeggar-refactor/internal/auth"
	"oldbeggar-refactor/internal/catalog"
	"oldbeggar-refactor/internal/config"
	"oldbeggar-refactor/internal/game"
	"oldbeggar-refactor/internal/httputil"
	"oldbeggar-refactor/internal/qq"
	"oldbeggar-refactor/internal/state"
	"oldbeggar-refactor/internal/wxpusher"
)

type App struct {
	cfg      *config.Config
	location *time.Location

	resolver *game.ServerResolver
	game     *game.Client
	qq       *qq.Client
	wxpusher *wxpusher.Client
	state    *state.Store
	http     *httputil.Client

	jobMu      sync.Mutex
	sessionsMu sync.RWMutex
	sessions   []*game.Session
}

const (
	oldBeggarSource  = "oldbeggar"
	noticeShopSource = "notice_shop"
)

type shopCheck struct {
	source    string
	label     string
	npcID     int
	watchOnly bool
}

func New(cfg *config.Config) (*App, error) {
	location, err := time.LoadLocation(cfg.App.Timezone)
	if err != nil {
		return nil, err
	}
	httpClient := httputil.New(cfg.HTTP)
	itemCatalog, err := catalog.Load(cfg.Catalog.ItemFile, cfg.Catalog.ItemNameFile)
	if err != nil {
		return nil, err
	}
	stateStore, err := state.Open(cfg.App.StateFile)
	if err != nil {
		return nil, err
	}
	return &App{
		cfg:      cfg,
		location: location,
		resolver: game.NewServerResolver(httpClient),
		game:     game.NewClient(httpClient, itemCatalog),
		qq:       qq.NewClient(httpClient, cfg.QQ),
		wxpusher: wxpusher.NewClient(httpClient, cfg.WxPusher),
		state:    stateStore,
		http:     httpClient,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	if a.cfg.Schedules.BootstrapOnStart {
		if err := a.SelfCheck(ctx); err != nil {
			return err
		}
	}

	scheduler := cron.New(
		cron.WithSeconds(),
		cron.WithLocation(a.location),
		cron.WithChain(cron.SkipIfStillRunning(cron.DefaultLogger)),
	)
	if _, err := scheduler.AddFunc(a.cfg.Schedules.RefreshSessions, func() {
		if err := a.SelfCheck(ctx); err != nil {
			log.Printf("self-check job failed: %v", err)
		}
	}); err != nil {
		return fmt.Errorf("add refresh schedule: %w", err)
	}
	if _, err := scheduler.AddFunc(a.cfg.Schedules.CheckShop, func() {
		if err := a.CheckShop(ctx); err != nil {
			log.Printf("check job failed: %v", err)
		}
	}); err != nil {
		return fmt.Errorf("add check schedule: %w", err)
	}

	scheduler.Start()
	log.Printf("oldbeggar started: refresh=%q check=%q dry_run=%v", a.cfg.Schedules.RefreshSessions, a.cfg.Schedules.CheckShop, a.cfg.App.DryRun)
	<-ctx.Done()
	stopCtx := scheduler.Stop()
	<-stopCtx.Done()
	return nil
}

func (a *App) Bootstrap(ctx context.Context) error {
	a.jobMu.Lock()
	defer a.jobMu.Unlock()

	if err := a.refreshSessions(ctx); err != nil {
		return err
	}
	return a.checkShopWithEventPrecheck(ctx)
}

func (a *App) RefreshSessions(ctx context.Context) error {
	a.jobMu.Lock()
	defer a.jobMu.Unlock()
	return a.refreshSessions(ctx)
}

func (a *App) SelfCheck(ctx context.Context) error {
	a.jobMu.Lock()
	defer a.jobMu.Unlock()
	if err := a.refreshSessions(ctx); err != nil {
		return err
	}
	log.Printf("self-check completed: sessions=%d", len(a.sessionSnapshot()))
	return nil
}

func (a *App) CheckShop(ctx context.Context) error {
	a.jobMu.Lock()
	defer a.jobMu.Unlock()
	if len(a.sessionSnapshot()) == 0 {
		if err := a.refreshSessions(ctx); err != nil {
			return err
		}
	}
	return a.checkShopWithEventPrecheck(ctx)
}

func (a *App) checkShopWithEventPrecheck(ctx context.Context) error {
	var firstErr error
	if err := a.markExistingEventOver(ctx); err != nil {
		log.Printf("event precheck completed with errors: %v", err)
		rememberFirstErr(&firstErr, err)
	}
	if err := a.checkShop(ctx); err != nil {
		log.Printf("shop check completed with errors: %v", err)
		rememberFirstErr(&firstErr, err)
	}
	return firstErr
}

func (a *App) refreshSessions(ctx context.Context) error {
	var next []*game.Session
	var firstErr error
	skip := func(err error) {
		log.Printf("skip session: %v", err)
		rememberFirstErr(&firstErr, err)
	}

	for _, variant := range a.cfg.EnabledVariants() {
		provider, err := auth.NewProvider(variant.Kind, variant.Auth, a.http)
		if err != nil {
			skip(err)
			continue
		}
		endpoints, err := a.resolver.Resolve(ctx, variant)
		if err != nil {
			skip(fmt.Errorf("resolve %s servers: %w", variant.Name, err))
			continue
		}

		credentialCache := map[string]auth.Credentials{}
		cacheCredentials := variant.Kind != "h5"
		for _, rawAccount := range variant.Accounts {
			accountCfg := variant.AccountFor(rawAccount)
			account := auth.Account{Username: accountCfg.Username, Password: accountCfg.Password}
			if accountCfg.Server == "" {
				skip(fmt.Errorf("variant %s has account with empty server", variant.Name))
				continue
			}
			endpoint, ok := endpoints[accountCfg.Server]
			if !ok || endpoint.QuickLoginURL == "" || endpoint.GameURL == "" {
				skip(fmt.Errorf("variant %s server %s not found", variant.Name, accountCfg.Server))
				continue
			}
			var creds auth.Credentials
			if cacheCredentials {
				cacheKey := account.Username + "\x00" + account.Password
				var ok bool
				creds, ok = credentialCache[cacheKey]
				if !ok {
					var err error
					creds, err = provider.Login(ctx, account)
					if err != nil {
						skip(fmt.Errorf("%s %s login: %w", variant.Name, accountCfg.Server, err))
						continue
					}
					credentialCache[cacheKey] = creds
				}
			} else {
				var err error
				creds, err = provider.Login(ctx, account)
				if err != nil {
					skip(fmt.Errorf("%s %s login: %w", variant.Name, accountCfg.Server, err))
					continue
				}
			}
			session := &game.Session{
				VariantName:   variant.Name,
				Kind:          variant.Kind,
				ServerCode:    accountCfg.Server,
				Account:       account,
				Credentials:   creds,
				QuickLoginURL: endpoint.QuickLoginURL,
				GameURL:       endpoint.GameURL,
			}
			err = a.quickLoginWithRetry(ctx, session)
			if variant.Kind == "h5" && errors.Is(err, game.ErrEmptyQuickLogin) {
				err = a.quickLoginWithCredentialRefresh(ctx, provider, account, session)
			}
			if err != nil {
				skip(fmt.Errorf("%s %s quicklogin: %w", variant.Name, accountCfg.Server, err))
				continue
			}
			log.Printf("session ready: variant=%s server=%s role_id=%s", variant.Name, accountCfg.Server, floatForLog(session.RoleID))
			next = append(next, session)
		}
	}

	if len(next) == 0 {
		if firstErr != nil {
			return firstErr
		}
		return fmt.Errorf("no sessions were created")
	}

	a.sessionsMu.Lock()
	a.sessions = next
	a.sessionsMu.Unlock()

	if firstErr != nil {
		log.Printf("WARNING: session refresh completed with failures: ok=%d failed_at_least=1 first_err=%v", len(next), firstErr)
	}
	return nil
}

func (a *App) quickLoginWithRetry(ctx context.Context, session *game.Session) error {
	attempts := a.cfg.HTTP.Retries + 1
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		if err := a.game.QuickLogin(ctx, session); err != nil {
			lastErr = err
		} else {
			return nil
		}
		if attempt >= attempts {
			break
		}
		timer := time.NewTimer(a.cfg.HTTP.Backoff.Duration * time.Duration(attempt))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return lastErr
}

func (a *App) quickLoginWithCredentialRefresh(ctx context.Context, provider auth.Provider, account auth.Account, session *game.Session) error {
	attempts := a.cfg.HTTP.Retries + 1
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		creds, err := provider.Login(ctx, account)
		if err != nil {
			lastErr = err
		} else {
			session.Credentials = creds
			if err := a.game.QuickLogin(ctx, session); err != nil {
				lastErr = err
			} else {
				return nil
			}
		}
		if attempt >= attempts {
			break
		}
		timer := time.NewTimer(a.cfg.HTTP.Backoff.Duration * time.Duration(attempt))
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return lastErr
}

func (a *App) markExistingEventOver(ctx context.Context) error {
	date := a.today()
	var firstErr error
	for _, session := range a.sessionSnapshot() {
		key := sourceStateKey(oldBeggarSource, session.ServerCode)
		if a.state.Done(key, date) || a.state.Done(session.ServerCode, date) {
			continue
		}
		over, err := a.game.EventIsOver(ctx, session)
		if err != nil {
			rememberFirstErr(&firstErr, fmt.Errorf("%s event precheck: %w", session.ServerCode, err))
			continue
		}
		if over {
			log.Printf("event already over: server=%s", session.ServerCode)
			if err := a.state.Mark(key, date, "event_over", nil); err != nil {
				rememberFirstErr(&firstErr, err)
			}
		}
	}
	return firstErr
}

func (a *App) checkShop(ctx context.Context) error {
	sessions := a.sessionSnapshot()
	if len(sessions) == 0 {
		return fmt.Errorf("no active sessions")
	}

	date := a.today()
	checks := a.shopChecks()
	oldBeggarCheck := checks[0]
	noticeCheck := checks[1]
	var firstErr error

	if err := a.checkShopPhase(ctx, sessions, date, oldBeggarCheck); err != nil {
		rememberFirstErr(&firstErr, err)
	}
	if a.anyShopPhasePending(sessions, date, noticeCheck) {
		if err := a.waitNoticeShopDelay(ctx); err != nil {
			rememberFirstErr(&firstErr, err)
			return firstErr
		}
		if err := a.checkShopPhase(ctx, sessions, date, noticeCheck); err != nil {
			rememberFirstErr(&firstErr, err)
		}
	}

	return firstErr
}

func (a *App) checkShopPhase(ctx context.Context, sessions []*game.Session, date string, check shopCheck) error {
	sem := make(chan struct{}, a.cfg.App.Concurrency)
	var wg sync.WaitGroup
	var errMu sync.Mutex
	var firstErr error

	for _, session := range sessions {
		if a.shopSourceDone(check, session.ServerCode, date) {
			continue
		}
		wg.Add(1)
		go func(session *game.Session) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				rememberFirstErrLocked(&errMu, &firstErr, ctx.Err())
				return
			}

			if err := a.checkShopSource(ctx, session, date, check); err != nil {
				rememberFirstErrLocked(&errMu, &firstErr, err)
			}
		}(session)
	}

	wg.Wait()
	return firstErr
}

func (a *App) checkShopSource(ctx context.Context, session *game.Session, date string, check shopCheck) error {
	items, err := a.game.SpecialShopItems(ctx, session, check.npcID)
	if err != nil {
		return fmt.Errorf("%s %s get shop data: %w", session.ServerCode, check.label, err)
	}
	return a.notifyShopItems(ctx, session, date, check, items)
}

func (a *App) waitNoticeShopDelay(ctx context.Context) error {
	delay := a.cfg.Notifications.NoticeShopDelay.Duration
	if delay <= 0 {
		return nil
	}
	log.Printf("wait before notice shop phase: %s", delay)
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (a *App) anyShopPhasePending(sessions []*game.Session, date string, check shopCheck) bool {
	for _, session := range sessions {
		if !a.shopSourceDone(check, session.ServerCode, date) {
			return true
		}
	}
	return false
}

func (a *App) notifyShopItems(ctx context.Context, session *game.Session, date string, check shopCheck, items []string) error {
	if len(items) == 0 {
		return nil
	}

	watchHits := watchedItems(items, a.cfg.Notifications.WatchItems)
	if check.watchOnly && len(watchHits) == 0 {
		log.Printf("skip non-watch shop: server=%s source=%s items=%s", session.ServerCode, check.label, game.MessageContent(items))
		return nil
	}

	content := game.MessageContent(items)
	summary := game.MessageSummary(session.ServerCode)
	status := "notified"
	if len(watchHits) > 0 {
		summary = game.WatchMessageSummary(session.ServerCode, watchHits)
		content = fmt.Sprintf("来源: %s\n重点商品: %s\n全部商品: %s", check.label, game.MessageContent(watchHits), content)
		status = "watch_notified"
	} else {
		content = fmt.Sprintf("来源: %s\n全部商品: %s", check.label, content)
	}

	key := sourceStateKey(check.source, session.ServerCode)
	if a.cfg.App.DryRun {
		if len(watchHits) > 0 {
			log.Printf("[dry-run][watch] server=%s source=%s watch=%s items=%s", session.ServerCode, check.label, game.MessageContent(watchHits), game.MessageContent(items))
		} else {
			log.Printf("[dry-run] server=%s source=%s items=%s", session.ServerCode, check.label, game.MessageContent(items))
		}
		// dry_run 不写入 state，确保切换为正式运行后仍能正常推送
		return nil
	}
	anySucceeded, pushErr := a.sendNotification(ctx, session.ServerCode, summary, content)
	if pushErr != nil && !anySucceeded {
		// 所有通道均失败，不标记 state，下次调度仍会重试
		return fmt.Errorf("%s %s push: %w", session.ServerCode, check.label, pushErr)
	}
	if pushErr != nil {
		// 至少一个通道成功，标记 state 防止重复推送，同时记录失败通道
		log.Printf("WARNING: partial push: server=%s source=%s err=%v", session.ServerCode, check.label, pushErr)
	} else {
		log.Printf("notified: server=%s source=%s items=%s", session.ServerCode, check.label, game.MessageContent(items))
	}
	return a.state.Mark(key, date, status, items)
}

func (a *App) shopChecks() []shopCheck {
	return []shopCheck{
		{
			source:    oldBeggarSource,
			label:     "老乞丐",
			npcID:     a.cfg.Notifications.OldBeggarNPCID,
			watchOnly: false,
		},
		{
			source:    noticeShopSource,
			label:     "告示牌奇货",
			npcID:     a.cfg.Notifications.NoticeShopNPCID,
			watchOnly: true,
		},
	}
}

func (a *App) shopSourceDone(check shopCheck, serverCode, date string) bool {
	if check.source == oldBeggarSource {
		return a.oldBeggarDone(serverCode, date) || a.oldBeggarEventOver(serverCode, date)
	}
	return a.state.Done(sourceStateKey(check.source, serverCode), date)
}

func (a *App) oldBeggarDone(serverCode, date string) bool {
	return a.state.Done(sourceStateKey(oldBeggarSource, serverCode), date) || a.state.Done(serverCode, date)
}

func (a *App) oldBeggarEventOver(serverCode, date string) bool {
	if status, ok := a.state.Status(sourceStateKey(oldBeggarSource, serverCode), date); ok && status == "event_over" {
		return true
	}
	if status, ok := a.state.Status(serverCode, date); ok && status == "event_over" {
		return true
	}
	return false
}

func sourceStateKey(source, serverCode string) string {
	return source + ":" + serverCode
}

// sendNotification sends to all configured channels.
// Returns (anySucceeded, err): anySucceeded is true if at least one channel delivered
// successfully. err is non-nil if any channel failed or no channel is configured.
func (a *App) sendNotification(ctx context.Context, serverCode, summary, content string) (anySucceeded bool, err error) {
	channels := 0
	var errs []string
	if a.qqEnabled() {
		channels++
		if sendErr := a.qq.Send(ctx, serverCode, summary, content); sendErr != nil {
			errs = append(errs, "qq: "+sendErr.Error())
		} else {
			anySucceeded = true
		}
	}
	if a.wxpusher.Enabled() {
		channels++
		if sendErr := a.wxpusher.Send(ctx, serverCode, summary, content); sendErr != nil {
			errs = append(errs, "wxpusher: "+sendErr.Error())
		} else {
			anySucceeded = true
		}
	}
	if channels == 0 {
		return false, errors.New("no notification channel configured")
	}
	if len(errs) > 0 {
		return anySucceeded, errors.New(strings.Join(errs, "; "))
	}
	return true, nil
}

func (a *App) qqEnabled() bool {
	return strings.TrimSpace(a.cfg.QQ.APIURL) != ""
}

func (a *App) sessionSnapshot() []*game.Session {
	a.sessionsMu.RLock()
	defer a.sessionsMu.RUnlock()
	out := make([]*game.Session, len(a.sessions))
	copy(out, a.sessions)
	return out
}

func (a *App) today() string {
	return time.Now().In(a.location).Format("2006-01-02")
}

func rememberFirstErr(firstErr *error, err error) {
	if err != nil && *firstErr == nil {
		*firstErr = err
	}
}

func rememberFirstErrLocked(mu *sync.Mutex, firstErr *error, err error) {
	mu.Lock()
	defer mu.Unlock()
	rememberFirstErr(firstErr, err)
}

func watchedItems(items, watchItems []string) []string {
	if len(items) == 0 || len(watchItems) == 0 {
		return nil
	}
	watchSet := make(map[string]struct{}, len(watchItems))
	for _, item := range watchItems {
		item = strings.TrimSpace(item)
		if item != "" {
			watchSet[strings.ToLower(item)] = struct{}{}
		}
	}
	var hits []string
	seen := map[string]struct{}{}
	for _, item := range items {
		item = strings.TrimSpace(item)
		key := strings.ToLower(item)
		if _, ok := watchSet[key]; !ok {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		hits = append(hits, item)
	}
	return hits
}

func floatForLog(value float64) string {
	return fmt.Sprintf("%.0f", value)
}
