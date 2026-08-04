package target

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/soundadam/soundprobe/internal/model"
)

type Family string

const (
	FamilyIPv4 Family = "ipv4"
	FamilyIPv6 Family = "ipv6"
	FamilyDual Family = "dual"
)

type Station struct {
	ID                string
	Label             string
	Description       string
	IPv4              *Spec
	IPv6              *Spec
	MLab              bool
	TerminalSupported bool
	UnsupportedReason string
}

type Spec struct {
	Provider   model.Provider
	StationID  string
	Label      string
	Family     string
	ServerName string
	ServerURL  string
}

type ProbeStatus string

const (
	ProbeReachable   ProbeStatus = "reachable"
	ProbeUnreachable ProbeStatus = "unreachable"
	ProbeUnsupported ProbeStatus = "unsupported"
	ProbeAutomatic   ProbeStatus = "automatic"
)

type ProbeResult struct {
	StationID string      `json:"stationId"`
	Family    string      `json:"family"`
	Status    ProbeStatus `json:"status"`
	LatencyMS *float64    `json:"latencyMs"`
	Message   string      `json:"message,omitempty"`
}

type Plan struct {
	StationIDs []string         `json:"stationIds"`
	Family     Family           `json:"family"`
	Providers  []model.Provider `json:"providers"`
}

func NewPlan(stationIDs []string, family Family) (Plan, error) {
	providers, err := Expand(stationIDs, family)
	if err != nil {
		return Plan{}, err
	}
	ids := make([]string, len(stationIDs))
	copy(ids, stationIDs)
	return Plan{StationIDs: ids, Family: family, Providers: providers}, nil
}

var stations = []Station{
	{
		ID:                "nju-campus",
		Label:             "NJU Campus",
		Description:       "NJU campus-internal path",
		IPv4:              &Spec{Provider: model.ProviderNJUCampusIPv4, StationID: "nju-campus", Label: "NJU Campus · IPv4", Family: "ipv4", ServerName: "NJU Campus IPv4", ServerURL: "http://speed.nju.edu.cn"},
		IPv6:              &Spec{Provider: model.ProviderNJUCampusIPv6, StationID: "nju-campus", Label: "NJU Campus · IPv6", Family: "ipv6", ServerName: "NJU Campus IPv6", ServerURL: "http://speed6.nju.edu.cn"},
		TerminalSupported: true,
	},
	{
		ID:                "nju-edge",
		Label:             "NJU Edge",
		Description:       "public path to NJU's internet-facing edge",
		IPv4:              &Spec{Provider: model.ProviderNJUEdgeIPv4, StationID: "nju-edge", Label: "NJU Edge · IPv4", Family: "ipv4", ServerName: "NJU Edge IPv4", ServerURL: "http://test.nju.edu.cn"},
		IPv6:              &Spec{Provider: model.ProviderNJUEdgeIPv6, StationID: "nju-edge", Label: "NJU Edge · IPv6", Family: "ipv6", ServerName: "NJU Edge IPv6", ServerURL: "http://test6.nju.edu.cn"},
		TerminalSupported: false,
		UnsupportedReason: "browser verification blocks the official backend from CLI clients",
	},
	{
		ID:                "mlab",
		Label:             "M-Lab",
		Description:       "general internet NDT7 measurement",
		MLab:              true,
		TerminalSupported: true,
	},
	{
		ID:                "cernet",
		Label:             "CERNET",
		Description:       "CERNET public LibreSpeed station",
		IPv4:              &Spec{Provider: model.ProviderCERNETIPv4, StationID: "cernet", Label: "CERNET · IPv4", Family: "ipv4", ServerName: "CERNET", ServerURL: "http://speedtest.sec.edu.cn"},
		TerminalSupported: true,
	},
	{
		ID:                "qlu",
		Label:             "QLU",
		Description:       "Qilu University of Technology LibreSpeed station",
		IPv4:              &Spec{Provider: model.ProviderQLUIPv4, StationID: "qlu", Label: "QLU · IPv4", Family: "ipv4", ServerName: "QLU", ServerURL: "https://speed.qlu.edu.cn"},
		TerminalSupported: true,
	},
	{
		ID:                "tongji",
		Label:             "Tongji",
		Description:       "Tongji University LibreSpeed station",
		IPv4:              &Spec{Provider: model.ProviderTongjiIPv4, StationID: "tongji", Label: "Tongji · IPv4", Family: "ipv4", ServerName: "Tongji", ServerURL: "https://dev.tongji.edu.cn/speedtest"},
		TerminalSupported: true,
	},
}

var stationByID map[string]Station
var specByProvider map[model.Provider]Spec

func init() {
	stationByID = make(map[string]Station, len(stations))
	specByProvider = make(map[model.Provider]Spec, len(stations)*2)
	for _, station := range stations {
		stationByID[station.ID] = station
		if station.IPv4 != nil {
			specByProvider[station.IPv4.Provider] = *station.IPv4
		}
		if station.IPv6 != nil {
			specByProvider[station.IPv6.Provider] = *station.IPv6
		}
	}
}

func Stations() []Station {
	result := make([]Station, len(stations))
	copy(result, stations)
	return result
}

func StationByID(id string) (Station, bool) {
	station, ok := stationByID[strings.ToLower(strings.TrimSpace(id))]
	return station, ok
}

func SpecFor(provider model.Provider) (Spec, bool) {
	spec, ok := specByProvider[provider]
	return spec, ok
}

func Label(provider model.Provider) string {
	if provider == model.ProviderMLab {
		return "M-Lab"
	}
	if provider == model.ProviderCampus {
		return "NJU Campus · IPv4"
	}
	if spec, ok := SpecFor(provider); ok {
		return spec.Label
	}
	return string(provider)
}

func Expand(ids []string, family Family) ([]model.Provider, error) {
	if len(ids) == 0 {
		return nil, errors.New("at least one target is required")
	}
	if family != FamilyIPv4 && family != FamilyIPv6 && family != FamilyDual {
		return nil, fmt.Errorf("unsupported family %q", family)
	}
	providers := make([]model.Provider, 0, len(ids)*2)
	seen := map[model.Provider]struct{}{}
	appendProvider := func(provider model.Provider) {
		if _, exists := seen[provider]; exists {
			return
		}
		seen[provider] = struct{}{}
		providers = append(providers, provider)
	}
	for _, rawID := range ids {
		id := strings.ToLower(strings.TrimSpace(rawID))
		station, ok := StationByID(id)
		if !ok {
			return nil, fmt.Errorf("unknown target %q", rawID)
		}
		if !station.TerminalSupported {
			return nil, fmt.Errorf("target %q is unavailable in terminal mode: %s", id, station.UnsupportedReason)
		}
		if station.MLab {
			appendProvider(model.ProviderMLab)
			continue
		}
		switch family {
		case FamilyIPv4:
			if station.IPv4 == nil {
				return nil, fmt.Errorf("target %q does not support IPv4", id)
			}
			appendProvider(station.IPv4.Provider)
		case FamilyIPv6:
			if station.IPv6 == nil {
				return nil, fmt.Errorf("target %q does not support IPv6", id)
			}
			appendProvider(station.IPv6.Provider)
		case FamilyDual:
			if station.IPv4 != nil {
				appendProvider(station.IPv4.Provider)
			}
			if station.IPv6 != nil {
				appendProvider(station.IPv6.Provider)
			}
			if station.IPv4 == nil && station.IPv6 == nil {
				return nil, fmt.Errorf("target %q has no supported address family", id)
			}
		}
	}
	return providers, nil
}

func NeedsMLab(providers []model.Provider) bool {
	for _, provider := range providers {
		if provider == model.ProviderMLab {
			return true
		}
	}
	return false
}

func StationIDs(providers []model.Provider) []string {
	ids := make([]string, 0, len(providers))
	seen := map[string]struct{}{}
	for _, provider := range providers {
		id := ""
		if provider == model.ProviderMLab {
			id = "mlab"
		} else if spec, ok := SpecFor(provider); ok {
			id = spec.StationID
		}
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func ProbeAll(ctx context.Context, timeout time.Duration) []ProbeResult {
	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}
	results := make([]ProbeResult, 0, len(stations)*2)
	var mutex sync.Mutex
	var wait sync.WaitGroup
	for _, station := range stations {
		station := station
		if !station.TerminalSupported {
			mutex.Lock()
			for _, spec := range []*Spec{station.IPv4, station.IPv6} {
				if spec == nil {
					continue
				}
				results = append(results, ProbeResult{StationID: station.ID, Family: spec.Family, Status: ProbeUnsupported, Message: station.UnsupportedReason})
			}
			mutex.Unlock()
			continue
		}
		if station.MLab {
			mutex.Lock()
			results = append(results, ProbeResult{StationID: station.ID, Family: "auto", Status: ProbeAutomatic, Message: "automatic server selection"})
			mutex.Unlock()
			continue
		}
		for _, spec := range []*Spec{station.IPv4, station.IPv6} {
			if spec == nil {
				continue
			}
			spec := *spec
			wait.Add(1)
			go func() {
				defer wait.Done()
				result := probe(ctx, spec, timeout)
				mutex.Lock()
				results = append(results, result)
				mutex.Unlock()
			}()
		}
	}
	wait.Wait()
	sort.Slice(results, func(i, j int) bool {
		if results[i].StationID == results[j].StationID {
			return results[i].Family < results[j].Family
		}
		return results[i].StationID < results[j].StationID
	})
	return results
}

func probe(parent context.Context, spec Spec, timeout time.Duration) ProbeResult {
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()
	startedAt := time.Now()
	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:  timeout,
			Resolver: net.DefaultResolver,
		}).DialContext,
		TLSHandshakeTimeout: timeout,
		DisableKeepAlives:   true,
	}
	client := &http.Client{Transport: transport, Timeout: timeout}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(spec.ServerURL, "/")+"/backend/empty.php", nil)
	if err != nil {
		return ProbeResult{StationID: spec.StationID, Family: spec.Family, Status: ProbeUnreachable, Message: err.Error()}
	}
	response, err := client.Do(request)
	latency := float64(time.Since(startedAt).Microseconds()) / 1000
	if err != nil {
		return ProbeResult{StationID: spec.StationID, Family: spec.Family, Status: ProbeUnreachable, LatencyMS: &latency, Message: compactError(err)}
	}
	_ = response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 400 {
		return ProbeResult{StationID: spec.StationID, Family: spec.Family, Status: ProbeUnreachable, LatencyMS: &latency, Message: response.Status}
	}
	return ProbeResult{StationID: spec.StationID, Family: spec.Family, Status: ProbeReachable, LatencyMS: &latency}
}

func compactError(err error) string {
	message := strings.Join(strings.Fields(err.Error()), " ")
	if len(message) > 160 {
		message = message[:160]
	}
	return message
}
