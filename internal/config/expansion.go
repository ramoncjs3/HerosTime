package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ExpansionAccountInput struct {
	VariantName     string `json:"variant_name"`
	Server          string `json:"server"`
	Username        string `json:"username"`
	Password        string `json:"password"`
	WxPusherTopicID int64  `json:"wxpusher_topic_id"`
	QQGroupID       string `json:"qq_group_id"`
}

type ExpansionRecord struct {
	VariantName     string    `json:"variant_name"`
	Kind            string    `json:"kind"`
	Server          string    `json:"server"`
	Username        string    `json:"username,omitempty"`
	Password        string    `json:"password,omitempty"`
	WxPusherTopicID int64     `json:"wxpusher_topic_id,omitempty"`
	QQGroupID       string    `json:"qq_group_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type expansionFileData struct {
	Servers []ExpansionRecord `json:"servers"`
}

func (c *Config) VariantsSnapshot() []Variant {
	c.mu.RLock()
	defer c.mu.RUnlock()

	out := make([]Variant, 0, len(c.Variants))
	for _, variant := range c.Variants {
		out = append(out, cloneVariant(variant))
	}
	return out
}

func (c *Config) ExpansionSnapshot() []ExpansionRecord {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return cloneExpansionRecords(c.expansions)
}

func (c *Config) AddExpansionAccount(input ExpansionAccountInput) (ExpansionRecord, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	record, variantIndex, err := c.normalizeExpansionInputLocked(input)
	if err != nil {
		return ExpansionRecord{}, err
	}
	if hasServer(c.Variants[variantIndex], record.Server) {
		return ExpansionRecord{}, fmt.Errorf("server %q already exists in variant %q", record.Server, record.VariantName)
	}

	oldAccounts := append([]Account(nil), c.Variants[variantIndex].Accounts...)
	oldExpansions := cloneExpansionRecords(c.expansions)
	oldTopicMap := cloneInt64Map(c.WxPusher.TopicMap)
	oldGroupMap := cloneStringMap(c.QQ.GroupMap)
	c.Variants[variantIndex].Accounts = append(c.Variants[variantIndex].Accounts, Account{
		Server:   record.Server,
		Username: record.Username,
		Password: record.Password,
	})
	c.applyExpansionDestinationsLocked(record)
	c.expansions = append(c.expansions, record)
	if err := c.saveExpansionFileLocked(); err != nil {
		c.Variants[variantIndex].Accounts = oldAccounts
		c.expansions = oldExpansions
		c.WxPusher.TopicMap = oldTopicMap
		c.QQ.GroupMap = oldGroupMap
		return ExpansionRecord{}, err
	}
	return record, nil
}

func (c *Config) RemoveExpansionAccount(input ExpansionAccountInput) (ExpansionRecord, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	variantName := strings.TrimSpace(input.VariantName)
	server := normalizeServer(input.Server)
	if variantName == "" || server == "" {
		return ExpansionRecord{}, errors.New("variant_name and server are required")
	}

	expansionIndex := -1
	var record ExpansionRecord
	for i, item := range c.expansions {
		if item.VariantName == variantName && item.Server == server {
			expansionIndex = i
			record = item
			break
		}
	}
	if expansionIndex < 0 {
		return ExpansionRecord{}, fmt.Errorf("server %q is not a dynamic expansion in variant %q", server, variantName)
	}

	variantIndex := c.variantIndexLocked(variantName)
	if variantIndex < 0 {
		return ExpansionRecord{}, fmt.Errorf("variant %q not found", variantName)
	}

	oldAccounts := append([]Account(nil), c.Variants[variantIndex].Accounts...)
	oldExpansions := cloneExpansionRecords(c.expansions)
	oldTopicMap := cloneInt64Map(c.WxPusher.TopicMap)
	oldGroupMap := cloneStringMap(c.QQ.GroupMap)
	c.expansions = append(c.expansions[:expansionIndex], c.expansions[expansionIndex+1:]...)
	c.Variants[variantIndex].Accounts = removeServerAccount(c.Variants[variantIndex].Accounts, server)
	c.removeExpansionDestinationsLocked(record)
	if err := c.saveExpansionFileLocked(); err != nil {
		c.Variants[variantIndex].Accounts = oldAccounts
		c.expansions = oldExpansions
		c.WxPusher.TopicMap = oldTopicMap
		c.QQ.GroupMap = oldGroupMap
		return ExpansionRecord{}, err
	}
	return record, nil
}

func (c *Config) loadExpansionFile() error {
	data, err := readExpansionFile(c.Expansion.File)
	if err != nil {
		return err
	}
	if len(data.Servers) == 0 {
		return nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.expansions = nil
	for _, item := range data.Servers {
		record, variantIndex, err := c.normalizeExpansionRecordLocked(item)
		if err != nil {
			return err
		}
		c.applyExpansionDestinationsLocked(record)
		if hasServer(c.Variants[variantIndex], record.Server) {
			continue
		}
		c.Variants[variantIndex].Accounts = append(c.Variants[variantIndex].Accounts, Account{
			Server:   record.Server,
			Username: record.Username,
			Password: record.Password,
		})
		c.expansions = append(c.expansions, record)
	}
	return nil
}

func (c *Config) normalizeExpansionInputLocked(input ExpansionAccountInput) (ExpansionRecord, int, error) {
	record := ExpansionRecord{
		VariantName:     strings.TrimSpace(input.VariantName),
		Server:          normalizeServer(input.Server),
		Username:        strings.TrimSpace(input.Username),
		Password:        strings.TrimSpace(input.Password),
		WxPusherTopicID: input.WxPusherTopicID,
		QQGroupID:       strings.TrimSpace(input.QQGroupID),
	}
	now := time.Now()
	record.CreatedAt = now
	record.UpdatedAt = now
	return c.normalizeExpansionRecordLocked(record)
}

func (c *Config) normalizeExpansionRecordLocked(record ExpansionRecord) (ExpansionRecord, int, error) {
	record.VariantName = strings.TrimSpace(record.VariantName)
	record.Server = normalizeServer(record.Server)
	record.Username = strings.TrimSpace(record.Username)
	record.Password = strings.TrimSpace(record.Password)
	record.QQGroupID = strings.TrimSpace(record.QQGroupID)
	if record.VariantName == "" {
		return ExpansionRecord{}, -1, errors.New("variant_name is required")
	}
	if record.Server == "" {
		return ExpansionRecord{}, -1, errors.New("server is required")
	}
	if !isSafeServerCode(record.Server) {
		return ExpansionRecord{}, -1, fmt.Errorf("server %q contains unsupported characters", record.Server)
	}
	if record.WxPusherTopicID < 0 {
		return ExpansionRecord{}, -1, errors.New("wxpusher_topic_id must be greater than zero")
	}

	variantIndex := c.variantIndexLocked(record.VariantName)
	if variantIndex < 0 {
		return ExpansionRecord{}, -1, fmt.Errorf("variant %q not found", record.VariantName)
	}
	variant := c.Variants[variantIndex]
	record.Kind = variant.Kind
	if record.Username == "" && strings.TrimSpace(variant.Auth.Username) == "" {
		return ExpansionRecord{}, -1, fmt.Errorf("variant %q requires username", record.VariantName)
	}
	if record.Password == "" && strings.TrimSpace(variant.Auth.Password) == "" {
		return ExpansionRecord{}, -1, fmt.Errorf("variant %q requires password", record.VariantName)
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now()
	}
	if record.UpdatedAt.IsZero() {
		record.UpdatedAt = record.CreatedAt
	}
	return record, variantIndex, nil
}

func (c *Config) applyExpansionDestinationsLocked(record ExpansionRecord) {
	if record.WxPusherTopicID > 0 {
		if c.WxPusher.TopicMap == nil {
			c.WxPusher.TopicMap = map[string]int64{}
		}
		c.WxPusher.TopicMap[record.Server] = record.WxPusherTopicID
	}
	if strings.TrimSpace(record.QQGroupID) != "" {
		if c.QQ.GroupMap == nil {
			c.QQ.GroupMap = map[string]string{}
		}
		c.QQ.GroupMap[record.Server] = strings.TrimSpace(record.QQGroupID)
	}
}

func (c *Config) removeExpansionDestinationsLocked(record ExpansionRecord) {
	if record.WxPusherTopicID > 0 && c.WxPusher.TopicMap != nil && c.WxPusher.TopicMap[record.Server] == record.WxPusherTopicID {
		delete(c.WxPusher.TopicMap, record.Server)
	}
	if strings.TrimSpace(record.QQGroupID) != "" && c.QQ.GroupMap != nil && c.QQ.GroupMap[record.Server] == strings.TrimSpace(record.QQGroupID) {
		delete(c.QQ.GroupMap, record.Server)
	}
}

func (c *Config) variantIndexLocked(name string) int {
	for i := range c.Variants {
		if c.Variants[i].Name == name {
			return i
		}
	}
	return -1
}

func (c *Config) saveExpansionFileLocked() error {
	if strings.TrimSpace(c.Expansion.File) == "" {
		return errors.New("expansion.file is empty")
	}
	if err := os.MkdirAll(filepath.Dir(c.Expansion.File), 0755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(expansionFileData{Servers: c.expansions}, "", "  ")
	if err != nil {
		return err
	}
	tmp := c.Expansion.File + ".tmp"
	if err := os.WriteFile(tmp, raw, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, c.Expansion.File)
}

func readExpansionFile(path string) (expansionFileData, error) {
	data := expansionFileData{Servers: []ExpansionRecord{}}
	if strings.TrimSpace(path) == "" {
		return data, nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return data, nil
		}
		return data, err
	}
	if len(raw) == 0 {
		return data, nil
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return data, err
	}
	if data.Servers == nil {
		data.Servers = []ExpansionRecord{}
	}
	return data, nil
}

func cloneVariant(variant Variant) Variant {
	variant.ServerRules = append([]ServerRule(nil), variant.ServerRules...)
	variant.Accounts = append([]Account(nil), variant.Accounts...)
	return variant
}

func cloneExpansionRecords(records []ExpansionRecord) []ExpansionRecord {
	out := make([]ExpansionRecord, len(records))
	copy(out, records)
	return out
}

func cloneInt64Map(in map[string]int64) map[string]int64 {
	if in == nil {
		return nil
	}
	out := make(map[string]int64, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func hasServer(variant Variant, server string) bool {
	for _, account := range variant.Accounts {
		if normalizeServer(account.Server) == server {
			return true
		}
	}
	return false
}

func removeServerAccount(accounts []Account, server string) []Account {
	out := accounts[:0]
	removed := false
	for _, account := range accounts {
		if !removed && normalizeServer(account.Server) == server {
			removed = true
			continue
		}
		out = append(out, account)
	}
	return out
}

func normalizeServer(value string) string {
	return strings.TrimSpace(value)
}

func isSafeServerCode(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' {
			continue
		}
		return false
	}
	return true
}
