package files

import (
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsimple"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
)

type Config struct {
	HTTP    *HTTPConfig    `hcl:"http,block"`
	Volumes []VolumeConfig `hcl:"volume,block"`
	Discord *DiscordConfig `hcl:"discord,block"`
	Roles   []RoleConfig   `hcl:"role,block"`
}

func (c *Config) IsAdmin(discordId string) bool {
	for _, role := range c.Roles {
		if role.Admin && role.HasUserId(discordId) {
			return true
		}
	}
	return false
}

type HTTPConfig struct {
	URL      string `hcl:"url,optional"`
	ShareURL string `hcl:"share_url,optional"`
	Bind     string `hcl:"bind"`
	Secret   string `hcl:"secret"`
}

func (h HTTPConfig) BaseShareURL() string {
	return strings.TrimRight(h.ShareURL, "/")
}

func (h HTTPConfig) BaseURL() string {
	return strings.TrimRight(h.URL, "/")
}

type VolumeConfig struct {
	Name     string   `hcl:"name,label"`
	Path     string   `hcl:"path"`
	Features []string `hcl:"features,optional"`
	Roles    []string `hcl:"roles,optional"`
	Privacy  string   `hcl:"privacy,optional"`
}

type DiscordConfig struct {
	GuildId      string `hcl:"guild_id"`
	ClientId     string `hcl:"client_id"`
	ClientSecret string `hcl:"client_secret"`
	Token        string `hcl:"token"`
}

type RoleConfig struct {
	Name    string   `hcl:"name,label"`
	UserIds []string `hcl:"user_ids"`
	Admin   bool     `hcl:"admin,optional"`
}

func (r *RoleConfig) HasUserId(userId string) bool {
	for _, id := range r.UserIds {
		if id == userId {
			return true
		}
	}
	return false
}

func newHCLEvalContext() *hcl.EvalContext {
	return &hcl.EvalContext{
		Variables: map[string]cty.Value{},
		Functions: map[string]function.Function{},
	}
}

func LoadConfig(path string) (*Config, error) {
	var cfg Config
	evalCtx := newHCLEvalContext()
	err := hclsimple.DecodeFile(path, evalCtx, &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}
