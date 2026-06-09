package vault

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	vaultapi "github.com/hashicorp/vault/api"

	"monitoring/config"
)

type Client struct {
	api   *vaultapi.Client
	mount string
	cfg   *config.Config
}

func NewClient(cfg *config.Config) (*Client, error) {
	vcfg := vaultapi.DefaultConfig()
	vcfg.Address = cfg.VaultAddr

	raw, err := vaultapi.NewClient(vcfg)
	if err != nil {
		return nil, fmt.Errorf("vault client: %w", err)
	}

	c := &Client{api: raw, mount: cfg.VaultMount, cfg: cfg}
	if err := c.login(); err != nil {
		return nil, fmt.Errorf("vault login: %w", err)
	}

	go c.refreshLoop()
	return c, nil
}

func (c *Client) login() error {
	secret, err := c.api.Logical().Write("auth/approle/login", map[string]interface{}{
		"role_id":   c.cfg.VaultRoleID,
		"secret_id": c.cfg.VaultSecretID,
	})
	if err != nil {
		return err
	}
	c.api.SetToken(secret.Auth.ClientToken)
	slog.Info("vault authenticated")
	return nil
}

func (c *Client) refreshLoop() {
	ticker := time.NewTicker(45 * time.Minute)
	for range ticker.C {
		if err := c.login(); err != nil {
			slog.Error("vault re-auth failed", "err", err)
		}
	}
}

// ReadSecret fetches KV v2 data at the given path and returns string key-value pairs.
func (c *Client) ReadSecret(ctx context.Context, path string) (map[string]string, error) {
	secret, err := c.api.KVv2(c.mount).Get(ctx, path)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(secret.Data))
	for k, v := range secret.Data {
		if s, ok := v.(string); ok {
			result[k] = s
		}
	}
	return result, nil
}

func (c *Client) WriteSecret(ctx context.Context, path string, data map[string]interface{}) error {
	_, err := c.api.KVv2(c.mount).Put(ctx, path, data)
	return err
}

func (c *Client) DeleteSecret(ctx context.Context, path string) error {
	return c.api.KVv2(c.mount).Delete(ctx, path)
}
