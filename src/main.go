package main

import (
	"flag"
	"log"

	"go.blok.doh/cache"
	"go.blok.doh/doh"
	"go.blok.doh/server"

	"github.com/spf13/viper"
	"golang.org/x/time/rate"
)

// Struktur konfigurasi resolver
type Resolver struct {
	ID     string `mapstructure:"id"`
	URL    string `mapstructure:"url"`
	Weight int    `mapstructure:"weight"`
}

// Struktur konfigurasi DOH
type DOHConfig struct {
	Resolvers []Resolver `mapstructure:"resolvers"`
}

// Struktur konfigurasi server
type ServerConfig struct {
	UDPPort        int  `mapstructure:"udp_port"`
	BufferSize     int  `mapstructure:"buffer_size"`
	EnableRecusion bool `mapstructure:"enable_recursion"`
}

// Struktur konfigurasi rate limit
type RateLimitCfg struct {
	MaxRequests   int `mapstructure:"max_requests"`
	WindowSeconds int `mapstructure:"window_seconds"`
}

// Struktur utama konfigurasi
type Config struct {
	DOH struct {
		Resolvers []doh.Resolver `mapstructure:"resolvers"`
	} `mapstructure:"doh"`
	Server    ServerConfig `mapstructure:"server"`
	RateLimit RateLimitCfg `mapstructure:"rate_limit"`
}

// Fungsi untuk memuat konfigurasi dari file YAML
func LoadConfig() (*Config, error) {
	viper.AddConfigPath("config")
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	if err := viper.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func main() {
	// Tambahkan argumen UDP Port
	udpPortArg := flag.Int("udp_port", 0, "Port UDP untuk server")
	flag.Parse()

	log.Println("[INFO] Starting go.blok.doh...")

	log.Println("[INFO] Loading configuration...")
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("[ERROR] Failed to load config: %v", err)
	}
	log.Println("[INFO] Configuration loaded successfully.")

	// Gunakan nilai dari argumen jika diberikan, jika tidak gunakan dari config
	udpPort := cfg.Server.UDPPort
	if *udpPortArg != 0 {
		udpPort = *udpPortArg
	}

	log.Println("[INFO] Initializing DOH client...")
	dohClient := doh.NewDOHClient(cfg.DOH.Resolvers)
	log.Println("[INFO] DOH client initialized.")

	log.Printf("[INFO] Starting UDP server on port %d...\n", udpPort)
	udpServer := &server.UDPServer{
		Port:        udpPort,
		BufferSize:  cfg.Server.BufferSize,
		DOHClient:   dohClient,
		Cache:       cache.NewDNSTTLCache(),
		RateLimiter: server.NewRateLimiterMap(rate.Limit(cfg.RateLimit.MaxRequests), cfg.RateLimit.MaxRequests),
	}

	udpServer.Start()
}
