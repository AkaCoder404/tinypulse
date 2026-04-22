package config

import (
	"fmt"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"
)

// Config represents the root of the YAML configuration file.
type Config struct {
	Notifiers map[string]NotifierConfig `yaml:"notifiers"`
	Endpoints map[string]EndpointConfig `yaml:"endpoints"`
}

// NotifierConfig represents a notifier definition in the YAML.
type NotifierConfig struct {
	Name   string                 `yaml:"name"`
	Type   string                 `yaml:"type"`
	Config map[string]interface{} `yaml:"config"` // Arbitrary key-value pairs for the notifier config
}

// EndpointConfig represents an endpoint definition in the YAML.
type EndpointConfig struct {
	Name            string   `yaml:"name"`
	Type            string   `yaml:"type"` // defaults to "http"
	URL             string   `yaml:"url"`
	IntervalSeconds int      `yaml:"interval_seconds"` // defaults to 60
	FailThreshold   int      `yaml:"fail_threshold"`   // defaults to 3
	Notifiers       []string `yaml:"notifiers"`        // list of notifier UIDs
}

// Parse reads the YAML configuration file, substitutes environment variables,
// and unmarshals it into a Config struct.
func Parse(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Substitute environment variables in the format ${VAR_NAME} or $VAR_NAME
	// Note: os.ExpandEnv handles both ${VAR} and $VAR.
	expandedData := os.ExpandEnv(string(data))

	var cfg Config
	if err := yaml.Unmarshal([]byte(expandedData), &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal yaml: %w", err)
	}

	// Apply defaults and validate
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	uidRegex := regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)

	supportedEndpointTypes := map[string]bool{"http": true, "tcp": true}
	supportedNotifierTypes := map[string]bool{"telegram": true, "pushover": true, "redis": true}

	for uid, n := range c.Notifiers {
		if !uidRegex.MatchString(uid) {
			return fmt.Errorf("invalid notifier UID %q: only alphanumeric, dash, dot, and underscore are allowed", uid)
		}
		if n.Name == "" {
			return fmt.Errorf("notifier %q is missing a name", uid)
		}
		if n.Type == "" {
			return fmt.Errorf("notifier %q is missing a type", uid)
		}
		if !supportedNotifierTypes[n.Type] {
			return fmt.Errorf("notifier %q uses unsupported type %q (must be lowercase e.g., 'telegram', 'pushover', 'redis')", uid, n.Type)
		}
		
		c.Notifiers[uid] = n
	}

	for uid, e := range c.Endpoints {
		if !uidRegex.MatchString(uid) {
			return fmt.Errorf("invalid endpoint UID %q: only alphanumeric, dash, dot, and underscore are allowed", uid)
		}
		if e.Name == "" {
			return fmt.Errorf("endpoint %q is missing a name", uid)
		}
		if e.URL == "" {
			return fmt.Errorf("endpoint %q is missing a url", uid)
		}
		
		// Set defaults
		ep := e
		if ep.Type == "" {
			ep.Type = "http"
		}
		if !supportedEndpointTypes[ep.Type] {
			return fmt.Errorf("endpoint %q uses unsupported type %q (must be lowercase e.g., 'http', 'tcp')", uid, ep.Type)
		}
		if ep.IntervalSeconds <= 0 {
			ep.IntervalSeconds = 60
		}
		if ep.FailThreshold <= 0 {
			ep.FailThreshold = 3
		}
		c.Endpoints[uid] = ep

		// Check if referenced notifiers exist
		for _, nUID := range ep.Notifiers {
			if _, exists := c.Notifiers[nUID]; !exists {
				return fmt.Errorf("endpoint %q references unknown notifier %q", uid, nUID)
			}
		}
	}

	return nil
}
