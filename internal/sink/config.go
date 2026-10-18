package sink

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/pilsu-choi/mcpgraph/internal/netutil"
)

// File names inside the data directory.
const (
	ConfigFile = "sink.json"
	OffsetFile = "sink.offset"
	LockFile   = "sink.lock"
)

// Config is the content of sink.json. Its existence turns the sink on.
type Config struct {
	URL   string `json:"url"`             // collector base URL, http or https
	Token string `json:"token,omitempty"` // sent as a Bearer token
	Host  string `json:"host"`            // name this machine reports as
}

// MaskedToken describes the token without revealing it.
func (c Config) MaskedToken() string {
	switch {
	case c.Token == "":
		return "(none)"
	case len(c.Token) <= 8:
		return "****"
	default:
		return c.Token[:2] + "****" + c.Token[len(c.Token)-2:]
	}
}

// CheckURL validates a collector URL. It returns a warning, not an error,
// when the URL is plain http to a non-loopback host (posture rule R7).
func CheckURL(raw string) (warning string, err error) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("collector URL %q: want http:// or https://host[:port]", raw)
	}
	if u.Scheme == "http" && !netutil.IsLoopbackHost(u.Hostname()) {
		return "events and the token travel unencrypted to " + u.Hostname() + "; use https", nil
	}
	return "", nil
}

// LoadConfig reads sink.json. ok is false when the sink is off.
func LoadConfig(dir string) (c Config, ok bool, err error) {
	b, err := os.ReadFile(filepath.Join(dir, ConfigFile))
	if errors.Is(err, os.ErrNotExist) {
		return Config{}, false, nil
	}
	if err != nil {
		return Config{}, false, fmt.Errorf("read sink config: %w", err)
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return Config{}, false, fmt.Errorf("parse %s: %w", ConfigFile, err)
	}
	if _, err := CheckURL(c.URL); err != nil {
		return Config{}, false, err
	}
	return c, true, nil
}

// SaveConfig validates c and writes sink.json with mode 0600.
func SaveConfig(dir string, c Config) error {
	if _, err := CheckURL(c.URL); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(filepath.Join(dir, ConfigFile), append(b, '\n'))
}

// RemoveConfig turns the sink off. Turning off an already-off sink is not
// an error. The offset is kept, so turning it on again resumes where it
// stopped instead of re-sending history.
func RemoveConfig(dir string) error {
	err := os.Remove(filepath.Join(dir, ConfigFile))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove sink config: %w", err)
	}
	return nil
}

// writeFile replaces path atomically with mode 0600: it writes a temp
// file in the same directory and renames it over the target, so a reader
// never sees half a file. os.CreateTemp already creates files as 0600.
func writeFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
