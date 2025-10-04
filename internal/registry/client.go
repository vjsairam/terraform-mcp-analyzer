package registry

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "time"
)

type Client struct {
    BaseURL   string
    UserAgent string
    http      *http.Client
}

func NewClient(baseURL string) *Client {
    if baseURL == "" {
        baseURL = "https://registry.terraform.io"
    }
    return &Client{
        BaseURL:   baseURL,
        UserAgent: "terraform-mcp-analyzer-pack/0.1 (+offline-first)",
        http:      &http.Client{Timeout: 30 * time.Second},
    }
}

// ProviderListResponse is a minimal shape for provider enumeration responses.
type ProviderListResponse struct {
    Providers []struct {
        Namespace string `json:"namespace"`
        Name      string `json:"name"`
    } `json:"providers"`
}

// ModuleListResponse is a minimal shape for module enumeration responses.
type ModuleListResponse struct {
    Modules []struct {
        Namespace string `json:"namespace"`
        Name      string `json:"name"`
        Provider  string `json:"provider"`
    } `json:"modules"`
}

// ProviderVersionsResponse for versions of a provider.
type ProviderVersionsResponse struct {
    ID       string `json:"id"`
    Versions []struct {
        Version string   `json:"version"`
        Protocols []string `json:"protocols"`
    } `json:"versions"`
}

// ModuleVersionsResponse for versions of a module.
type ModuleVersionsResponse struct {
    Modules []struct {
        Versions []struct{ Version string `json:"version"` } `json:"versions"`
    } `json:"modules"`
}

// ProviderInfoResponse attempts to capture provider metadata including source repo.
// The Registry response schema is not fully documented here; we keep fields flexible.
type ProviderInfoResponse struct {
    ID          string `json:"id"`
    Namespace   string `json:"namespace"`
    Name        string `json:"name"`
    Source      string `json:"source"`       // e.g., "github.com/hashicorp/terraform-provider-aws"
    HomepageURL string `json:"homepage_url"` // sometimes present
}

// ModuleInfoResponse for module metadata (v1 modules endpoint summary, not full search).
type ModuleInfoResponse struct {
    ID        string `json:"id"`
    Namespace string `json:"namespace"`
    Name      string `json:"name"`
    Provider  string `json:"provider"`
    Source    string `json:"source"` // e.g., "github.com/terraform-aws-modules/terraform-aws-vpc"
}

func (c *Client) ListProviders(ctx context.Context, limit, offset int) (ProviderListResponse, error) {
    var out ProviderListResponse
    u := fmt.Sprintf("%s/v1/providers?limit=%d&offset=%d", c.BaseURL, limit, offset)
    b, err := c.get(ctx, u)
    if err != nil {
        return out, err
    }
    if err := json.Unmarshal(b, &out); err != nil {
        return out, err
    }
    return out, nil
}

func (c *Client) ListModules(ctx context.Context, limit, offset int) (ModuleListResponse, error) {
    var out ModuleListResponse
    u := fmt.Sprintf("%s/v1/modules?limit=%d&offset=%d", c.BaseURL, limit, offset)
    b, err := c.get(ctx, u)
    if err != nil {
        return out, err
    }
    if err := json.Unmarshal(b, &out); err != nil {
        return out, err
    }
    return out, nil
}

func (c *Client) ProviderVersions(ctx context.Context, namespace, name string) (ProviderVersionsResponse, error) {
    var out ProviderVersionsResponse
    u := fmt.Sprintf("%s/v1/providers/%s/%s/versions", c.BaseURL, url.PathEscape(namespace), url.PathEscape(name))
    b, err := c.get(ctx, u)
    if err != nil {
        return out, err
    }
    if err := json.Unmarshal(b, &out); err != nil {
        return out, err
    }
    return out, nil
}

func (c *Client) ModuleVersions(ctx context.Context, namespace, name, provider string) (ModuleVersionsResponse, error) {
    var out ModuleVersionsResponse
    u := fmt.Sprintf("%s/v1/modules/%s/%s/%s/versions", c.BaseURL, url.PathEscape(namespace), url.PathEscape(name), url.PathEscape(provider))
    b, err := c.get(ctx, u)
    if err != nil {
        return out, err
    }
    if err := json.Unmarshal(b, &out); err != nil {
        return out, err
    }
    return out, nil
}

func (c *Client) ProviderInfo(ctx context.Context, namespace, name string) (ProviderInfoResponse, error) {
    var out ProviderInfoResponse
    u := fmt.Sprintf("%s/v1/providers/%s/%s", c.BaseURL, url.PathEscape(namespace), url.PathEscape(name))
    b, err := c.get(ctx, u)
    if err != nil {
        return out, err
    }
    if err := json.Unmarshal(b, &out); err != nil {
        return out, err
    }
    return out, nil
}

func (c *Client) ModuleInfo(ctx context.Context, namespace, name, provider string) (ModuleInfoResponse, error) {
    var out ModuleInfoResponse
    u := fmt.Sprintf("%s/v1/modules/%s/%s/%s", c.BaseURL, url.PathEscape(namespace), url.PathEscape(name), url.PathEscape(provider))
    b, err := c.get(ctx, u)
    if err != nil {
        return out, err
    }
    if err := json.Unmarshal(b, &out); err != nil {
        return out, err
    }
    return out, nil
}

func (c *Client) get(ctx context.Context, u string) ([]byte, error) {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
    if err != nil {
        return nil, err
    }
    if c.UserAgent != "" {
        req.Header.Set("User-Agent", c.UserAgent)
    }
    resp, err := c.http.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
        return nil, fmt.Errorf("http %d: %s: %s", resp.StatusCode, u, string(b))
    }
    return io.ReadAll(resp.Body)
}
