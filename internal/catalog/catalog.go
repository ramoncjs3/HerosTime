package catalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

type Catalog struct {
	itemCodeByShopKey map[string]string
	nameByItemCode    map[string]string
}

func Load(itemFile, itemNameFile string) (*Catalog, error) {
	itemRaw, err := os.ReadFile(itemFile)
	if err != nil {
		return nil, fmt.Errorf("read item file: %w", err)
	}
	nameRaw, err := os.ReadFile(itemNameFile)
	if err != nil {
		return nil, fmt.Errorf("read item name file: %w", err)
	}

	var rawItems map[string][]json.RawMessage
	if err := json.Unmarshal(itemRaw, &rawItems); err != nil {
		return nil, fmt.Errorf("parse item file: %w", err)
	}
	var names map[string]string
	if err := json.Unmarshal(nameRaw, &names); err != nil {
		return nil, fmt.Errorf("parse item name file: %w", err)
	}

	c := &Catalog{
		itemCodeByShopKey: make(map[string]string, len(rawItems)),
		nameByItemCode:    names,
	}
	for shopKey, fields := range rawItems {
		if len(fields) <= 7 {
			continue
		}
		code, err := decodeCode(fields[7])
		if err != nil || code == "" {
			continue
		}
		c.itemCodeByShopKey[shopKey] = code
	}
	return c, nil
}

func (c *Catalog) NameForShopKey(shopKey string) (string, bool) {
	code, ok := c.itemCodeByShopKey[shopKey]
	if !ok {
		return "", false
	}
	name, ok := c.nameByItemCode[code]
	return name, ok
}

func decodeCode(raw json.RawMessage) (string, error) {
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString, nil
	}
	var asNumber json.Number
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&asNumber); err != nil {
		return "", err
	}
	return asNumber.String(), nil
}
