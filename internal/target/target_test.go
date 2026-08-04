package target

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/soundadam/soundprobe/internal/model"
)

func TestExpandPreservesStationAndFamilyOrder(t *testing.T) {
	providers, err := Expand([]string{"nju-campus", "mlab", "qlu"}, FamilyDual)
	if err != nil {
		t.Fatal(err)
	}
	want := []model.Provider{
		model.ProviderNJUCampusIPv4,
		model.ProviderNJUCampusIPv6,
		model.ProviderMLab,
		model.ProviderQLUIPv4,
	}
	if len(providers) != len(want) {
		t.Fatalf("providers = %#v", providers)
	}
	for index := range want {
		if providers[index] != want[index] {
			t.Fatalf("providers[%d] = %q, want %q", index, providers[index], want[index])
		}
	}
}

func TestExpandRejectsBrowserProtectedEdge(t *testing.T) {
	if _, err := Expand([]string{"nju-edge"}, FamilyIPv4); err == nil {
		t.Fatal("Expand() accepted the browser-protected NJU Edge target")
	}
}

func TestExpandRejectsUnsupportedIPv6Station(t *testing.T) {
	if _, err := Expand([]string{"qlu"}, FamilyIPv6); err == nil {
		t.Fatal("Expand() succeeded for an IPv4-only station")
	}
}

func TestNewPlanDeduplicatesRepeatedStation(t *testing.T) {
	plan, err := NewPlan([]string{"mlab", "mlab", "nju-campus"}, FamilyIPv4)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Providers) != 2 || plan.Providers[0] != model.ProviderMLab || plan.Providers[1] != model.ProviderNJUCampusIPv4 {
		t.Fatalf("providers = %#v", plan.Providers)
	}
}

func TestProbeUsesConfiguredBackend(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/backend/empty.php" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		response.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	result := probe(context.Background(), Spec{StationID: "test", Family: "ipv4", ServerURL: server.URL}, time.Second)
	if result.Status != ProbeReachable || result.LatencyMS == nil {
		t.Fatalf("result = %#v", result)
	}
}

func TestLabelsExposeStationAndFamily(t *testing.T) {
	if got := Label(model.ProviderNJUEdgeIPv6); got != "NJU Edge · IPv6" {
		t.Fatalf("label = %q", got)
	}
	if got := Label(model.ProviderMLab); got != "M-Lab" {
		t.Fatalf("label = %q", got)
	}
}
