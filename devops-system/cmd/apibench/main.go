// apibench is an HTTP load-testing tool that drives the full business
// pipeline to verify the system meets its performance targets.
//
// Division of labor with idbench:
//   - idbench: talks directly to devops-id over gRPC and measures the
//     issuance service's own throughput ceiling;
//   - apibench: hits the devops-system business write path (login → JWT →
//     Casbin → idclient prefetch → devops-id issuance → MySQL persistence),
//     measuring true end-to-end performance while wiring together the
//     Grafana HTTP RED / ID buffer / SQL connection pool / login business
//     metrics.
//
// Usage examples (run from the repository root):
//
//	# 20 concurrent workers creating users for 30s (consumes IDs, drives buffer refills), direct to the backend
//	go run ./devops-system/cmd/apibench -c 20 -d 30s
//
//	# Load test through the Higress gateway (edge + backend, full chain)
//	go run ./devops-system/cmd/apibench -addr http://127.0.0.1:8080 -host api.devops.local -c 20 -d 30s
//
//	# Load test the login endpoint (bcrypt verification is the main cost;
//	# note that every successful login creates a session)
//	go run ./devops-system/cmd/apibench -mode login -c 20 -d 30s
//
// SLO verdicts (aligned with the alert thresholds in deploy/prometheus/alerts.yml, adjustable as needed):
//   - Business write path P99 latency ≤ 1s (DevOpsHTTPP99High)
//   - Failure rate (HTTP non-200 or business code non-200) ≤ 1% (DevOpsHTTP5xxRateHigh definition);
//     during the run, also cross-check in Grafana that idgen_buffer_remaining
//     never hits zero, idgen_refill_failures_total stays at zero, and the SQL
//     connection pool has no waiting (DevOpsSQLPoolWaiting).
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// envelope is the unified response envelope (only the fields needed for the verdict are extracted).
type envelope struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data json.RawMessage `json:"data"`
}

// loginData holds data.access_token.
type loginData struct {
	AccessToken string `json:"access_token"`
}

func main() {
	addr := flag.String("addr", "http://127.0.0.1:9999", "devops-system base address (when going through the gateway, set the gateway address and configure -host)")
	host := flag.String("host", "", "HTTP Host header (api.devops.local when going through the gateway; leave empty for a direct connection)")
	mode := flag.String("mode", "create", "benchmark mode: create (create users, consumes IDs) | login (log in) | mixed (half and half)")
	username := flag.String("username", "admin", "login account (must have the system:user:create permission)")
	password := flag.String("password", "Admin@123", "login password")
	tenant := flag.String("tenant", "default", "tenant code")
	dept := flag.String("dept", "101", "deptId for created users (must already exist)")
	prefix := flag.String("prefix", "b_", "benchmark username prefix (a timestamp + sequence number is appended automatically to guarantee uniqueness; total length ≤ 30)")
	conc := flag.Int("c", 20, "number of concurrent goroutines")
	dur := flag.Duration("d", 20*time.Second, "benchmark duration (0 means run only until the -n total request count is reached)")
	total := flag.Int64("n", 0, "upper bound on total requests (0 means run only for the -d duration)")
	p99SLO := flag.Duration("p99-slo", time.Second, "SLO: P99 latency threshold")
	errSLO := flag.Float64("err-slo", 0.01, "SLO: failure rate threshold (0-1)")
	timeout := flag.Duration("timeout", 10*time.Second, "per-request timeout")
	flag.Parse()

	if *mode != "create" && *mode != "login" && *mode != "mixed" {
		fmt.Fprintf(os.Stderr, "apibench: -mode only supports create | login | mixed\n")
		os.Exit(2)
	}
	if *dur <= 0 && *total <= 0 {
		fmt.Fprintf(os.Stderr, "apibench: at least one of -d and -n must be given\n")
		os.Exit(2)
	}

	// Connection pool: the default Transport keeps only 2 idle connections per host, so it must be scaled up before benchmarking
	transport := &http.Transport{
		MaxIdleConns:        512,
		MaxIdleConnsPerHost: 512,
		IdleConnTimeout:     90 * time.Second,
	}
	client := &http.Client{Transport: transport, Timeout: *timeout}

	// Log in once to obtain a Bearer token (also serves as connection warm-up; retries tolerate startup jitter)
	var token string
	var lastErr error
	for i := 0; i < 5; i++ {
		token, lastErr = login(client, *addr, *host, *tenant, *username, *password)
		if lastErr == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if lastErr != nil {
		fmt.Fprintf(os.Stderr, "apibench: login failed: %v\n", lastErr)
		os.Exit(1)
	}
	fmt.Printf("login succeeded (%s), starting benchmark: mode=%s concurrency=%d\n", *addr, *mode, *conc)
	if *mode == "login" {
		fmt.Println("note: every successful request in login mode creates a Redis session")
	}
	if *mode == "create" {
		fmt.Println("note: create mode really writes user data (the bench prefix can be bulk-deleted afterwards)")
	}

	var (
		sent, okN atomic.Int64
		failN     atomic.Int64
		latMu     sync.Mutex
		latencies []time.Duration
		detailMu  sync.Mutex
		details   = map[string]int64{}
	)

	recordDetail := func(k string) {
		detailMu.Lock()
		details[k]++
		detailMu.Unlock()
	}

	startAt := time.Now()
	deadline := startAt.Add(*dur)
	stopped := func() bool {
		if *total > 0 && sent.Load() >= *total {
			return true
		}
		return *dur > 0 && time.Now().After(deadline)
	}

	progressDone := make(chan struct{})
	go func() {
		defer close(progressDone)
		last, lastAt := int64(0), startAt
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		fmt.Printf("elapsed \ttotal \tok \tfailed \trate\n")
		for range ticker.C {
			now := time.Now()
			cur := sent.Load()
			fmt.Printf("  %6.1fs  %7d  %7d  %5d  %.0f/s\n",
				time.Since(startAt).Seconds(), cur, okN.Load(), failN.Load(),
				float64(cur-last)/now.Sub(lastAt).Seconds())
			last, lastAt = cur, now
		}
	}()

	var seq atomic.Int64
	tsBase := time.Now().Unix()
	var wg sync.WaitGroup
	for i := 0; i < *conc; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			local := make([]time.Duration, 0, 1024)
			defer func() {
				latMu.Lock()
				latencies = append(latencies, local...)
				latMu.Unlock()
			}()
			for !stopped() {
				n := seq.Add(1)
				var (
					path, body string
					withToken  bool
				)
				switch *mode {
				case "login":
					path, body, withToken = "/system/api/v1/auth/login",
						fmt.Sprintf(`{"tenant_code":%q,"username":%q,"password":%q}`, *tenant, *username, *password), false
				case "create":
					path, body, withToken = "/system/api/v1/users",
						fmt.Sprintf(`{"username":"%s%d_%d","password":"Bench@123456","nickname":"bench","deptId":%q}`,
							*prefix, tsBase, n, *dept), true
				default: // mixed: alternate between the two modes
					if n%2 == 0 {
						path, body, withToken = "/system/api/v1/auth/login",
							fmt.Sprintf(`{"tenant_code":%q,"username":%q,"password":%q}`, *tenant, *username, *password), false
					} else {
						path, body, withToken = "/system/api/v1/users",
							fmt.Sprintf(`{"username":"%s%d_%d","password":"Bench@123456","nickname":"bench","deptId":%q}`,
								*prefix, tsBase, n, *dept), true
					}
				}

				t0 := time.Now()
				status, code, err := doPost(client, *addr, *host, path, body, token, withToken)
				local = append(local, time.Since(t0))
				sent.Add(1)
				if err != nil {
					failN.Add(1)
					recordDetail("transport_err:" + errClass(err))
					continue
				}
				if status == http.StatusOK && code == 200 {
					okN.Add(1)
					continue
				}
				failN.Add(1)
				recordDetail("http_" + strconv.Itoa(status) + "_code_" + strconv.Itoa(code))
			}
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(startAt)
	close(progressDone)

	// Summary
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	sentN := sent.Load()
	fail := failN.Load()
	fmt.Println()
	fmt.Printf("======== Benchmark Results ========\n")
	fmt.Printf("target %s (mode=%s) concurrency %d  duration %.1fs\n", *addr, *mode, *conc, elapsed.Seconds())
	fmt.Printf("total requests %d  ok %d  failed %d (%.2f%%)\n", sentN, okN.Load(), fail, pct(fail, sentN))
	if elapsed.Seconds() > 0 {
		fmt.Printf("QPS %.1f\n", float64(sentN)/elapsed.Seconds())
	}
	if n := len(latencies); n > 0 {
		var sum time.Duration
		for _, d := range latencies {
			sum += d
		}
		p99 := pctile(latencies, 0.99)
		fmt.Printf("latency  avg %s  P50 %s  P90 %s  P95 %s  P99 %s  P99.9 %s  max %s\n",
			round(sum/time.Duration(n)),
			round(pctile(latencies, 0.50)), round(pctile(latencies, 0.90)), round(pctile(latencies, 0.95)),
			round(p99), round(pctile(latencies, 0.999)), round(latencies[n-1]))
		if len(details) > 0 {
			fmt.Printf("failure details: ")
			for k, v := range details {
				fmt.Printf("%s=%d ", k, v)
			}
			fmt.Println()
		}

		// ---- SLO verdicts ----
		fmt.Println()
		fmt.Printf("======== SLO Verdicts ========\n")
		passP99 := p99 <= *p99SLO
		fmt.Printf("[%s] P99 latency %s ≤ %s\n", verdict(passP99), round(p99), round(*p99SLO))
		errRate := 1.0
		if sentN > 0 {
			errRate = float64(fail) / float64(sentN)
		}
		passErr := errRate <= *errSLO
		fmt.Printf("[%s] failure rate %.2f%% ≤ %.2f%%\n", verdict(passErr), errRate*100, *errSLO*100)
		fmt.Println()
		fmt.Println("also cross-check in Grafana (within the benchmark window):")
		fmt.Println("  - idgen_buffer_remaining never hit 0 (ID buffer not exhausted)")
		fmt.Println("  - idgen_refill_failures_total shows no series (zero refill failures)")
		fmt.Println("  - go_sql_wait_count_total shows no growth (no SQL connection pool waiting)")
		if !passP99 || !passErr {
			os.Exit(1)
		}
	}
}

// login calls the login endpoint and returns the access_token.
func login(client *http.Client, addr, host, tenant, username, password string) (string, error) {
	body := fmt.Sprintf(`{"tenant_code":%q,"username":%q,"password":%q}`, tenant, username, password)
	status, code, raw, err := doPostRaw(client, addr, host, "/system/api/v1/auth/login", body, "", false)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK || code != 200 {
		return "", fmt.Errorf("unexpected login response: http=%d code=%d body=%.200s", status, code, raw)
	}
	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return "", err
	}
	var data loginData
	if err := json.Unmarshal(env.Data, &data); err != nil {
		return "", err
	}
	if data.AccessToken == "" {
		return "", fmt.Errorf("response is missing access_token: %.200s", raw)
	}
	return data.AccessToken, nil
}

// doPost returns (http status, business code, error).
func doPost(client *http.Client, addr, host, path, body, token string, withToken bool) (int, int, error) {
	status, code, _, err := doPostRaw(client, addr, host, path, body, token, withToken)
	return status, code, err
}

// doPostRaw sends the request and parses the envelope code (a failed response may have no JSON body; the code is recorded as 0).
func doPostRaw(client *http.Client, addr, host, path, body, token string, withToken bool) (int, int, []byte, error) {
	req, err := http.NewRequest(http.MethodPost, addr+path, bytes.NewReader([]byte(body)))
	if err != nil {
		return 0, 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if host != "" {
		req.Host = host
	}
	if withToken {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, 0, nil, err
	}
	var env envelope
	_ = json.Unmarshal(raw, &env)
	return resp.StatusCode, env.Code, raw, nil
}

func pctile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(float64(len(sorted)-1) * p)
	if idx < 0 {
		idx = 0
	}
	return sorted[idx]
}

func round(d time.Duration) time.Duration {
	switch {
	case d > time.Millisecond:
		return d.Round(100 * time.Microsecond)
	case d > time.Microsecond:
		return d.Round(100 * time.Nanosecond)
	default:
		return d
	}
}

func pct(part, total int64) float64 {
	if total == 0 {
		return 0
	}
	return float64(part) / float64(total) * 100
}

func verdict(pass bool) string {
	if pass {
		return "PASS"
	}
	return "FAIL"
}

// errClass classifies transport-layer errors (connection refused / timeout, etc.).
func errClass(err error) string {
	s := err.Error()
	switch {
	case bytes.Contains([]byte(s), []byte("connection refused")):
		return "connection_refused"
	case bytes.Contains([]byte(s), []byte("Client.Timeout")):
		return "client_timeout"
	default:
		return "other"
	}
}
