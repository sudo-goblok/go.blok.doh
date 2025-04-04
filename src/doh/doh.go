package doh

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Struktur resolver dengan ID, URL, dan Weight
type Resolver struct {
	ID     string
	URL    string
	Weight int
}

type DOHResponse struct {
	Status   int  `json:"Status"`
	TC       bool `json:"TC"`
	RD       bool `json:"RD"`
	RA       bool `json:"RA"`
	AD       bool `json:"AD"`
	CD       bool `json:"CD"`
	Question []struct {
		Name string `json:"name"`
		Type int    `json:"type"`
	} `json:"Question"`
	Answer []struct {
		Name string `json:"name"`
		Type int    `json:"type"`
		TTL  int    `json:"TTL"`
		Data string `json:"data"`
	} `json:"Answer"`
	Authority []struct {
		Name string `json:"name"`
		Type int    `json:"type"`
		TTL  int    `json:"TTL"`
		Data string `json:"data"`
	} `json:"Authority"`
	Comment json.RawMessage `json:"Comment,omitempty"` // Pakai RawMessage untuk fleksibilitas
}

type DOHClient struct {
	Resolvers []Resolver
	Client    *http.Client

	mu          sync.Mutex
	totalWeight int
}
type ResolverInfo struct {
	Resolver    string `json:"resolver"`
	ResolverURL string `json:"resolver_url"`
}

func (d *DOHResponse) GetComment() []string {
	var comments []string

	// Cek apakah Comment kosong
	if len(d.Comment) == 0 {
		return comments
	}

	// Coba decode sebagai array string
	if err := json.Unmarshal(d.Comment, &comments); err == nil {
		return comments
	}

	// Jika gagal, coba decode sebagai string tunggal
	var singleComment string
	if err := json.Unmarshal(d.Comment, &singleComment); err == nil {
		return []string{singleComment}
	}

	// Jika semua gagal, log error (opsional)
	log.Printf("[ERROR] Failed to parse Comment: %s", string(d.Comment))
	return comments
}

// **Fungsi untuk mendapatkan subnet ECS dengan penanganan IPv6**
func getECSSubnet(ip net.IP, prefixLen int) string {
	if ip.To4() != nil {
		ip = ip.Mask(net.CIDRMask(prefixLen, 32))
	} else {
		ip = ip.Mask(net.CIDRMask(prefixLen, 128))
	}
	return fmt.Sprintf("%s/%d", ip.String(), prefixLen)
}

// **Fungsi untuk mengecek apakah IP adalah private**
func isPrivateIP(ip net.IP) bool {
	privateBlocks := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"0.0.0.0/8",
		"224.0.0.0/4",
		"240.0.0.0/4",
		"::1/128",
		"fc00::/7",
	}

	for _, cidr := range privateBlocks {
		_, block, _ := net.ParseCIDR(cidr)
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

// **Constructor untuk DOHClient dengan perhitungan total weight**
func NewDOHClient(resolvers []Resolver) *DOHClient {
	totalWeight := 0
	for _, r := range resolvers {
		totalWeight += r.Weight
	}

	transport := &http.Transport{
		MaxIdleConns:       100,
		IdleConnTimeout:    90 * time.Second,
		DisableCompression: true,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS13,
		},
	}

	client := &http.Client{
		Timeout:   5 * time.Second,
		Transport: transport,
	}

	return &DOHClient{
		Resolvers:   resolvers,
		Client:      client,
		totalWeight: totalWeight,
	}
}

// **Fungsi memilih resolver berbasis weight (bobot)**
func (d *DOHClient) getNextResolver() Resolver {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.totalWeight == 0 || len(d.Resolvers) == 0 {
		return d.Resolvers[rand.Intn(len(d.Resolvers))] // Fallback ke random
	}

	randVal := rand.Intn(d.totalWeight)
	for _, r := range d.Resolvers {
		if randVal < r.Weight {
			return r
		}
		randVal -= r.Weight
	}
	return d.Resolvers[0] // Fallback jika terjadi error
}

// **Query dengan resolver yang dipilih secara berbasis weight**
func (d *DOHClient) Query(domain string, qtype uint16, clientIP string) (*DOHResponse, ResolverInfo, error) {
	if len(d.Resolvers) == 0 {
		return nil, ResolverInfo{}, fmt.Errorf("no resolvers available")
	}

	domain = strings.TrimSuffix(domain, ".")
	qtypeStr := fmt.Sprintf("%d", qtype)

	// Coba sebanyak jumlah resolver (weighted round-robin fallback)
	for i := 0; i < len(d.Resolvers); i++ {
		resolver := d.getNextResolver()

		var url string
		if isPrivateIP(net.ParseIP(clientIP)) {
			url = fmt.Sprintf("%s?name=%s&type=%s", resolver.URL, domain, qtypeStr)
		} else {
			clientSubnet := getECSSubnet(net.ParseIP(clientIP), 24)
			url = fmt.Sprintf("%s?name=%s&type=%s&edns_client_subnet=%s", resolver.URL, domain, qtypeStr, clientSubnet)
		}

		// log.Printf("[DEBUG] Querying resolver [%s] %s: %s", resolver.ID, resolver.URL, url)
		log.Printf("[DEBUG] Querying resolver [%s]: %s", resolver.ID, url)
		resolverInfo := ResolverInfo{
			Resolver:    resolver.ID,
			ResolverURL: resolver.URL,
		}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Printf("[ERROR] Failed to create request: %v", err)
			continue
		}

		req.Header.Set("Accept", "application/dns-json")

		resp, err := d.Client.Do(req)
		if err != nil {
			log.Printf("[ERROR] Failed to send request: %v", err)
			continue
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("[ERROR] Failed to read response: %v", err)
			continue
		}

		var dohResp DOHResponse
		if err := json.Unmarshal(body, &dohResp); err != nil {
			log.Printf("[ERROR] Failed to parse JSON response: %v", err)
			continue
		}

		if len(dohResp.Answer) > 0 || len(dohResp.Authority) > 0 {
			return &dohResp, resolverInfo, nil
		}
	}

	return nil, ResolverInfo{}, fmt.Errorf("all resolvers failed or no answer for domain: %s", domain)
}
