package cloudflare

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"go.uber.org/zap"
)

type Cloudflare interface {
	// CreateRecord Creates a new DNS record
	CreateRecord(subdomain, ip string) error

	// UpdateRecord Updates an existing DNS record
	UpdateRecord(subdomain, ip string) error

	// DeleteRecord Deletes a DNS record
	DeleteRecord(subdomain string) error
}

type cloudflare struct {
	config *Config
	domain string
	logger *zap.Logger
}

// DNSRecord is a partial request and response model for working with Cloudflare API
type DNSRecord struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
	Type    string `json:"type"`
	TTL     int    `json:"ttl"`
}

func New(cfg *Config, domain string, lg *zap.Logger) Cloudflare {
	return &cloudflare{config: cfg, domain: domain, logger: lg}
}

func (c *cloudflare) listRecords() ([]DNSRecord, error) {
	url := fmt.Sprintf("%s/zones/%s/dns_records", c.config.CloudflareURL, c.config.ZoneID)

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("Authorization", "Bearer "+c.config.Token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.logger.Error("Error listing DNS records", zap.Error(err))
		return []DNSRecord{}, err
	}
	defer resp.Body.Close()

	response := struct {
		Records []DNSRecord `json:"result"`
	}{}

	// var records []DNSRecord
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		c.logger.Error("Error decoding JSON", zap.Error(err))
		return []DNSRecord{}, err
	}

	return response.Records, nil
}

func (c *cloudflare) getRecordID(subdomain string) (string, error) {
	records, err := c.listRecords()
	if err != nil {
		c.logger.Error("Error list records", zap.Error(err))
		return "", err
	}

	recordID := ""
	for index := 0; index < len(records); index++ {
		if records[index].Name == subdomain {
			recordID = records[index].ID
		}
	}

	return recordID, nil
}

var (
	RecordAlreadyExists = errors.New("Record already exists")
	RecordNotFound      = errors.New("Record not found")
)

func (c *cloudflare) sendRequest(url string, payload []byte, method string) error {
	req, _ := http.NewRequest(method, url, bytes.NewBuffer(payload))
	req.Header.Set("Authorization", "Bearer "+c.config.Token)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.logger.Error("Error performing request", zap.Error(err))
		return err
	}
	defer resp.Body.Close()

	return nil
}

func (c *cloudflare) CreateRecord(subdomain, ip string) error {
	recordID, err := c.getRecordID(subdomain)
	if err != nil {
		c.logger.Error("Error get record id", zap.Error(err))
		return err
	} else if recordID != "" {
		return RecordAlreadyExists
	}

	payload, _ := json.Marshal(DNSRecord{
		Type:    "A",
		Name:    subdomain,
		Content: ip,
	})

	url := fmt.Sprintf("%s/zones/%s/dns_records", c.config.CloudflareURL, c.config.ZoneID)
	return c.sendRequest(url, payload, "POST")

}

func (c *cloudflare) UpdateRecord(subdomain, ip string) error {
	recordID, err := c.getRecordID(subdomain)
	if err != nil {
		c.logger.Error("Error get record id", zap.Error(err))
		return err
	} else if recordID == "" {
		return RecordNotFound
	}

	payload, _ := json.Marshal(DNSRecord{
		Content: ip,
	})

	url := fmt.Sprintf("%s/zones/%s/dns_records/%s", c.config.CloudflareURL, c.config.ZoneID, recordID)
	return c.sendRequest(url, payload, "PUT")
}

func (c *cloudflare) DeleteRecord(subdomain string) error {
	recordID, err := c.getRecordID(subdomain)
	if err != nil {
		c.logger.Error("Error get record id", zap.Error(err))
		return err
	} else if recordID == "" {
		return RecordNotFound
	}

	url := fmt.Sprintf("%s/zones/%s/dns_records/%s", c.config.CloudflareURL, c.config.ZoneID, recordID)
	return c.sendRequest(url, []byte{}, "DELETE")
}
