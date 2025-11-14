package jira

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"telegram-bot-jira/internal/common"
	"telegram-bot-jira/internal/config"
	"time"
)

type Client struct {
	baseURL    string
	projectKey string
	issueType  string
	authHeader string
	http       *http.Client
}

type issueTransition struct {
	ID   string
	Name string
	To   struct {
		Name string
	} `json:"to"`
}

func New(cfg config.Config) (*Client, error) {
	if cfg.JiraBaseURL == "" || cfg.JiraEmail == "" || cfg.JiraAPIToken == "" || cfg.JiraProjectKey == "" || cfg.JiraIssueType == "" {
		return nil, errors.New("jira: baseURL, email, apiToken, projectKey, issueType are required")
	}
	cfg.JiraBaseURL = strings.TrimRight(cfg.JiraBaseURL, "/")
	creds := base64.StdEncoding.EncodeToString([]byte(cfg.JiraEmail + ":" + cfg.JiraAPIToken))
	return &Client{
		baseURL:    cfg.JiraBaseURL,
		projectKey: cfg.JiraProjectKey,
		issueType:  cfg.JiraIssueType,
		authHeader: "Basic " + creds,
		http:       &http.Client{Timeout: 15 * time.Second},
	}, nil
}

// ProjectKey returns configured Jira project key.
func (c *Client) ProjectKey() string { return c.projectKey }

// BrowseURL builds an URL to view issue in browser.
func (c *Client) BrowseURL(key string) string { return c.baseURL + "/browse/" + strings.TrimSpace(key) }

type createIssueRequest struct {
	Fields map[string]any `json:"fields"`
}

type createIssueResponse struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Self string `json:"self"`
}

// CreateIssue creates a Jira issue using REST v3 API.
func (c *Client) CreateIssue(ctx context.Context, summary string, description any) (string, string, error) {
	if strings.TrimSpace(summary) == "" {
		return "", "", errors.New("jira: summary is required")
	}

	// Decide whether issuetype is provided as numeric ID or as name
	issueTypeField := map[string]any{}
	if _, err := strconv.Atoi(c.issueType); err == nil {
		issueTypeField["id"] = c.issueType
	} else {
		issueTypeField["name"] = c.issueType
	}

	fields := map[string]any{
		"project":   map[string]any{"key": c.projectKey},
		"issuetype": issueTypeField,
		"summary":   summary,
		"labels":    []string{"telegram"},
	}
	switch d := description.(type) {
	case nil:
		// no description
	case string:
		if strings.TrimSpace(d) != "" {
			fields["description"] = map[string]any{
				"type":    "doc",
				"version": 1,
				"content": []any{
					map[string]any{
						"type": "paragraph",
						"content": []any{
							map[string]any{"type": "text", "text": d},
						},
					},
				},
			}
		}
	case map[string]any:
		fields["description"] = d
	default:
		// best-effort: stringify
		s := strings.TrimSpace(fmt.Sprint(d))
		if s != "" {
			fields["description"] = map[string]any{
				"type":    "doc",
				"version": 1,
				"content": []any{
					map[string]any{
						"type": "paragraph",
						"content": []any{
							map[string]any{"type": "text", "text": s},
						},
					},
				},
			}
		}
	}
	body, _ := json.Marshal(createIssueRequest{Fields: fields})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/rest/api/3/issue", strings.NewReader(string(body)))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusCreated {
		return "", "", fmt.Errorf("jira: create failed (%d): %s", resp.StatusCode, truncate(string(data), 512))
	}
	var out createIssueResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return "", "", err
	}
	return out.Key, c.baseURL + "/browse/" + out.Key, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// ErrNotFound indicates that the requested issue does not exist or is not accessible.
var ErrNotFound = errors.New("jira: issue not found")

// IssueStatus is a lightweight view of a Jira issue used for status responses.
type IssueStatus struct {
	Key      string
	Summary  string
	Status   string
	Assignee string
	Priority string
	Created  time.Time
	Updated  time.Time
}

// GetIssueStatus fetches minimal fields of an issue required to render status.
func (c *Client) GetIssueStatus(ctx context.Context, key string) (*IssueStatus, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, errors.New("jira: issue key is required")
	}

	url := c.baseURL + "/rest/api/3/issue/" + key + "?fields=summary,status,assignee,priority,created,updated"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jira: get issue failed (%d): %s", resp.StatusCode, truncate(string(data), 512))
	}

	// Minimal JSON structure for fields we care about
	var raw struct {
		Key    string `json:"key"`
		Fields struct {
			Summary string `json:"summary"`
			Status  *struct {
				Name string `json:"name"`
			} `json:"status"`
			Assignee *struct {
				DisplayName string `json:"displayName"`
			} `json:"assignee"`
			Priority *struct {
				Name string `json:"name"`
			} `json:"priority"`
			Created string `json:"created"`
			Updated string `json:"updated"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	var out IssueStatus
	out.Key = raw.Key
	out.Summary = raw.Fields.Summary
	if raw.Fields.Status != nil {
		out.Status = raw.Fields.Status.Name
	}
	if raw.Fields.Assignee != nil {
		out.Assignee = raw.Fields.Assignee.DisplayName
	}
	if raw.Fields.Priority != nil {
		out.Priority = raw.Fields.Priority.Name
	}
	out.Created = parseJiraTime(raw.Fields.Created)
	out.Updated = parseJiraTime(raw.Fields.Updated)

	return &out, nil
}

var jiraTimeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05.999999-0700",
	"2006-01-02T15:04:05.999-0700",
	"2006-01-02T15:04:05.000-0700",
	"2006-01-02T15:04:05-0700",
	"2006-01-02T15:04:05.999999Z0700",
	"2006-01-02T15:04:05.999Z0700",
}

func parseJiraTime(s string) time.Time {
	if t, ok := parseJiraTimeFlexible(s); ok {
		return t
	}
	return time.Time{}
}

func parseJiraTimeFlexible(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range jiraTimeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// GetIssueDescriptionADF fetches issue description as ADF document (raw map).
func (c *Client) GetIssueDescriptionADF(ctx context.Context, key string) (map[string]any, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, errors.New("jira: issue key is required")
	}
	url := c.baseURL + "/rest/api/3/issue/" + key + "?fields=description"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jira: get description failed (%d): %s", resp.StatusCode, truncate(string(data), 512))
	}
	var raw struct {
		Fields struct {
			Description map[string]any `json:"description"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	return raw.Fields.Description, nil
}

// UpdateIssueDescriptionADF updates issue description with provided ADF doc.
func (c *Client) UpdateIssueDescriptionADF(ctx context.Context, key string, doc map[string]any) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("jira: issue key is required")
	}
	body, _ := json.Marshal(map[string]any{
		"fields": map[string]any{
			"description": doc,
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+"/rest/api/3/issue/"+key, strings.NewReader(string(body)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira: update description failed (%d): %s", resp.StatusCode, truncate(string(data), 512))
	}
	return nil
}

// TransitionIssueToStatus moves an issue to a status reachable via available transitions.
func (c *Client) TransitionIssueToStatus(ctx context.Context, key, statusName string) error {
	key = strings.TrimSpace(key)
	statusName = strings.TrimSpace(statusName)
	if key == "" {
		return errors.New("jira: issue key is required")
	}
	if statusName == "" {
		return errors.New("jira: status name is required")
	}
	transitions, err := c.issueTransitions(ctx, key)
	if err != nil {
		return err
	}
	var transitionID string
	for _, t := range transitions {
		if strings.EqualFold(t.To.Name, statusName) || strings.EqualFold(t.Name, statusName) {
			transitionID = t.ID
			break
		}
	}
	if transitionID == "" {
		return fmt.Errorf("jira: transition to status %q not found", statusName)
	}
	payload, _ := json.Marshal(map[string]any{
		"transition": map[string]string{"id": transitionID},
	})
	url := c.baseURL + "/rest/api/3/issue/" + key + "/transitions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira: transition failed (%d): %s", resp.StatusCode, truncate(string(data), 512))
	}
	return nil
}

// AddComment adds a plain text comment to the issue using ADF payload.
func (c *Client) AddComment(ctx context.Context, key, body string) error {
	key = strings.TrimSpace(key)
	body = strings.TrimSpace(body)
	if key == "" {
		return errors.New("jira: issue key is required")
	}
	if body == "" {
		return errors.New("jira: comment body is empty")
	}
	paragraph := map[string]any{
		"type":    "paragraph",
		"content": []any{},
	}
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if i > 0 {
			paragraph["content"] = append(paragraph["content"].([]any), map[string]any{"type": "hardBreak"})
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		paragraph["content"] = append(paragraph["content"].([]any), map[string]any{
			"type": "text",
			"text": line,
		})
	}
	if len(paragraph["content"].([]any)) == 0 {
		paragraph["content"] = append(paragraph["content"].([]any), map[string]any{
			"type": "text",
			"text": body,
		})
	}
	payload, _ := json.Marshal(map[string]any{
		"body": map[string]any{
			"type":    "doc",
			"version": 1,
			"content": []any{paragraph},
		},
	})
	url := c.baseURL + "/rest/api/3/issue/" + key + "/comment"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jira: add comment failed (%d): %s", resp.StatusCode, truncate(string(data), 512))
	}
	return nil
}

// AddCommentReaction adds reaction to a Jira comment to acknowledge processing.
func (c *Client) AddCommentReaction(ctx context.Context, key, commentID, reaction string) error {
	key = strings.TrimSpace(key)
	commentID = strings.TrimSpace(commentID)
	reaction = strings.TrimSpace(reaction)
	if key == "" {
		return errors.New("jira: issue key is required")
	}
	if commentID == "" {
		return errors.New("jira: comment id is required")
	}
	if reaction == "" {
		return errors.New("jira: reaction is required")
	}

	payload, _ := json.Marshal(map[string]any{
		"reaction": map[string]string{
			"emojiId": reaction,
		},
	})

	issueURL := fmt.Sprintf("%s/rest/api/3/issue/%s/comment/%s/reaction", c.baseURL, key, commentID)
	if err := c.postCommentReaction(ctx, issueURL, payload); err == nil {
		return nil
	} else {
		commentURL := fmt.Sprintf("%s/rest/api/3/comment/%s/reaction", c.baseURL, commentID)
		if err2 := c.postCommentReaction(ctx, commentURL, payload); err2 == nil {
			return nil
		} else {
			return fmt.Errorf("jira: add comment reaction failed; issue path error: %v; direct path error: %v", err, err2)
		}
	}
}

func (c *Client) postCommentReaction(ctx context.Context, url string, payload []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-ExperimentalApi", "opt-in")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusNoContent {
		return nil
	}

	data, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("jira: add comment reaction failed (%d): %s", resp.StatusCode, truncate(string(data), 512))
}

func (c *Client) issueTransitions(ctx context.Context, key string) ([]issueTransition, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, errors.New("jira: issue key is required")
	}
	url := c.baseURL + "/rest/api/3/issue/" + key + "/transitions"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jira: get transitions failed (%d): %s", resp.StatusCode, truncate(string(data), 512))
	}
	var out struct {
		Transitions []issueTransition `json:"transitions"`
	}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out.Transitions, nil
}

// JiraTime handles Jira timestamps that may appear with or without colon in the timezone.
type JiraTime struct {
	time.Time
}

// UnmarshalJSON accepts multiple Jira timestamp formats.
func (jt *JiraTime) UnmarshalJSON(data []byte) error {
	raw := strings.TrimSpace(string(data))
	if raw == "null" || raw == "" {
		jt.Time = time.Time{}
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s == "" {
		jt.Time = time.Time{}
		return nil
	}
	if t, ok := parseJiraTimeFlexible(s); ok {
		jt.Time = t
		return nil
	}
	return fmt.Errorf("jira: unable to parse time %q", s)
}

// Comment represents a Jira comment structure.
type Comment struct {
	ID           string      `json:"id"`
	Body         CommentBody `json:"body"`
	RenderedBody string      `json:"renderedBody"`
	Created      JiraTime    `json:"created"`
	Updated      JiraTime    `json:"updated"`
	Author       struct {
		DisplayName string `json:"displayName"`
		Email       string `json:"emailAddress"`
	} `json:"author"`
	UpdatesAuthor struct {
		DisplayName string `json:"displayName"`
		Email       string `json:"emailAddress"`
	} `json:"updateAuthor"`
}

// CommentBody captures Jira comment bodies returned either as plain strings or as Atlassian Document (ADF) objects.
type CommentBody struct {
	Text string
	Raw  any
}

// String returns the parsed body as plain text.
func (cb CommentBody) String() string {
	return cb.Text
}

// UnmarshalJSON handles both legacy plain string and modern ADF comment payloads.
func (cb *CommentBody) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		cb.Text = strings.TrimSpace(s)
		cb.Raw = nil
		return nil
	}

	var raw any
	if err := json.Unmarshal(data, &raw); err == nil {
		cb.Raw = raw
		text := strings.TrimSpace(extractADFPlainText(raw))
		cb.Text = text
		return nil
	}

	return fmt.Errorf("jira: unsupported comment body format")
}

func extractADFPlainText(node any) string {
	switch v := node.(type) {
	case map[string]any:
		typ, _ := v["type"].(string)
		if typ == "hardBreak" {
			return "\n"
		}
		if typ == "text" {
			if txt, _ := v["text"].(string); txt != "" {
				return txt
			}
		}
		if content, ok := v["content"].([]any); ok {
			var b strings.Builder
			for _, child := range content {
				b.WriteString(extractADFPlainText(child))
			}
			if typ == "paragraph" && b.Len() > 0 {
				b.WriteString("\n")
			}
			return b.String()
		}
	case []any:
		var b strings.Builder
		for _, child := range v {
			b.WriteString(extractADFPlainText(child))
		}
		return b.String()
	case string:
		return v
	}
	return ""
}

// GetComments retrieves all comments for a given issue key.
func (c *Client) GetComments(ctx context.Context, key string) ([]Comment, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, errors.New("jira: issue key is required")
	}

	url := c.baseURL + "/rest/api/3/issue/" + key + "/comment?expand=renderedBody"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jira: get comments failed (%d): %s", resp.StatusCode, truncate(string(data), 512))
	}

	var raw struct {
		Comments []Comment `json:"comments"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	return raw.Comments, nil
}

// AddAttachment uploads a file and attaches it to a Jira issue
func (c *Client) AddAttachment(ctx context.Context, key string, filename string, fileData []byte) (string, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return "", errors.New("jira: issue key is required")
	}
	if filename == "" {
		return "", errors.New("jira: filename is required")
	}
	if len(fileData) == 0 {
		return "", errors.New("jira: file data is empty")
	}

	url := c.baseURL + "/rest/api/3/issue/" + key + "/attachments"
	
	// Create multipart form
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	
	_, err = part.Write(fileData)
	if err != nil {
		return "", err
	}
	
	err = writer.Close()
	if err != nil {
		return "", err
	}
	
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return "", err
	}
	
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("X-Atlassian-Token", "no-check")
	req.Header.Set("Content-Type", writer.FormDataContentType())
	
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("jira: add attachment failed (%d): %s", resp.StatusCode, truncate(string(data), 512))
	}
	
	// Parse response to get attachment ID
	var attachments []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &attachments); err != nil {
		return "", err
	}
	
	if len(attachments) == 0 {
		return "", errors.New("jira: no attachment returned")
	}
	
	return attachments[0].ID, nil
}

// AddCommentWithEmbeddedImages adds a comment with embedded images to a Jira issue
func (c *Client) AddCommentWithEmbeddedImages(ctx context.Context, key, body string, imageUrls []string) error {
	key = strings.TrimSpace(key)
	body = strings.TrimSpace(body)
	if key == "" {
		return errors.New("jira: issue key is required")
	}
	if body == "" && len(imageUrls) == 0 {
		return errors.New("jira: comment body and images are empty")
	}
	
	// First, upload images as attachments
	var attachmentIds []string
	for i, imageUrl := range imageUrls {
		if imageUrl != "" {
			// Download the image
			resp, err := http.Get(imageUrl)
			if err != nil {
				fmt.Printf("DEBUG: Failed to download image %s: %v\n", imageUrl, err)
				continue
			}
			defer resp.Body.Close()
			
			if resp.StatusCode != http.StatusOK {
				fmt.Printf("DEBUG: Failed to download image %s: HTTP %d\n", imageUrl, resp.StatusCode)
				continue
			}
			
			var buf bytes.Buffer
			_, err = buf.ReadFrom(resp.Body)
			if err != nil {
				fmt.Printf("DEBUG: Failed to read image data %s: %v\n", imageUrl, err)
				continue
			}
			
			// Upload as attachment
			filename := fmt.Sprintf("telegram_image_%d.jpg", i)
			attachmentId, err := c.AddAttachment(ctx, key, filename, buf.Bytes())
			if err != nil {
				fmt.Printf("DEBUG: Failed to upload attachment %s: %v\n", filename, err)
				continue
			}
			
			attachmentIds = append(attachmentIds, attachmentId)
			fmt.Printf("DEBUG: Uploaded attachment %s with ID %s\n", filename, attachmentId)
		}
	}
	
	// Create ADF content for the comment
	content := []any{}
	
	// Add text content if provided
	if body != "" {
		paragraph := map[string]any{
			"type":    "paragraph",
			"content": []any{},
		}
		lines := strings.Split(body, "\n")
		for i, line := range lines {
			if i > 0 {
				paragraph["content"] = append(paragraph["content"].([]any), map[string]any{"type": "hardBreak"})
			}
			if strings.TrimSpace(line) == "" {
				continue
			}
			paragraph["content"] = append(paragraph["content"].([]any), map[string]any{
				"type": "text",
				"text": line,
			})
		}
		if len(paragraph["content"].([]any)) == 0 {
			paragraph["content"] = append(paragraph["content"].([]any), map[string]any{
				"type": "text",
				"text": body,
			})
		}
		content = append(content, paragraph)
	}
	
	// Add embedded images if provided
	for _, id := range attachmentIds {
		if id != "" {
			// Create an inline image using ADF format with attachment reference
			imageBlock := map[string]any{
				"type": "paragraph",
				"content": []any{
					map[string]any{
						"type": "inlineCard",
						"attrs": map[string]any{
							"url": fmt.Sprintf("%s/secure/attachment/%s/", c.baseURL, id),
						},
					},
				},
			}
			content = append(content, imageBlock)
		}
	}
	
	commentBody := map[string]any{
		"type":    "doc",
		"version": 1,
		"content": content,
	}
	
	payload, _ := json.Marshal(map[string]any{
		"body": commentBody,
	})
	
	// Debug logging
	fmt.Printf("DEBUG: Sending comment to Jira with ADF\n")
	fmt.Printf("DEBUG: URL: %s\n", c.baseURL+"/rest/api/3/issue/"+key+"/comment")
	fmt.Printf("DEBUG: Payload: %s\n", string(payload))
	
	url := c.baseURL + "/rest/api/3/issue/" + key + "/comment"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	data, _ := io.ReadAll(resp.Body)
	fmt.Printf("DEBUG: Response status: %d\n", resp.StatusCode)
	fmt.Printf("DEBUG: Response body: %s\n", string(data))
	
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("jira: add comment with embedded images failed (%d): %s", resp.StatusCode, truncate(string(data), 512))
	}
	
	return nil
}

// AddCommentWithEmbeddedFiles adds a comment with embedded files to a Jira issue
func (c *Client) AddCommentWithEmbeddedFiles(ctx context.Context, key, body string, files []common.FileInfo) error {
	key = strings.TrimSpace(key)
	body = strings.TrimSpace(body)
	if key == "" {
		return errors.New("jira: issue key is required")
	}
	if body == "" && len(files) == 0 {
		return errors.New("jira: comment body and files are empty")
	}
	
	// First, upload files as attachments
	var attachmentIds []string
	var fileNames []string
	for i, file := range files {
		if file.Url != "" {
			// Download the file
			resp, err := http.Get(file.Url)
			if err != nil {
				fmt.Printf("DEBUG: Failed to download file %s: %v\n", file.Url, err)
				continue
			}
			defer resp.Body.Close()
			
			if resp.StatusCode != http.StatusOK {
				fmt.Printf("DEBUG: Failed to download file %s: HTTP %d\n", file.Url, resp.StatusCode)
				continue
			}
			
			var buf bytes.Buffer
			_, err = buf.ReadFrom(resp.Body)
			if err != nil {
				fmt.Printf("DEBUG: Failed to read file data %s: %v\n", file.Url, err)
				continue
			}
			
			// Determine file extension from URL or use default
			filename := file.Name
			if filename == "" && strings.Contains(file.Url, ".") {
				parts := strings.Split(file.Url, ".")
				if len(parts) > 1 {
					ext := parts[len(parts)-1]
					// Remove any query parameters from extension
					if idx := strings.Index(ext, "?"); idx != -1 {
						ext = ext[:idx]
					}
					filename = fmt.Sprintf("telegram_file_%d.%s", i, ext)
				}
			}
			if filename == "" {
				filename = fmt.Sprintf("telegram_file_%d", i)
			}
			
			// Upload as attachment
			attachmentId, err := c.AddAttachment(ctx, key, filename, buf.Bytes())
			if err != nil {
				fmt.Printf("DEBUG: Failed to upload attachment %s: %v\n", filename, err)
				continue
			}
			
			attachmentIds = append(attachmentIds, attachmentId)
			fileNames = append(fileNames, filename)
			fmt.Printf("DEBUG: Uploaded attachment %s with ID %s\n", filename, attachmentId)
		}
	}
	
	// Create ADF content for the comment
	content := []any{}
	
	// Add text content if provided
	if body != "" {
		paragraph := map[string]any{
			"type":    "paragraph",
			"content": []any{},
		}
		lines := strings.Split(body, "\n")
		for i, line := range lines {
			if i > 0 {
				paragraph["content"] = append(paragraph["content"].([]any), map[string]any{"type": "hardBreak"})
			}
			if strings.TrimSpace(line) == "" {
				continue
			}
			paragraph["content"] = append(paragraph["content"].([]any), map[string]any{
				"type": "text",
				"text": line,
			})
		}
		if len(paragraph["content"].([]any)) == 0 {
			paragraph["content"] = append(paragraph["content"].([]any), map[string]any{
				"type": "text",
				"text": body,
			})
		}
		content = append(content, paragraph)
	}
	
	// Add file attachments as links
	for i, id := range attachmentIds {
		if id != "" && i < len(fileNames) {
			// Create a link to the attachment
			fileLink := map[string]any{
				"type": "paragraph",
				"content": []any{
					map[string]any{
						"type": "text",
						"text": "📎 Вложенный файл: ",
					},
					map[string]any{
						"type": "text",
						"text": fileNames[i],
						"marks": []any{
							map[string]any{
								"type": "link",
								"attrs": map[string]any{
									"href": fmt.Sprintf("%s/secure/attachment/%s/", c.baseURL, id),
								},
							},
						},
					},
				},
			}
			content = append(content, fileLink)
		}
	}
	
	commentBody := map[string]any{
		"type":    "doc",
		"version": 1,
		"content": content,
	}
	
	payload, _ := json.Marshal(map[string]any{
		"body": commentBody,
	})
	
	// Debug logging
	fmt.Printf("DEBUG: Sending comment to Jira with files\n")
	fmt.Printf("DEBUG: URL: %s\n", c.baseURL+"/rest/api/3/issue/"+key+"/comment")
	fmt.Printf("DEBUG: Payload: %s\n", string(payload))
	
	url := c.baseURL + "/rest/api/3/issue/" + key + "/comment"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader(string(payload)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.authHeader)
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	data, _ := io.ReadAll(resp.Body)
	fmt.Printf("DEBUG: Response status: %d\n", resp.StatusCode)
	fmt.Printf("DEBUG: Response body: %s\n", string(data))
	
	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("jira: add comment with files failed (%d): %s", resp.StatusCode, truncate(string(data), 512))
	}
	
	return nil
}
