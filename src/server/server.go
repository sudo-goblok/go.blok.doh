package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/miekg/dns"
	"go.blok.doh/cache"
	"go.blok.doh/doh"
	"go.blok.doh/logdb"
)

type SOARecord struct {
	PrimaryNS  string
	AdminEmail string
	Serial     int
	Refresh    int
	Retry      int
	Expire     int
	MinimumTTL int
}
type UDPServer struct {
	Port        int
	BufferSize  int
	DOHClient   *doh.DOHClient
	Cache       *cache.DNSTTLCache
	RateLimiter *RateLimiterMap
}

func ParseSOA(soaString string) (*SOARecord, error) {
	soaParts := strings.Fields(soaString)
	if len(soaParts) < 7 {
		return nil, fmt.Errorf("invalid SOA record format")
	}

	serial, err := strconv.Atoi(soaParts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid serial number")
	}
	refresh, err := strconv.Atoi(soaParts[3])
	if err != nil {
		return nil, fmt.Errorf("invalid refresh value")
	}
	retry, err := strconv.Atoi(soaParts[4])
	if err != nil {
		return nil, fmt.Errorf("invalid retry value")
	}
	expire, err := strconv.Atoi(soaParts[5])
	if err != nil {
		return nil, fmt.Errorf("invalid expire value")
	}
	minTTL, err := strconv.Atoi(soaParts[6])
	if err != nil {
		return nil, fmt.Errorf("invalid minimum TTL value")
	}

	adminEmail := soaParts[1]

	return &SOARecord{
		PrimaryNS:  soaParts[0],
		AdminEmail: adminEmail,
		Serial:     serial,
		Refresh:    refresh,
		Retry:      retry,
		Expire:     expire,
		MinimumTTL: minTTL,
	}, nil
}

func BuildResp(conn *net.UDPConn, domain string, response *dns.Msg, responseData *doh.DOHResponse, remoteAddr *net.UDPAddr) (*dns.Msg, logdb.DNSLog) {
	hasAnswer := false
	dnsLog := logdb.DNSLog{
		Timestamp: time.Now().UnixNano(),
		ClientIP:  remoteAddr.IP.String(),
		Query:     domain,
		QueryType: int(response.Question[0].Qtype),
		Resolver:  "DOH",
	}
	// **Proses Answer Section**
	for _, answer := range responseData.Answer {
		switch answer.Type {
		case int(dns.TypeA):
			ip := net.ParseIP(answer.Data)
			if ip != nil {
				response.Answer = append(response.Answer, &dns.A{
					Hdr: dns.RR_Header{
						Name:   dns.Fqdn(domain),
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    uint32(answer.TTL),
					},
					A: ip,
				})
				dnsLog.Response = append(dnsLog.Response, struct {
					Name  string `json:"name"`
					Type  int    `json:"type"`
					Class int    `json:"class"`
					TTL   int    `json:"TTL"`
					Data  string `json:"data"`
				}{
					Name:  dns.Fqdn(domain),
					Type:  int(dns.TypeA),
					Class: int(dns.ClassINET),
					TTL:   int(answer.TTL),
					Data:  answer.Data,
				})
				hasAnswer = true
			}
		case int(dns.TypeAAAA):
			ip := net.ParseIP(answer.Data)
			if ip != nil {
				response.Answer = append(response.Answer, &dns.AAAA{
					Hdr: dns.RR_Header{
						Name:   dns.Fqdn(domain),
						Rrtype: dns.TypeAAAA,
						Class:  dns.ClassINET,
						Ttl:    uint32(answer.TTL),
					},
					AAAA: ip,
				})

				dnsLog.Response = append(dnsLog.Response, struct {
					Name  string `json:"name"`
					Type  int    `json:"type"`
					Class int    `json:"class"`
					TTL   int    `json:"TTL"`
					Data  string `json:"data"`
				}{
					Name:  dns.Fqdn(domain),
					Type:  int(dns.TypeAAAA),
					Class: int(dns.ClassINET),
					TTL:   int(answer.TTL),
					Data:  answer.Data,
				})

				hasAnswer = true
			}
		case int(dns.TypeCNAME):
			response.Answer = append(response.Answer, &dns.CNAME{
				Hdr: dns.RR_Header{
					Name:   dns.Fqdn(domain),
					Rrtype: dns.TypeCNAME,
					Class:  dns.ClassINET,
					Ttl:    uint32(answer.TTL),
				},
				Target: dns.Fqdn(answer.Data),
			})

			dnsLog.Response = append(dnsLog.Response, struct {
				Name  string `json:"name"`
				Type  int    `json:"type"`
				Class int    `json:"class"`
				TTL   int    `json:"TTL"`
				Data  string `json:"data"`
			}{
				Name:  dns.Fqdn(domain),
				Type:  int(dns.TypeCNAME),
				Class: int(dns.ClassINET),
				TTL:   int(answer.TTL),
				Data:  answer.Data,
			})
			hasAnswer = true
		case int(dns.TypeMX):
			response.Answer = append(response.Answer, &dns.MX{
				Hdr: dns.RR_Header{
					Name:   dns.Fqdn(domain),
					Rrtype: dns.TypeMX,
					Class:  dns.ClassINET,
					Ttl:    uint32(answer.TTL),
				},
				Preference: 10, // Default priority
				Mx:         dns.Fqdn(answer.Data),
			})

			dnsLog.Response = append(dnsLog.Response, struct {
				Name  string `json:"name"`
				Type  int    `json:"type"`
				Class int    `json:"class"`
				TTL   int    `json:"TTL"`
				Data  string `json:"data"`
			}{
				Name:  dns.Fqdn(domain),
				Type:  int(dns.TypeMX),
				Class: int(dns.ClassINET),
				TTL:   int(answer.TTL),
				Data:  answer.Data,
			})

			hasAnswer = true
		case int(dns.TypeTXT):
			response.Answer = append(response.Answer, &dns.TXT{
				Hdr: dns.RR_Header{
					Name:   dns.Fqdn(domain),
					Rrtype: dns.TypeTXT,
					Class:  dns.ClassINET,
					Ttl:    uint32(answer.TTL),
				},
				Txt: []string{answer.Data},
			})

			dnsLog.Response = append(dnsLog.Response, struct {
				Name  string `json:"name"`
				Type  int    `json:"type"`
				Class int    `json:"class"`
				TTL   int    `json:"TTL"`
				Data  string `json:"data"`
			}{
				Name:  dns.Fqdn(domain),
				Type:  int(dns.TypeTXT),
				Class: int(dns.ClassINET),
				TTL:   int(answer.TTL),
				Data:  strings.Join([]string{answer.Data}, " "),
			})

			hasAnswer = true
		}
	}

	// **Cek Authority**

	if !hasAnswer && len(responseData.Authority) > 0 {
		log.Printf("[INFO] No Answer found, but Authority section exists, got %d", len(responseData.Authority))

		for _, authority := range responseData.Authority {

			switch int(authority.Type) {
			case int(dns.TypeNS):
				response.Ns = append(response.Ns, &dns.NS{
					Hdr: dns.RR_Header{
						Name:   dns.Fqdn(domain),
						Rrtype: dns.TypeNS,
						Class:  dns.ClassINET,
						Ttl:    uint32(authority.TTL),
					},
					Ns: dns.Fqdn(authority.Data),
				})

				dnsLog.Response = append(dnsLog.Response, struct {
					Name  string `json:"name"`
					Type  int    `json:"type"`
					Class int    `json:"class"`
					TTL   int    `json:"TTL"`
					Data  string `json:"data"`
				}{
					Name:  dns.Fqdn(domain),
					Type:  int(dns.TypeNS),
					Class: int(dns.ClassINET),
					TTL:   int(authority.TTL),
					Data:  authority.Data,
				})

			case int(dns.TypeSOA):
				soaRecord, err := ParseSOA(authority.Data)
				if err != nil {
					fmt.Println("Error:", err)
					return nil, logdb.DNSLog{}
				}
				response.Ns = append(response.Ns, &dns.SOA{
					Hdr: dns.RR_Header{
						Name:   dns.Fqdn(domain),
						Rrtype: dns.TypeSOA,
						Class:  dns.ClassINET,
						Ttl:    uint32(authority.TTL),
					},
					Ns:      dns.Fqdn(soaRecord.PrimaryNS),
					Mbox:    soaRecord.AdminEmail,
					Serial:  uint32(soaRecord.Serial),
					Refresh: uint32(soaRecord.Refresh),
					Retry:   uint32(soaRecord.Retry),
					Expire:  uint32(soaRecord.Expire),
					Minttl:  uint32(soaRecord.MinimumTTL),
				})

				dnsLog.Response = append(dnsLog.Response, struct {
					Name  string `json:"name"`
					Type  int    `json:"type"`
					Class int    `json:"class"`
					TTL   int    `json:"TTL"`
					Data  string `json:"data"`
				}{
					Name:  dns.Fqdn(domain),
					Type:  int(dns.TypeSOA),
					Class: int(dns.ClassINET),
					TTL:   int(authority.TTL),
					Data:  authority.Data,
				})

			}
		}
	}

	if !hasAnswer && len(response.Ns) == 0 {
		response.Rcode = dns.RcodeNameError
	}
	// Serialize response
	responseBytes, err := response.Pack()
	if err != nil {
		log.Printf("[ERROR] Failed to serialize DNS response: %v", err)
		return nil, logdb.DNSLog{}
	}

	_, err = conn.WriteToUDP(responseBytes, remoteAddr)
	if err != nil {
		log.Printf("[ERROR] Failed to send response: %v", err)
		return nil, logdb.DNSLog{}
	}
	return response, dnsLog
}
func (u *UDPServer) Start() {

	addr := fmt.Sprintf(":%d", u.Port)
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		log.Fatalf("[ERROR] Failed to resolve UDP address: %v", err)
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Fatalf("[ERROR] Failed to start UDP server: %v", err)
	}
	defer conn.Close()

	logManager, err := logdb.NewLogManager("./dns_logs")
	if err != nil {
		fmt.Println("Error membuka database:", err)
		return
	}
	defer logManager.Close()

	u.Cache.StartCleanupLoop(30 * time.Second)

	log.Printf("[INFO] UDP server started on port %d\n", u.Port)

	buffer := make([]byte, u.BufferSize)
	for {
		n, remoteAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			log.Printf("[ERROR] Failed to read from UDP: %v", err)
			continue
		}

		go func(n int, remoteAddr *net.UDPAddr) {
			ipStr := remoteAddr.IP.String()
			limiter := u.RateLimiter.GetLimiter(ipStr)
			if !limiter.Allow() {
				log.Printf("[WARN] Rate limit exceeded for %s", ipStr)

				// Kirim respon DNS SERVFAIL
				// msg := new(dns.Msg)
				// msg.SetRcodeFormatError(&dns.Msg{}) // kosong dulu biar format benar
				// msg.Rcode = dns.RcodeServerFailure

				// respBytes, err := msg.Pack()
				// if err != nil {
				// 	log.Printf("[ERROR] Failed to pack rate-limit SERVFAIL response: %v", err)
				// 	return
				// }

				// _, err = conn.WriteToUDP(respBytes, remoteAddr)
				// if err != nil {
				// 	log.Printf("[ERROR] Failed to send SERVFAIL response: %v", err)
				// }
				return
			}
			msg := new(dns.Msg)
			err = msg.Unpack(buffer[:n])
			if err != nil {
				log.Printf("[ERROR] Failed to parse DNS query: %v", err)
				return
			}

			if len(msg.Question) == 0 {
				log.Printf("[ERROR] No question found in DNS query")
				return
			}

			domain := msg.Question[0].Name
			qtype := msg.Question[0].Qtype
			cacheKey := fmt.Sprintf("%s:%d", domain, qtype)

			log.Printf("[INFO] Received query for %s (type: %d) from %v", domain, qtype, remoteAddr)

			response := new(dns.Msg)
			response.SetReply(msg)
			response.Compress = true

			if cachedData, found := u.Cache.Get(cacheKey); found {
				log.Printf("[INFO] Found %t,  Cache hit for %s", found, cacheKey)
				var responseData *doh.DOHResponse
				err := json.Unmarshal(cachedData.([]byte), &responseData)
				if err != nil {
					log.Printf("[ERROR] Failed to deserialize DOHResponse: %v", err)
				}
				var logEntry logdb.DNSLog
				response, logEntry = BuildResp(conn, domain, response, responseData, remoteAddr)
				if response != nil {
					log.Print("[INFO] response from cache sent")
					logEntry.Resolver = "Cache"
					logEntry.ResolverURL = "cache://" + cacheKey
					// logEntry := logdb.DNSLog{
					// 	ClientIP:    ipStr,
					// 	Query:       domain,
					// 	QueryType:   int(qtype),
					// 	Resolver:    "Cache",
					// 	ResolverURL: "cache://" + cacheKey,
					// 	Response: []struct {
					// 		Name string `json:"name"`
					// 		Type int    `json:"type"`
					// 		TTL  int    `json:"TTL"`
					// 		Data string `json:"data"`
					// 	}{
					// 		{Name: "example.com", Type: 1, TTL: 60, Data: "93.184.216.34"},
					// 	},
					// 	Comment: []string{"Success"},
					// }
					logManager.SaveLog(logEntry)

				} else {
					log.Print("[ERROR] response from cache error")

				}
				return
			}

			responseData, resolverInfo, err := u.DOHClient.Query(domain, qtype, ipStr)

			if err != nil {
				log.Printf("[ERROR] Failed to resolve domain: %v", err)
				response.Rcode = dns.RcodeServerFailure
			} else {

				ttl := uint32(0) // Default TTL

				if len(responseData.Answer) > 0 {
					ttl = uint32(responseData.Answer[0].TTL)
				} else if len(responseData.Authority) > 0 {
					ttl = uint32(responseData.Authority[0].TTL)
				}
				serializedDohResp, err := json.Marshal(responseData)
				if err != nil {
					log.Printf("[ERROR] Failed to serialize DOHResponse: %v", err)
					return
				}

				u.Cache.Set(cacheKey, serializedDohResp, ttl)
				var logEntry logdb.DNSLog
				response, logEntry = BuildResp(conn, domain, response, responseData, remoteAddr)
				if response != nil {
					log.Print("[INFO] response sent")
					logEntry.Resolver = resolverInfo.Resolver
					logEntry.ResolverURL = resolverInfo.ResolverURL
					logManager.SaveLog(logEntry)
				} else {
					log.Print("[ERROR] response error")

				}
			}

		}(n, remoteAddr)
	}
}
