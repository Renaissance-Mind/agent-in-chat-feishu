package daemon

import "testing"

func TestNormalizeDaemonPATH_AppendsCommonExecutableDirs(t *testing.T) {
	exists := func(path string) bool {
		switch path {
		case "/opt/homebrew/bin", "/usr/local/bin", "/usr/bin", "/bin", "/usr/sbin", "/sbin":
			return true
		default:
			return false
		}
	}

	got := normalizeDaemonPATHWithExists("/Users/me/.local/bin:/usr/bin", exists)
	want := "/Users/me/.local/bin:/usr/bin:/opt/homebrew/bin:/usr/local/bin:/bin:/usr/sbin:/sbin"
	if got != want {
		t.Fatalf("normalizeDaemonPATHWithExists() = %q, want %q", got, want)
	}
}

func TestNormalizeDaemonPATH_DeduplicatesAndSkipsMissingDirs(t *testing.T) {
	exists := func(path string) bool {
		return path == "/bin" || path == "/usr/bin"
	}

	got := normalizeDaemonPATHWithExists("/bin:/bin:/custom/bin", exists)
	want := "/bin:/custom/bin:/usr/bin"
	if got != want {
		t.Fatalf("normalizeDaemonPATHWithExists() = %q, want %q", got, want)
	}
}
