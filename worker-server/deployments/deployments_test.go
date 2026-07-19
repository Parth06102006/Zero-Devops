package deployments

import "testing"

func TestValidateCloneURL_ValidHTTPSGithub(t *testing.T) {
	urls := []string{
		"https://github.com/user/repo.git",
		"https://github.com/org/project",
		"https://github.com/org/project.git",
	}
	for _, u := range urls {
		if err := validateCloneURL(u); err != nil {
			t.Errorf("validateCloneURL(%q) = %v, want nil", u, err)
		}
	}
}

func TestValidateCloneURL_RejectsNonHTTPS(t *testing.T) {
	urls := []string{
		"http://github.com/user/repo.git",
		"git@github.com:user/repo.git",
		"ftp://github.com/user/repo",
		"file:///path/to/repo",
	}
	for _, u := range urls {
		if err := validateCloneURL(u); err == nil {
			t.Errorf("validateCloneURL(%q) = nil, want error for non-https scheme", u)
		}
	}
}

func TestValidateCloneURL_RejectsDashPrefix(t *testing.T) {
	urls := []string{
		"--depth=1",
		"-oUserKnownHostsFile=/dev/null",
		"-",
	}
	for _, u := range urls {
		if err := validateCloneURL(u); err == nil {
			t.Errorf("validateCloneURL(%q) = nil, want error for dash prefix", u)
		}
	}
}

func TestValidateCloneURL_RejectsNonGithubHost(t *testing.T) {
	urls := []string{
		"https://gitlab.com/user/repo.git",
		"https://bitbucket.org/user/repo",
		"https://example.com/repo",
	}
	for _, u := range urls {
		if err := validateCloneURL(u); err == nil {
			t.Errorf("validateCloneURL(%q) = nil, want error for non-github host", u)
		}
	}
}

func TestValidateCloneURL_RejectsMalformedURL(t *testing.T) {
	urls := []string{
		":invalid",
		"%",
		"https://",
	}
	for _, u := range urls {
		if err := validateCloneURL(u); err == nil {
			t.Errorf("validateCloneURL(%q) = nil, want error for malformed URL", u)
		}
	}
}
