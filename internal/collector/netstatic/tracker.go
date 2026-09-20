package netstatic

import (
	"encoding/json"
	"errors"
	"fmt"
	log "github.com/komari-probe/komari-probe-agent/internal/logging"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	gnet "github.com/shirou/gopsutil/v4/net"
)

const (
	defaultDataPreserveDays = 31.0
	defaultDetectInterval   = 2 * time.Second
	defaultSaveInterval     = 10 * time.Minute
	defaultSaveFilePath     = "./net_static.json"
)

// Config controls retention, sampling, persistence, and monitored interfaces.
type Config struct {
	DataPreserveDays float64  `json:"data_preserve_day"`
	DetectInterval   float64  `json:"detect_interval"`
	SaveInterval     float64  `json:"save_interval"`
	NICs             []string `json:"nics"`
}

// TrafficData is the traffic delta observed for one interface at a point in time.
type TrafficData struct {
	Timestamp uint64 `json:"timestamp"`
	Tx        uint64 `json:"tx"`
	Rx        uint64 `json:"rx"`
}

type persistedData struct {
	Interfaces map[string][]TrafficData `json:"interfaces"`
	Config     Config                   `json:"config"`
}

// Tracker owns traffic-history state for one Collector instance.
type Tracker struct {
	mu sync.Mutex

	filePath     string
	config       Config
	interfaces   map[string][]TrafficData
	cache        map[string][]TrafficData
	lastCounters map[string]trafficCounters

	running      bool
	detectTicker *time.Ticker
	saveTicker   *time.Ticker
	stopCh       chan struct{}
	workerWG     sync.WaitGroup
}

type trafficCounters struct {
	Tx uint64
	Rx uint64
}

// NewTracker creates an independent traffic tracker. An empty path uses the
// historical default path in the Agent working directory.
func NewTracker(filePath string) *Tracker {
	if filePath == "" {
		filePath = defaultSaveFilePath
	}
	return &Tracker{
		filePath:     filePath,
		config:       defaultConfig(Config{}),
		interfaces:   make(map[string][]TrafficData),
		cache:        make(map[string][]TrafficData),
		lastCounters: make(map[string]trafficCounters),
	}
}

// Start loads persisted history and begins collecting traffic deltas.
func (t *Tracker) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.running {
		return nil
	}
	if err := t.loadLocked(); err != nil {
		return err
	}
	t.startLocked()
	return nil
}

// ConfigureNICs updates the monitored network-interface allowlist. A nil or
// empty slice records traffic for every interface.
func (t *Tracker) ConfigureNICs(nicNames []string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.config.NICs = append([]string(nil), nicNames...)
	if t.running && len(t.config.NICs) > 0 {
		allowed := make(map[string]struct{}, len(t.config.NICs))
		for _, name := range t.config.NICs {
			allowed[name] = struct{}{}
		}
		for name := range t.lastCounters {
			if _, ok := allowed[name]; !ok {
				delete(t.lastCounters, name)
			}
		}
		for name := range t.cache {
			if _, ok := allowed[name]; !ok {
				delete(t.cache, name)
			}
		}
	}
	t.purgeExpiredLocked()
	return t.saveLocked()
}

// TotalTrafficBetween returns totals by interface in the requested Unix-time
// range. Zero for either bound leaves that side unbounded.
func (t *Tracker) TotalTrafficBetween(start, end uint64) (map[string]TrafficData, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	totals := make(map[string]TrafficData)
	inRange := func(timestamp uint64) bool {
		return (start == 0 || timestamp >= start) && (end == 0 || timestamp <= end)
	}
	add := func(name string, tx, rx uint64) {
		current := totals[name]
		current.Tx += tx
		current.Rx += rx
		totals[name] = current
	}
	for name, samples := range t.interfaces {
		for _, sample := range samples {
			if inRange(sample.Timestamp) {
				add(name, sample.Tx, sample.Rx)
			}
		}
	}
	for name, samples := range t.cache {
		for _, sample := range samples {
			if inRange(sample.Timestamp) {
				add(name, sample.Tx, sample.Rx)
			}
		}
	}
	return totals, nil
}

// Close stops background collection and flushes pending traffic data.
func (t *Tracker) Close() error {
	t.mu.Lock()
	if !t.running {
		t.mu.Unlock()
		return nil
	}
	t.running = false
	t.detectTicker.Stop()
	t.saveTicker.Stop()
	close(t.stopCh)
	t.flushLocked(uint64(time.Now().Unix()))
	t.purgeExpiredLocked()
	err := t.saveLocked()
	t.mu.Unlock()
	t.workerWG.Wait()
	return err
}

func (t *Tracker) startLocked() {
	t.config = defaultConfig(t.config)
	t.detectTicker = time.NewTicker(time.Duration(t.config.DetectInterval * float64(time.Second)))
	t.saveTicker = time.NewTicker(time.Duration(t.config.SaveInterval * float64(time.Second)))
	t.stopCh = make(chan struct{})
	t.running = true
	t.workerWG.Add(2)
	go func() {
		defer t.workerWG.Done()
		t.collectLoop(t.detectTicker, t.stopCh)
	}()
	go func() {
		defer t.workerWG.Done()
		t.saveLoop(t.saveTicker, t.stopCh)
	}()
}

func (t *Tracker) collectLoop(ticker *time.Ticker, stop <-chan struct{}) {
	for {
		select {
		case <-ticker.C:
			t.mu.Lock()
			if !t.running {
				t.mu.Unlock()
				return
			}
			t.sampleLocked()
			t.mu.Unlock()
		case <-stop:
			return
		}
	}
}

func (t *Tracker) saveLoop(ticker *time.Ticker, stop <-chan struct{}) {
	for {
		select {
		case now := <-ticker.C:
			t.mu.Lock()
			if !t.running {
				t.mu.Unlock()
				return
			}
			t.flushLocked(uint64(now.Unix()))
			t.purgeExpiredLocked()
			if err := t.saveLocked(); err != nil {
				log.Printf("save network traffic statistics: %v", err)
			}
			t.mu.Unlock()
		case <-stop:
			return
		}
	}
}

func (t *Tracker) sampleLocked() {
	counters, err := gnet.IOCounters(true)
	if err != nil {
		return
	}
	timestamp := uint64(time.Now().Unix())
	for _, counter := range counters {
		if !t.isNICAllowedLocked(counter.Name) {
			continue
		}
		current := trafficCounters{Tx: counter.BytesSent, Rx: counter.BytesRecv}
		if previous, ok := t.lastCounters[counter.Name]; ok {
			tx := counterDelta(current.Tx, previous.Tx)
			rx := counterDelta(current.Rx, previous.Rx)
			if tx > 0 || rx > 0 {
				t.cache[counter.Name] = append(t.cache[counter.Name], TrafficData{Timestamp: timestamp, Tx: tx, Rx: rx})
			}
		}
		t.lastCounters[counter.Name] = current
	}
}

func (t *Tracker) isNICAllowedLocked(name string) bool {
	if len(t.config.NICs) == 0 {
		return true
	}
	for _, allowed := range t.config.NICs {
		if name == allowed {
			return true
		}
	}
	return false
}

func (t *Tracker) flushLocked(timestamp uint64) {
	for name, samples := range t.cache {
		var tx, rx uint64
		for _, sample := range samples {
			tx += sample.Tx
			rx += sample.Rx
		}
		if tx > 0 || rx > 0 {
			t.interfaces[name] = append(t.interfaces[name], TrafficData{Timestamp: timestamp, Tx: tx, Rx: rx})
		}
	}
	t.cache = make(map[string][]TrafficData)
}

func (t *Tracker) loadLocked() error {
	file, err := os.Open(t.filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("open traffic history: %w", err)
	}
	defer file.Close()

	contents, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("read traffic history: %w", err)
	}
	if len(contents) == 0 {
		return nil
	}

	var persisted persistedData
	if err := json.Unmarshal(contents, &persisted); err != nil {
		backupPath := t.filePath + ".bak"
		if backupErr := os.Rename(t.filePath, backupPath); backupErr != nil {
			log.Printf("backup corrupt traffic history: %v", backupErr)
		}
		return nil
	}
	if persisted.Interfaces != nil {
		t.interfaces = persisted.Interfaces
	}
	t.config = defaultConfig(persisted.Config)
	t.purgeExpiredLocked()
	return nil
}

func (t *Tracker) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(t.filePath), 0o755); err != nil {
		return fmt.Errorf("create traffic-history directory: %w", err)
	}
	persisted := persistedData{Interfaces: t.interfaces, Config: t.config}
	contents, err := json.Marshal(persisted)
	if err != nil {
		return fmt.Errorf("marshal traffic history: %w", err)
	}
	temporaryPath := t.filePath + ".tmp"
	if err := os.WriteFile(temporaryPath, contents, 0o644); err != nil {
		return fmt.Errorf("write traffic history: %w", err)
	}
	if err := os.Rename(temporaryPath, t.filePath); err != nil {
		return fmt.Errorf("replace traffic history: %w", err)
	}
	return nil
}

func (t *Tracker) purgeExpiredLocked() {
	retention := time.Duration(t.config.DataPreserveDays * 24 * float64(time.Hour))
	cutoff := uint64(time.Now().Add(-retention).Unix())
	for name, samples := range t.interfaces {
		kept := samples[:0]
		for _, sample := range samples {
			if sample.Timestamp >= cutoff {
				kept = append(kept, sample)
			}
		}
		if len(kept) == 0 {
			delete(t.interfaces, name)
		} else {
			t.interfaces[name] = kept
		}
	}
}

func defaultConfig(config Config) Config {
	if config.DataPreserveDays <= 0 {
		config.DataPreserveDays = defaultDataPreserveDays
	}
	if config.DetectInterval <= 0 {
		config.DetectInterval = defaultDetectInterval.Seconds()
	}
	if config.SaveInterval <= 0 {
		config.SaveInterval = defaultSaveInterval.Seconds()
	}
	return config
}

func counterDelta(current, previous uint64) uint64 {
	if current >= previous {
		return current - previous
	}
	return 0
}
