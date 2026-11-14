package handlers

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/luketaylor45/atlas/core/internal/models"
)

func TestReconcileStatus(t *testing.T) {
	old := 5 * time.Minute
	fresh := 5 * time.Second

	tests := []struct {
		name       string
		current    string
		since      time.Duration
		container  string
		installing bool
		want       *statusCorrection
	}{
		{"install still running is left alone", StatusInstalling, old, "", true, nil},
		{"node installing but core disagrees", StatusOffline, old, "stopped", true, &statusCorrection{Status: StatusInstalling}},
		{"fresh installing status is not judged yet", StatusInstalling, fresh, "", false, nil},
		{"stuck install with no container failed", StatusInstalling, old, "", false, &statusCorrection{Status: StatusInstallationFailed, Stage: interruptedInstallStage}},
		{"stuck install with a container finished", StatusInstalling, old, "stopped", false, &statusCorrection{Status: StatusOffline, Stage: "Installation complete"}},
		{"failed install stays failed", StatusInstallationFailed, old, "", false, nil},
		{"running but container stopped", StatusRunning, old, "stopped", false, &statusCorrection{Status: StatusOffline}},
		{"running but container gone", StatusRunning, old, "", false, &statusCorrection{Status: StatusOffline}},
		{"offline but container running", StatusOffline, old, "running", false, &statusCorrection{Status: StatusRunning}},
		{"stuck starting resolves to running", StatusStarting, old, "running", false, &statusCorrection{Status: StatusRunning}},
		{"stuck stopping resolves to offline", StatusStopping, old, "stopped", false, &statusCorrection{Status: StatusOffline}},
		{"fresh stopping is not judged yet", StatusStopping, fresh, "running", false, nil},
		{"agreement needs no change", StatusRunning, old, "running", false, nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := reconcileStatus(test.current, test.since, test.container, test.installing)
			if (got == nil) != (test.want == nil) {
				t.Fatalf("got %+v, want %+v", got, test.want)
			}
			if got != nil && *got != *test.want {
				t.Fatalf("got %+v, want %+v", *got, *test.want)
			}
		})
	}
}

func palworldPorts() []models.EggPort {
	return []models.EggPort{
		{Name: "Query", Env: "QUERY_PORT", Default: 27015, Protocol: "udp"},
		{Name: "RCON", Env: "RCON_PORT", Default: 25575, Protocol: "tcp"},
	}
}

func portsByEnv(plan []models.ServicePort) map[string]models.ServicePort {
	result := make(map[string]models.ServicePort, len(plan))
	for _, port := range plan {
		result[port.Env] = port
	}
	return result
}

func TestPlanServicePortsUsesDefaultsWhenFree(t *testing.T) {
	plan, err := planServicePorts(map[int]bool{}, 8211, palworldPorts(), nil)
	if err != nil {
		t.Fatal(err)
	}
	ports := portsByEnv(plan)
	if !ports[models.PrimaryPortEnv].IsPrimary || ports[models.PrimaryPortEnv].Port != 8211 {
		t.Fatalf("primary port = %+v", ports[models.PrimaryPortEnv])
	}
	if ports["QUERY_PORT"].Port != 27015 || ports["QUERY_PORT"].Protocol != "udp" {
		t.Fatalf("query port = %+v", ports["QUERY_PORT"])
	}
	if ports["RCON_PORT"].Port != 25575 || ports["RCON_PORT"].Protocol != "tcp" {
		t.Fatalf("rcon port = %+v", ports["RCON_PORT"])
	}
}

func TestPlanServicePortsSkipsPortsAlreadyInUse(t *testing.T) {
	// A second server of the same game: the defaults are taken.
	used := map[int]bool{8211: true, 27015: true, 27016: true, 25575: true}

	plan, err := planServicePorts(used, 8212, palworldPorts(), nil)
	if err != nil {
		t.Fatal(err)
	}
	ports := portsByEnv(plan)
	if ports["QUERY_PORT"].Port != 27017 {
		t.Fatalf("query port = %d, want the next free port 27017", ports["QUERY_PORT"].Port)
	}
	if ports["RCON_PORT"].Port != 25576 {
		t.Fatalf("rcon port = %d, want 25576", ports["RCON_PORT"].Port)
	}
}

func TestPlanServicePortsNeverReusesAPortWithinOneService(t *testing.T) {
	declared := []models.EggPort{
		{Name: "A", Env: "A_PORT", Default: 8211},
		{Name: "B", Env: "B_PORT", Default: 8211},
	}
	plan, err := planServicePorts(map[int]bool{}, 8211, declared, nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[int]bool{}
	for _, port := range plan {
		if seen[port.Port] {
			t.Fatalf("port %d was allocated twice: %+v", port.Port, plan)
		}
		seen[port.Port] = true
	}
}

func TestPlanServicePortsRejectsConflictsAndBadInput(t *testing.T) {
	tests := []struct {
		name      string
		used      map[int]bool
		primary   int
		requested map[string]int
		taken     bool
	}{
		{"primary in use", map[int]bool{8211: true}, 8211, nil, true},
		{"requested extra in use", map[int]bool{25575: true}, 8211, map[string]int{"RCON_PORT": 25575}, true},
		{"extra equals primary", map[int]bool{}, 8211, map[string]int{"RCON_PORT": 8211}, true},
		{"primary out of range", map[int]bool{}, 0, nil, false},
		{"extra out of range", map[int]bool{}, 8211, map[string]int{"RCON_PORT": 99999}, false},
		{"port the egg does not declare", map[int]bool{}, 8211, map[string]int{"OTHER_PORT": 1234}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := planServicePorts(test.used, test.primary, palworldPorts(), test.requested)
			if err == nil {
				t.Fatal("invalid plan was accepted")
			}
			if errors.Is(err, errPortTaken) != test.taken {
				t.Fatalf("errPortTaken = %t, want %t (%v)", errors.Is(err, errPortTaken), test.taken, err)
			}
		})
	}
}

func TestBuildServerSpecCarriesPortsEntryAndMergedEnvironment(t *testing.T) {
	ports, _ := json.Marshal(palworldPorts())
	service := &models.Service{
		UUID:        "14778ff7-a24f-43c5-b6d3-ec2febce7177",
		Memory:      8192,
		Cpu:         400,
		Port:        8211,
		DockerImage: "",
		Environment: `{"SERVER_NAME":"Mine","RCON_PORT":"1"}`,
		Egg: models.Egg{
			DockerImages:    `["image:one","image:two"]`,
			StartupCommand:  "./start {{SERVER_PORT}}",
			ScriptInstall:   "#!/bin/bash\necho hi",
			ScriptContainer: "installer:debian",
			ScriptEntry:     "bash",
			Ports:           string(ports),
			Variables: []models.EggVariable{
				{EnvironmentVariable: "SERVER_NAME", DefaultValue: "Default"},
				{EnvironmentVariable: "MAX_PLAYERS", DefaultValue: "32"},
			},
		},
		Ports: []models.ServicePort{
			{Port: 8211, Env: models.PrimaryPortEnv, IsPrimary: true},
			{Port: 25580, Env: "RCON_PORT", Protocol: "tcp"},
		},
	}

	spec := buildServerSpec(service)

	if spec["egg_image"] != "image:one" {
		t.Fatalf("egg_image = %v, want the egg's first image", spec["egg_image"])
	}
	if spec["install_entry"] != "bash" || spec["install_container"] != "installer:debian" {
		t.Fatalf("installer settings were not carried: %v", spec)
	}
	if spec["startup_command"] != "./start {{SERVER_PORT}}" {
		t.Fatalf("startup_command = %v, want the egg's command as fallback", spec["startup_command"])
	}

	extra := spec["ports"].([]map[string]interface{})
	if len(extra) != 1 || extra[0]["port"] != 25580 || extra[0]["protocol"] != "tcp" {
		t.Fatalf("extra ports = %v", extra)
	}

	environment := map[string]string{}
	if err := json.Unmarshal([]byte(spec["environment"].(string)), &environment); err != nil {
		t.Fatal(err)
	}
	if environment["SERVER_NAME"] != "Mine" || environment["MAX_PLAYERS"] != "32" {
		t.Fatalf("environment = %v", environment)
	}
	// The allocation wins over a stale or hand-edited variable.
	if environment["RCON_PORT"] != "25580" {
		t.Fatalf("RCON_PORT = %q, want the allocated port 25580", environment["RCON_PORT"])
	}
}

func TestEggPortsIgnoresInvalidDeclarations(t *testing.T) {
	egg := &models.Egg{Ports: `[{"name":"RCON","env":"RCON_PORT","default":25575,"protocol":"TCP"},{"name":"No env"},{"name":"Primary","env":"SERVER_PORT"}]`}
	ports := eggPorts(egg)
	if len(ports) != 1 || ports[0].Env != "RCON_PORT" || ports[0].Protocol != "tcp" {
		t.Fatalf("ports = %+v", ports)
	}
	if got := eggPorts(&models.Egg{Ports: "not json"}); got != nil {
		t.Fatalf("invalid JSON produced %+v", got)
	}
}

func TestPlanServicePortsDerivesPortsTheGameWorksOutItself(t *testing.T) {
	declared := []models.EggPort{
		{Name: "Query", Env: "QUERY_PORT", Default: 2457, Protocol: "udp", Offset: 1},
		{Name: "RCON", Env: "RCON_PORT", Default: 2458, Protocol: "tcp"},
	}

	// The derived port follows the primary one, whatever its default says.
	plan, err := planServicePorts(map[int]bool{}, 3000, declared, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for _, port := range plan {
		got[port.Env] = port.Port
	}
	if got["QUERY_PORT"] != 3001 || got["RCON_PORT"] != 2458 || got[models.PrimaryPortEnv] != 3000 {
		t.Fatalf("plan = %v", got)
	}

	// An independent port steps out of the way of a derived one.
	plan, err = planServicePorts(map[int]bool{}, 2457, declared, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, port := range plan {
		got[port.Env] = port.Port
	}
	if got["QUERY_PORT"] != 2458 || got["RCON_PORT"] != 2459 {
		t.Fatalf("plan = %v", got)
	}

	// A derived port that is taken cannot be moved: the game port has to change.
	_, err = planServicePorts(map[int]bool{3001: true}, 3000, declared, nil)
	var derived *derivedPortTakenError
	if !errors.As(err, &derived) || !errors.Is(err, errPortTaken) || derived.Port != 3001 {
		t.Fatalf("taken derived port: %v", err)
	}

	// Asking for the value it would get anyway is fine; anything else is not.
	if _, err := planServicePorts(map[int]bool{}, 3000, declared, map[string]int{"QUERY_PORT": 3001}); err != nil {
		t.Fatalf("matching request: %v", err)
	}
	if _, err := planServicePorts(map[int]bool{}, 3000, declared, map[string]int{"QUERY_PORT": 4000}); err == nil {
		t.Fatal("a derived port was allowed to differ from the game port plus its offset")
	}
	if _, err := planServicePorts(map[int]bool{}, 65535, declared, nil); err == nil {
		t.Fatal("a derived port above 65535 was accepted")
	}
}
