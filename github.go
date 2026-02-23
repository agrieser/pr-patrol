package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"time"
)

type PRNode struct {
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	Number    int       `json:"number"`
	IsDraft   bool      `json:"isDraft"`
	CreatedAt time.Time `json:"createdAt"`
	Author    struct {
		Login string `json:"login"`
	} `json:"author"`
	Repository struct {
		Name          string `json:"name"`
		NameWithOwner string `json:"nameWithOwner"`
	} `json:"repository"`
	Reviews struct {
		Nodes []ReviewNode `json:"nodes"`
	} `json:"reviews"`
	Comments struct {
		Nodes []CommentNode `json:"nodes"`
	} `json:"comments"`
	Commits struct {
		Nodes []CommitNode `json:"nodes"`
	} `json:"commits"`
	ReviewRequests struct {
		Nodes []ReviewRequestNode `json:"nodes"`
	} `json:"reviewRequests"`
}

type ReviewNode struct {
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
	State       string    `json:"state"`
	SubmittedAt time.Time `json:"submittedAt"`
}

type CommentNode struct {
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
	CreatedAt time.Time `json:"createdAt"`
}

type CommitNode struct {
	Commit struct {
		CommittedDate time.Time `json:"committedDate"`
	} `json:"commit"`
}

type ReviewRequestNode struct {
	AsCodeOwner       bool `json:"asCodeOwner"`
	RequestedReviewer struct {
		Login string `json:"login"`
		Slug  string `json:"slug"`
	} `json:"requestedReviewer"`
}

type searchResult struct {
	Data struct {
		Search struct {
			PageInfo struct {
				HasNextPage bool   `json:"hasNextPage"`
				EndCursor   string `json:"endCursor"`
			} `json:"pageInfo"`
			Nodes []PRNode `json:"nodes"`
		} `json:"search"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

const graphQLQuery = `query($searchQuery: String!, $cursor: String) {
  search(query: $searchQuery, type: ISSUE, first: 100, after: $cursor) {
    pageInfo {
      hasNextPage
      endCursor
    }
    nodes {
      ... on PullRequest {
        title
        url
        number
        createdAt
        isDraft
        author { login }
        repository { name nameWithOwner }
        reviews(last: 100) {
          nodes {
            author { login }
            state
            submittedAt
          }
        }
        comments(last: 100) {
          nodes {
            author { login }
            createdAt
          }
        }
        commits(last: 1) {
          nodes {
            commit { committedDate }
          }
        }
        reviewRequests(first: 100) {
          nodes {
            asCodeOwner
            requestedReviewer {
              ... on User { login }
              ... on Team { slug }
            }
          }
        }
      }
    }
  }
}`

func ghToken() (string, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return "", fmt.Errorf("GITHUB_TOKEN environment variable is required")
	}
	return token, nil
}

func ghRequest(method, url string, body io.Reader) ([]byte, error) {
	token, err := ghToken()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(data))
	}

	return data, nil
}

// ghRequestPaginated fetches all pages of a paginated REST endpoint.
var linkNextRE = regexp.MustCompile(`<([^>]+)>;\s*rel="next"`)

func ghRequestPaginated(url string) ([]byte, error) {
	token, err := ghToken()
	if err != nil {
		return nil, err
	}

	var allData []json.RawMessage
	nextURL := url

	for nextURL != "" {
		req, err := http.NewRequest("GET", nextURL, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Accept", "application/vnd.github+json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("HTTP request failed: %w", err)
		}

		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("reading response: %w", err)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, string(data))
		}

		var page []json.RawMessage
		if err := json.Unmarshal(data, &page); err != nil {
			return nil, fmt.Errorf("parsing paginated response: %w", err)
		}
		allData = append(allData, page...)

		// Parse Link header for next page
		nextURL = ""
		if link := resp.Header.Get("Link"); link != "" {
			if m := linkNextRE.FindStringSubmatch(link); len(m) == 2 {
				nextURL = m[1]
			}
		}
	}

	return json.Marshal(allData)
}

func parseUserTeams(data []byte, org string) (map[string]bool, error) {
	var teams []struct {
		Slug         string `json:"slug"`
		Organization struct {
			Login string `json:"login"`
		} `json:"organization"`
	}
	if err := json.Unmarshal(data, &teams); err != nil {
		return nil, fmt.Errorf("parsing teams response: %w", err)
	}
	result := make(map[string]bool)
	for _, t := range teams {
		if t.Organization.Login == org {
			result[t.Slug] = true
		}
	}
	return result, nil
}

func fetchUserTeams(org string) (map[string]bool, error) {
	out, err := ghRequestPaginated("https://api.github.com/user/teams?per_page=100")
	if err != nil {
		return nil, fmt.Errorf("fetching teams: %w", err)
	}
	return parseUserTeams(out, org)
}

func fetchCurrentUser() (string, error) {
	out, err := ghRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return "", fmt.Errorf("fetching current user: %w", err)
	}
	var user struct {
		Login string `json:"login"`
	}
	if err := json.Unmarshal(out, &user); err != nil {
		return "", fmt.Errorf("parsing user response: %w", err)
	}
	if user.Login == "" {
		return "", fmt.Errorf("could not determine GitHub username; is your GITHUB_TOKEN valid?")
	}
	return user.Login, nil
}

func postComment(repo string, number int, body string) error {
	url := fmt.Sprintf("https://api.github.com/repos/%s/issues/%d/comments", repo, number)
	payload, _ := json.Marshal(map[string]string{"body": body})
	_, err := ghRequest("POST", url, bytes.NewReader(payload))
	return err
}

func fetchOpenPRs(org string) ([]PRNode, error) {
	var allPRs []PRNode
	searchQuery := fmt.Sprintf("is:pr is:open org:%s", org)
	var cursor *string

	for {
		variables := map[string]interface{}{
			"searchQuery": searchQuery,
			"cursor":      cursor,
		}
		payload, _ := json.Marshal(map[string]interface{}{
			"query":     graphQLQuery,
			"variables": variables,
		})

		out, err := ghRequest("POST", "https://api.github.com/graphql", bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("GraphQL query failed: %w", err)
		}

		var result searchResult
		if err := json.Unmarshal(out, &result); err != nil {
			return nil, fmt.Errorf("parsing GraphQL response: %w", err)
		}
		if len(result.Errors) > 0 {
			return nil, fmt.Errorf("GraphQL error: %s", result.Errors[0].Message)
		}

		for _, node := range result.Data.Search.Nodes {
			if node.Number == 0 {
				continue // skip non-PR nodes
			}
			allPRs = append(allPRs, node)
		}

		if !result.Data.Search.PageInfo.HasNextPage {
			break
		}
		c := result.Data.Search.PageInfo.EndCursor
		cursor = &c
	}

	return allPRs, nil
}
