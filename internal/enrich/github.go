package enrich

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "path"
    "strings"
    "time"
)

type GitHubClient struct {
    BaseAPI string // e.g., https://api.github.com
    Token   string // optional
    http    *http.Client
}

func NewGitHubClient(baseAPI, token string) *GitHubClient {
    if baseAPI == "" {
        baseAPI = "https://api.github.com"
    }
    return &GitHubClient{
        BaseAPI: baseAPI,
        Token:   token,
        http:    &http.Client{Timeout: 30 * time.Second},
    }
}

type Repo struct {
    FullName      string `json:"full_name"`
    DefaultBranch string `json:"default_branch"`
}

type Release struct {
    ID      int64  `json:"id"`
    TagName string `json:"tag_name"`
    Name    string `json:"name"`
    Body    string `json:"body"`
    Draft   bool   `json:"draft"`
    Prerelease bool `json:"prerelease"`
    PublishedAt string `json:"published_at"`
}

func (c *GitHubClient) GetRepo(ctx context.Context, owner, repo string) (Repo, error) {
    var out Repo
    u := fmt.Sprintf("%s/repos/%s/%s", strings.TrimRight(c.BaseAPI, "/"), url.PathEscape(owner), url.PathEscape(repo))
    b, err := c.get(ctx, u)
    if err != nil {
        return out, err
    }
    if err := json.Unmarshal(b, &out); err != nil {
        return out, err
    }
    return out, nil
}

func (c *GitHubClient) ListReleases(ctx context.Context, owner, repo string, page, perPage int) ([]Release, error) {
    u := fmt.Sprintf("%s/repos/%s/%s/releases?page=%d&per_page=%d", strings.TrimRight(c.BaseAPI, "/"), url.PathEscape(owner), url.PathEscape(repo), page, perPage)
    b, err := c.get(ctx, u)
    if err != nil {
        return nil, err
    }
    var out []Release
    if err := json.Unmarshal(b, &out); err != nil {
        return nil, err
    }
    return out, nil
}

func (c *GitHubClient) GetRawFile(ctx context.Context, owner, repo, ref, filePath string) ([]byte, error) {
    // Use raw.githubusercontent.com; not the API for simplicity.
    u := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, ref, path.Clean(filePath))
    return c.get(ctx, u)
}

func (c *GitHubClient) get(ctx context.Context, u string) ([]byte, error) {
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
    if err != nil {
        return nil, err
    }
    if c.Token != "" && strings.Contains(c.BaseAPI, "api.github.com") {
        req.Header.Set("Authorization", "Bearer "+c.Token)
    }
    req.Header.Set("User-Agent", "terraform-mcp-analyzer-pack/0.1 (+offline-first)")
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
