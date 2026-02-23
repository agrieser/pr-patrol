package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

type PRNode struct {
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	Number    int       `json:"number"`
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
}

type CommitNode struct {
	Commit struct {
		CommittedDate time.Time `json:"committedDate"`
	} `json:"commit"`
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
          }
        }
        commits(last: 1) {
          nodes {
            commit { committedDate }
          }
        }
      }
    }
  }
}`

func fetchCurrentUser() (string, error) {
	out, err := exec.Command("gh", "api", "/user").Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", fmt.Errorf("gh CLI not found; install from https://cli.github.com")
		}
		if execErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("gh api /user failed: %s", string(execErr.Stderr))
		}
		return "", fmt.Errorf("gh api /user failed: %w", err)
	}
	var user struct {
		Login string `json:"login"`
	}
	if err := json.Unmarshal(out, &user); err != nil {
		return "", fmt.Errorf("parsing user response: %w", err)
	}
	if user.Login == "" {
		return "", fmt.Errorf("could not determine GitHub username; are you authenticated? Run 'gh auth login'")
	}
	return user.Login, nil
}

func fetchOpenPRs(org string) ([]PRNode, error) {
	var allPRs []PRNode
	searchQuery := fmt.Sprintf("is:pr is:open org:%s", org)
	var cursor *string

	for {
		args := []string{"api", "graphql",
			"-f", fmt.Sprintf("query=%s", graphQLQuery),
			"-f", fmt.Sprintf("searchQuery=%s", searchQuery),
		}

		if cursor != nil {
			args = append(args, "-f", fmt.Sprintf("cursor=%s", *cursor))
		} else {
			args = append(args, "-F", "cursor=null")
		}

		out, err := exec.Command("gh", args...).Output()
		if err != nil {
			if execErr, ok := err.(*exec.ExitError); ok {
				return nil, fmt.Errorf("GraphQL query failed: %s", string(execErr.Stderr))
			}
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
