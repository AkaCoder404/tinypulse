package notifier

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// Compile-time assertion that Redis implements Provider
var _ Provider = (*Redis)(nil)

type Redis struct {
	Address   string `json:"address"`
	Password  string `json:"password"`
	DB        int    `json:"db"`
	KeyPrefix string `json:"key_prefix"`
}

func init() {
	Register("redis", func(configJSON string) (Provider, error) {
		var r Redis
		if err := json.Unmarshal([]byte(configJSON), &r); err != nil {
			return nil, fmt.Errorf("unmarshal redis config: %w", err)
		}
		if r.Address == "" {
			return nil, fmt.Errorf("redis config missing address")
		}
		if r.KeyPrefix == "" {
			r.KeyPrefix = "tinypulse"
		}
		return &r, nil
	})
}

func (r *Redis) Type() string {
	return "redis"
}

func (r *Redis) Send(ctx context.Context, endpointID int64, endpointName, title, message string) error {
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", r.Address)
	if err != nil {
		return fmt.Errorf("redis dial %s: %w", r.Address, err)
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	}

	br := bufio.NewReader(conn)

	if r.Password != "" {
		if err := r.sendCommand(conn, br, "AUTH", r.Password); err != nil {
			return fmt.Errorf("redis auth: %w", err)
		}
	}

	if r.DB != 0 {
		if err := r.sendCommand(conn, br, "SELECT", strconv.Itoa(r.DB)); err != nil {
			return fmt.Errorf("redis select db: %w", err)
		}
	}

	countKey := fmt.Sprintf("%s:endpoint:%d:count", r.KeyPrefix, endpointID)
	if err := r.sendCommand(conn, br, "INCR", countKey); err != nil {
		return fmt.Errorf("redis incr: %w", err)
	}

	nameKey := fmt.Sprintf("%s:endpoint:%d:name", r.KeyPrefix, endpointID)
	if err := r.sendCommand(conn, br, "SET", nameKey, endpointName); err != nil {
		return fmt.Errorf("redis set name: %w", err)
	}

	lastTitleKey := fmt.Sprintf("%s:endpoint:%d:last_title", r.KeyPrefix, endpointID)
	if err := r.sendCommand(conn, br, "SET", lastTitleKey, title); err != nil {
		return fmt.Errorf("redis set last_title: %w", err)
	}

	lastMessageKey := fmt.Sprintf("%s:endpoint:%d:last_message", r.KeyPrefix, endpointID)
	if err := r.sendCommand(conn, br, "SET", lastMessageKey, message); err != nil {
		return fmt.Errorf("redis set last_message: %w", err)
	}

	return nil
}

// sendCommand writes a RESP array command and checks for an OK/integer response.
func (r *Redis) sendCommand(conn net.Conn, br *bufio.Reader, args ...string) error {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("*%d\r\n", len(args)))
	for _, arg := range args {
		b.WriteString(fmt.Sprintf("$%d\r\n%s\r\n", len(arg), arg))
	}

	if _, err := conn.Write([]byte(b.String())); err != nil {
		return err
	}

	line, err := br.ReadString('\n')
	if err != nil {
		return err
	}

	line = strings.TrimRight(line, "\r\n")

	switch {
	case strings.HasPrefix(line, "+"):
		// Simple string (e.g., +OK)
		if line != "+OK" {
			return fmt.Errorf("redis unexpected response: %s", line)
		}
	case strings.HasPrefix(line, ":"):
		// Integer (e.g., :1) — OK for INCR
		return nil
	case strings.HasPrefix(line, "-"):
		// Error
		return fmt.Errorf("redis error: %s", line[1:])
	case strings.HasPrefix(line, "$"):
		// Bulk string — read the payload
		length, err := strconv.Atoi(line[1:])
		if err != nil {
			return err
		}
		if length >= 0 {
			buf := make([]byte, length+2) // +2 for \r\n
			if _, err := br.Read(buf); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("redis unknown response: %s", line)
	}

	return nil
}
