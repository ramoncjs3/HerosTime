package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode && node.Value == "" {
		return nil
	}
	var raw string
	if err := node.Decode(&raw); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return fmt.Errorf("parse duration %q: %w", raw, err)
	}
	d.Duration = parsed
	return nil
}

type Config struct {
	mu            sync.RWMutex       `yaml:"-"`
	Path          string             `yaml:"-"`
	App           AppConfig          `yaml:"app"`
	HTTP          HTTPConfig         `yaml:"http"`
	Admin         AdminConfig        `yaml:"admin"`
	Storage       StorageConfig      `yaml:"storage"`
	Expansion     ExpansionConfig    `yaml:"expansion"`
	Schedules     ScheduleConfig     `yaml:"schedules"`
	QQ            QQConfig           `yaml:"qq"`
	WxPusher      WxPusherConfig     `yaml:"wxpusher"`
	Notifications NotificationConfig `yaml:"notifications"`
	Catalog       CatalogConfig      `yaml:"catalog"`
	Captcha       CaptchaConfig      `yaml:"captcha"`
	Variants      []Variant          `yaml:"variants"`
	expansions    []ExpansionRecord
}

type AppConfig struct {
	Name        string `yaml:"name"`
	Timezone    string `yaml:"timezone"`
	DryRun      bool   `yaml:"dry_run"`
	StateFile   string `yaml:"state_file"`
	Concurrency int    `yaml:"concurrency"`
}

type HTTPConfig struct {
	Timeout     Duration `yaml:"timeout"`
	Retries     int      `yaml:"retries"`
	Backoff     Duration `yaml:"backoff"`
	InsecureTLS bool     `yaml:"insecure_tls"`
}

type AdminConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Addr     string `yaml:"addr"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type StorageConfig struct {
	Type        string      `yaml:"type"`
	MySQL       MySQLConfig `yaml:"mysql"`
	AutoMigrate *bool       `yaml:"auto_migrate"`
}

type MySQLConfig struct {
	DSN        string `yaml:"dsn"`
	StateTable string `yaml:"state_table"`
	EventTable string `yaml:"event_table"`
}

type ExpansionConfig struct {
	File string `yaml:"file"`
}

type ScheduleConfig struct {
	BootstrapOnStart bool   `yaml:"bootstrap_on_start"`
	RefreshSessions  string `yaml:"refresh_sessions"`
	CheckShop        string `yaml:"check_shop"`
}

type QQConfig struct {
	APIURL       string            `yaml:"api_url"`
	AccessToken  string            `yaml:"access_token"`
	TargetType   string            `yaml:"target_type"`
	DefaultID    string            `yaml:"default_id"`
	GroupMap     map[string]string `yaml:"group_map"`
	GroupMapFile string            `yaml:"group_map_file"`
	MinInterval  Duration          `yaml:"min_interval"`
}

type WxPusherConfig struct {
	Enabled      bool             `yaml:"enabled"`
	Endpoint     string           `yaml:"endpoint"`
	AppToken     string           `yaml:"app_token"`
	ContentType  int              `yaml:"content_type"`
	URL          string           `yaml:"url"`
	TopicMap     map[string]int64 `yaml:"topic_map"`
	TopicMapFile string           `yaml:"topic_map_file"`
}

type NotificationConfig struct {
	WatchItems      []string `yaml:"watch_items"`
	OldBeggarNPCID  int      `yaml:"oldbeggar_npc_id"`
	NoticeShopNPCID int      `yaml:"notice_shop_npc_id"`
	NoticeShopDelay Duration `yaml:"notice_shop_delay"`
}

type CatalogConfig struct {
	ItemFile     string `yaml:"item_file"`
	ItemNameFile string `yaml:"item_name_file"`
}

// CaptchaConfig 控制登录图形验证码的自动识别。
type CaptchaConfig struct {
	// Enabled 显式设置为 true 时才启用验证码识别；默认关闭，
	// 避免旧配置升级后意外改变生产行为。
	Enabled *bool `yaml:"enabled"`
	// Engine 支持 onnx（本地 ddddocr 模型）与 http（远程识别服务）。
	Engine string `yaml:"engine"`
	// ModelFile 为 ddddocr common.onnx 模型路径，相对配置目录解析。
	ModelFile string `yaml:"model_file"`
	// Endpoint 为 http 引擎的识别地址。
	Endpoint string `yaml:"endpoint"`
	// MaxAttempts 为单次登录允许尝试识别验证码的最大次数。
	MaxAttempts int `yaml:"max_attempts"`
	// RateLimitCooldown 为触发限流后账号的冷却时间，冷却内不再尝试登录。
	RateLimitCooldown Duration `yaml:"rate_limit_cooldown"`
}

// IsEnabled 返回验证码识别是否开启（默认关闭，需显式 enabled: true）。
func (c CaptchaConfig) IsEnabled() bool {
	return c.Enabled != nil && *c.Enabled
}

type Variant struct {
	Name              string       `yaml:"name"`
	Kind              string       `yaml:"kind"`
	Enabled           *bool        `yaml:"enabled"`
	ServerIndexURL    string       `yaml:"server_index_url"`
	ServerListPayload string       `yaml:"server_list_payload"`
	Auth              AuthConfig   `yaml:"auth"`
	ServerRules       []ServerRule `yaml:"server_rules"`
	Accounts          []Account    `yaml:"accounts"`
}

type AuthConfig struct {
	Username    string `yaml:"username"`
	Password    string `yaml:"password"`
	OpenSSLKey  string `yaml:"openssl_key"`
	DESKey      string `yaml:"des_key"`
	ProductCode string `yaml:"product_code"`
	ChannelCode string `yaml:"channel_code"`
	GameID      string `yaml:"game_id"`
	GameKey     string `yaml:"game_key"`
	PackageName string `yaml:"package_name"`
}

type ServerRule struct {
	MatchPrefix string `yaml:"match_prefix"`
	CodePrefix  string `yaml:"code_prefix"`
}

type Account struct {
	Server   string `yaml:"server"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

func Load(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	expanded := os.ExpandEnv(string(raw))
	var cfg Config
	if err := yaml.Unmarshal([]byte(expanded), &cfg); err != nil {
		return nil, err
	}
	cfg.Path = path
	cfg.applyDefaults()
	cfg.resolveRelativePaths()
	cfg.applyEnvOverrides()
	if err := cfg.loadExpansionFile(); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (c *Config) applyDefaults() {
	if c.App.Name == "" {
		c.App.Name = "oldbeggar"
	}
	if c.App.Timezone == "" {
		c.App.Timezone = "Asia/Shanghai"
	}
	if c.App.StateFile == "" {
		c.App.StateFile = "state/oldbeggar-state.json"
	}
	if c.App.Concurrency <= 0 {
		c.App.Concurrency = 4
	}
	if c.HTTP.Timeout.Duration == 0 {
		c.HTTP.Timeout.Duration = 30 * time.Second
	}
	if c.HTTP.Backoff.Duration == 0 {
		c.HTTP.Backoff.Duration = 2 * time.Second
	}
	if c.HTTP.Retries < 0 {
		c.HTTP.Retries = 0
	}
	if c.Admin.Addr == "" {
		c.Admin.Addr = ":8088"
	}
	if c.Admin.Username == "" {
		c.Admin.Username = "admin"
	}
	c.Storage.Type = strings.ToLower(strings.TrimSpace(c.Storage.Type))
	if c.Storage.Type == "" {
		c.Storage.Type = "json"
	}
	if c.Storage.MySQL.StateTable == "" {
		c.Storage.MySQL.StateTable = "oldbeggar_push_state"
	}
	if c.Storage.MySQL.EventTable == "" {
		c.Storage.MySQL.EventTable = "oldbeggar_push_events"
	}
	if c.Schedules.RefreshSessions == "" {
		c.Schedules.RefreshSessions = "0 30 7 * * *"
	}
	if c.Schedules.CheckShop == "" {
		c.Schedules.CheckShop = "30 0,30 8-23 * * *"
	}
	if c.QQ.TargetType == "" {
		c.QQ.TargetType = "group"
	}
	if c.QQ.GroupMap == nil {
		c.QQ.GroupMap = map[string]string{}
	}
	if c.WxPusher.Endpoint == "" {
		c.WxPusher.Endpoint = "https://wxpusher.zjiecode.com/api/send/message"
	}
	if c.WxPusher.ContentType == 0 {
		c.WxPusher.ContentType = 1
	}
	if c.WxPusher.TopicMap == nil {
		c.WxPusher.TopicMap = map[string]int64{}
	}
	if c.Notifications.OldBeggarNPCID == 0 {
		c.Notifications.OldBeggarNPCID = 10163
	}
	if c.Notifications.NoticeShopNPCID == 0 {
		c.Notifications.NoticeShopNPCID = 1009
	}
	if c.Notifications.NoticeShopDelay.Duration == 0 {
		c.Notifications.NoticeShopDelay.Duration = 20 * time.Second
	}
	if c.Captcha.Engine == "" {
		c.Captcha.Engine = "onnx"
	}
	if c.Captcha.ModelFile == "" {
		c.Captcha.ModelFile = "ocr/common.onnx"
	}
	if c.Captcha.MaxAttempts <= 0 {
		c.Captcha.MaxAttempts = 3
	}
	if c.Captcha.RateLimitCooldown.Duration == 0 {
		c.Captcha.RateLimitCooldown.Duration = 5 * time.Minute
	}
	for i := range c.Variants {
		v := &c.Variants[i]
		v.Kind = strings.ToLower(strings.TrimSpace(v.Kind))
		if v.Name == "" {
			v.Name = v.Kind
		}
		if v.ServerIndexURL == "" {
			v.ServerIndexURL = defaultServerIndexURL(v.Kind)
		}
		if v.ServerListPayload == "" {
			v.ServerListPayload = defaultServerListPayload(v.Kind)
		}
		if v.Auth.OpenSSLKey == "" {
			v.Auth.OpenSSLKey = "lzYW5qaXVqa"
		}
		if v.Auth.DESKey == "" {
			v.Auth.DESKey = "57493415"
		}
		if v.Auth.ProductCode == "" {
			v.Auth.ProductCode = "83313602112675691534121381984132"
		}
		if v.Auth.ChannelCode == "" {
			v.Auth.ChannelCode = "27"
		}
		if v.Auth.GameID == "" {
			v.Auth.GameID = "100053785"
		}
		if v.Auth.GameKey == "" {
			v.Auth.GameKey = "116798"
		}
		if v.Auth.PackageName == "" {
			v.Auth.PackageName = "com.maple.madherogo.m4399"
		}
	}
}

func defaultServerIndexURL(kind string) string {
	switch kind {
	case "h5", "mini":
		return "https://bz.maple-game.com/h5.json"
	default:
		return "http://bz.maple-game.com/bz.json"
	}
}

func defaultServerListPayload(kind string) string {
	switch kind {
	case "h5":
		return "a515314766c66a0146918898435cb2c08938a1cf3899c350cd905566983202334bea7b42c11ddb6b32cf21a1e61ec92ce74011509d3e126e12091d5f8590ce8c9d9e475c6ad6a014e3e04da25a2e82049f8c6378ecdac4950025843ee7a5dfd0"
	case "mini":
		return "a515314766c66a0146918898435cb2c08938a1cf3899c350cd905566983202334bea7b42c11ddb6b32cf21a1e61ec92ce74011509d3e126e12091d5f8590ce8cb75d44f0f78bcbc433d53fdb588112e91eb5a91dec5a1c3ca504d729520b2ad6"
	case "baozou":
		return "a515314766c66a0146918898435cb2c08938a1cf3899c350cd905566983202334bea7b42c11ddb6b32cf21a1e61ec92ce74011509d3e126e12091d5f8590ce8ca721bb5d3b728cab1275abe305e5abbe0fd05533e1ca6610ad638f4ae08c5551"
	case "apple":
		return "a515314766c66a0146918898435cb2c08938a1cf3899c350cd905566983202334bea7b42c11ddb6b32cf21a1e61ec92ce74011509d3e126e12091d5f8590ce8cf18e9dd0193e1110c359241f02af452deb07ad8b6d79ee11d9c327d08863e025"
	default:
		return "a515314766c66a0146918898435cb2c08938a1cf3899c350cd905566983202334bea7b42c11ddb6b32cf21a1e61ec92ce74011509d3e126e12091d5f8590ce8c98987160177e7d91ea8b118e2429484a6b6ca73e366b3bb7acbfb8cc2db804ca"
	}
}

func (c *Config) resolveRelativePaths() {
	base := filepath.Dir(c.Path)
	c.App.StateFile = resolvePath(base, c.App.StateFile)
	if c.Expansion.File == "" {
		c.Expansion.File = filepath.Join(filepath.Dir(c.App.StateFile), "oldbeggar-expansion.json")
	} else {
		c.Expansion.File = resolvePath(base, c.Expansion.File)
	}
	c.Catalog.ItemFile = resolvePath(base, c.Catalog.ItemFile)
	c.Catalog.ItemNameFile = resolvePath(base, c.Catalog.ItemNameFile)
	c.Captcha.ModelFile = resolvePath(base, c.Captcha.ModelFile)
	c.QQ.GroupMapFile = resolvePath(base, c.QQ.GroupMapFile)
	c.WxPusher.TopicMapFile = resolvePath(base, c.WxPusher.TopicMapFile)
}

func resolvePath(base, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Clean(filepath.Join(base, path))
}

func (c *Config) applyEnvOverrides() {
	if os.Getenv("DRY_RUN") == "1" {
		c.App.DryRun = true
	}
	if value := strings.TrimSpace(os.Getenv("OLDBEGGAR_ADMIN_ENABLED")); value != "" {
		c.Admin.Enabled = isTruthy(value)
	}
	if value := strings.TrimSpace(os.Getenv("OLDBEGGAR_ADMIN_ADDR")); value != "" {
		c.Admin.Addr = value
	}
	if value := strings.TrimSpace(os.Getenv("OLDBEGGAR_ADMIN_USERNAME")); value != "" {
		c.Admin.Username = value
	}
	if value := strings.TrimSpace(os.Getenv("OLDBEGGAR_ADMIN_PASSWORD")); value != "" {
		c.Admin.Password = value
	}
	if value := strings.TrimSpace(os.Getenv("OLDBEGGAR_STORAGE_TYPE")); value != "" {
		c.Storage.Type = strings.ToLower(value)
	}
	if value := strings.TrimSpace(os.Getenv("OLDBEGGAR_MYSQL_DSN")); value != "" {
		c.Storage.MySQL.DSN = value
	}
	if value := strings.TrimSpace(os.Getenv("OLDBEGGAR_MYSQL_STATE_TABLE")); value != "" {
		c.Storage.MySQL.StateTable = value
	}
	if value := strings.TrimSpace(os.Getenv("OLDBEGGAR_MYSQL_EVENT_TABLE")); value != "" {
		c.Storage.MySQL.EventTable = value
	}
	if value := strings.TrimSpace(os.Getenv("OLDBEGGAR_STORAGE_AUTO_MIGRATE")); value != "" {
		enabled := isTruthy(value)
		c.Storage.AutoMigrate = &enabled
	}
	if value := strings.TrimSpace(os.Getenv("OLDBEGGAR_EXPANSION_FILE")); value != "" {
		c.Expansion.File = resolvePath(filepath.Dir(c.Path), value)
	}
	if c.QQ.APIURL == "" {
		c.QQ.APIURL = os.Getenv("QQ_BOT_API_URL")
	}
	if c.QQ.AccessToken == "" {
		c.QQ.AccessToken = os.Getenv("QQ_BOT_ACCESS_TOKEN")
	}
	if c.QQ.DefaultID == "" {
		c.QQ.DefaultID = firstNonEmpty(os.Getenv("QQ_TARGET_ID"), os.Getenv("QQ_GROUP_ID"))
	}
	if c.QQ.GroupMapFile == "" {
		c.QQ.GroupMapFile = os.Getenv("QQ_GROUP_MAP_FILE")
	}
	if value := strings.TrimSpace(os.Getenv("WXPUSHER_ENABLED")); value != "" {
		c.WxPusher.Enabled = isTruthy(value)
	}
	if value := strings.TrimSpace(os.Getenv("WXPUSHER_ENDPOINT")); value != "" {
		c.WxPusher.Endpoint = value
	}
	if c.WxPusher.AppToken == "" {
		c.WxPusher.AppToken = os.Getenv("WXPUSHER_APP_TOKEN")
	}
	if c.WxPusher.TopicMapFile == "" {
		c.WxPusher.TopicMapFile = os.Getenv("WXPUSHER_TOPIC_MAP_FILE")
	}
}

func (c *Config) Validate() error {
	if c.Catalog.ItemFile == "" {
		return fmt.Errorf("catalog.item_file is required")
	}
	if c.Catalog.ItemNameFile == "" {
		return fmt.Errorf("catalog.item_name_file is required")
	}
	if c.Admin.Enabled {
		if strings.TrimSpace(c.Admin.Addr) == "" {
			return fmt.Errorf("admin.addr is required when admin is enabled")
		}
		if strings.TrimSpace(c.Admin.Username) == "" {
			return fmt.Errorf("admin.username or OLDBEGGAR_ADMIN_USERNAME is required when admin is enabled")
		}
		if strings.TrimSpace(c.Admin.Password) == "" {
			return fmt.Errorf("admin.password or OLDBEGGAR_ADMIN_PASSWORD is required when admin is enabled")
		}
	}
	switch c.Storage.Type {
	case "json", "file":
	case "mysql":
		if strings.TrimSpace(c.Storage.MySQL.DSN) == "" {
			return fmt.Errorf("storage.mysql.dsn or OLDBEGGAR_MYSQL_DSN is required when storage.type=mysql")
		}
		if !isSafeIdentifier(c.Storage.MySQL.StateTable) {
			return fmt.Errorf("storage.mysql.state_table contains unsupported characters")
		}
		if !isSafeIdentifier(c.Storage.MySQL.EventTable) {
			return fmt.Errorf("storage.mysql.event_table contains unsupported characters")
		}
	default:
		return fmt.Errorf("unsupported storage.type %q", c.Storage.Type)
	}
	if len(c.Variants) == 0 {
		return fmt.Errorf("at least one variant is required")
	}
	for _, v := range c.EnabledVariants() {
		if v.Kind == "" {
			return fmt.Errorf("variant %q kind is required", v.Name)
		}
		switch v.Kind {
		case "official", "h5", "mini", "baozou", "apple":
		default:
			return fmt.Errorf("variant %q has unsupported kind %q", v.Name, v.Kind)
		}
		if len(v.ServerRules) == 0 {
			return fmt.Errorf("variant %q requires at least one server rule", v.Name)
		}
		if len(v.Accounts) == 0 {
			return fmt.Errorf("variant %q requires at least one account/server", v.Name)
		}
	}
	return nil
}

func (c *Config) EnabledVariants() []Variant {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.enabledVariantsLocked()
}

func (c *Config) enabledVariantsLocked() []Variant {
	var out []Variant
	for _, v := range c.Variants {
		if v.Enabled != nil && !*v.Enabled {
			continue
		}
		out = append(out, cloneVariant(v))
	}
	return out
}

func (v Variant) AccountFor(raw Account) Account {
	if raw.Username == "" {
		raw.Username = v.Auth.Username
	}
	if raw.Password == "" {
		raw.Password = v.Auth.Password
	}
	return raw
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func isTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func isSafeIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}
