package main

import (
	"encoding/json"
	"testing"
	"time"
)

const sampleGraphQLResponse = `{
  "data": {
    "search": {
      "pageInfo": {
        "hasNextPage": false,
        "endCursor": "Y3Vyc29yOjE="
      },
      "nodes": [
        {
          "title": "Add new feature",
          "url": "https://github.com/org/repo/pull/42",
          "number": 42,
          "createdAt": "2025-01-15T10:00:00Z",
          "author": { "login": "alice" },
          "repository": { "name": "repo", "nameWithOwner": "org/repo" },
          "reviews": {
            "nodes": [
              {
                "author": { "login": "bob" },
                "state": "APPROVED",
                "submittedAt": "2025-01-16T10:00:00Z"
              }
            ]
          },
          "comments": {
            "nodes": [
              { "author": { "login": "carol" } }
            ]
          },
          "commits": {
            "nodes": [
              { "commit": { "committedDate": "2025-01-15T09:00:00Z" } }
            ]
          },
          "reviewRequests": {
            "nodes": [
              {
                "asCodeOwner": true,
                "requestedReviewer": { "login": "bob" }
              },
              {
                "asCodeOwner": false,
                "requestedReviewer": { "slug": "backend-team" }
              }
            ]
          }
        },
        {},
        {
          "title": "Fix bug",
          "url": "https://github.com/org/repo/pull/43",
          "number": 43,
          "createdAt": "2025-01-14T10:00:00Z",
          "author": { "login": "dave" },
          "repository": { "name": "repo", "nameWithOwner": "org/repo" },
          "reviews": { "nodes": [] },
          "comments": { "nodes": [] },
          "commits": {
            "nodes": [
              { "commit": { "committedDate": "2025-01-14T09:00:00Z" } }
            ]
          },
          "reviewRequests": { "nodes": [] }
        }
      ]
    }
  }
}`

func TestParseSearchResult(t *testing.T) {
	var result searchResult
	if err := json.Unmarshal([]byte(sampleGraphQLResponse), &result); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	nodes := result.Data.Search.Nodes
	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes (including empty), got %d", len(nodes))
	}

	// First node: real PR
	pr := nodes[0]
	if pr.Title != "Add new feature" {
		t.Fatalf("expected 'Add new feature', got %q", pr.Title)
	}
	if pr.Number != 42 {
		t.Fatalf("expected number 42, got %d", pr.Number)
	}
	if pr.Author.Login != "alice" {
		t.Fatalf("expected author 'alice', got %q", pr.Author.Login)
	}
	if pr.Repository.NameWithOwner != "org/repo" {
		t.Fatalf("expected 'org/repo', got %q", pr.Repository.NameWithOwner)
	}
	if len(pr.Reviews.Nodes) != 1 {
		t.Fatalf("expected 1 review, got %d", len(pr.Reviews.Nodes))
	}
	if pr.Reviews.Nodes[0].State != "APPROVED" {
		t.Fatalf("expected APPROVED, got %q", pr.Reviews.Nodes[0].State)
	}
	if len(pr.Comments.Nodes) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(pr.Comments.Nodes))
	}
	if len(pr.Commits.Nodes) != 1 {
		t.Fatalf("expected 1 commit, got %d", len(pr.Commits.Nodes))
	}

	// Review requests
	if len(pr.ReviewRequests.Nodes) != 2 {
		t.Fatalf("expected 2 review requests, got %d", len(pr.ReviewRequests.Nodes))
	}
	rr := pr.ReviewRequests.Nodes[0]
	if !rr.AsCodeOwner {
		t.Fatal("expected first review request to be asCodeOwner")
	}
	if rr.RequestedReviewer.Login != "bob" {
		t.Fatalf("expected requested reviewer 'bob', got %q", rr.RequestedReviewer.Login)
	}
	rrTeam := pr.ReviewRequests.Nodes[1]
	if rrTeam.AsCodeOwner {
		t.Fatal("expected second review request to NOT be asCodeOwner")
	}
	if rrTeam.RequestedReviewer.Slug != "backend-team" {
		t.Fatalf("expected team slug 'backend-team', got %q", rrTeam.RequestedReviewer.Slug)
	}

	// Second node: empty (non-PR)
	if nodes[1].Number != 0 {
		t.Fatalf("expected empty node with number 0, got %d", nodes[1].Number)
	}

	// Pagination
	if result.Data.Search.PageInfo.HasNextPage {
		t.Fatal("expected hasNextPage=false")
	}
	if result.Data.Search.PageInfo.EndCursor != "Y3Vyc29yOjE=" {
		t.Fatalf("unexpected endCursor: %q", result.Data.Search.PageInfo.EndCursor)
	}
}

func TestParseSearchResult_WithErrors(t *testing.T) {
	errJSON := `{"errors": [{"message": "Something went wrong"}], "data": null}`
	var result searchResult
	if err := json.Unmarshal([]byte(errJSON), &result); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(result.Errors))
	}
	if result.Errors[0].Message != "Something went wrong" {
		t.Fatalf("unexpected error message: %q", result.Errors[0].Message)
	}
}

func TestParseSearchResult_DraftField(t *testing.T) {
	draftJSON := `{
	  "data": {
	    "search": {
	      "pageInfo": { "hasNextPage": false, "endCursor": "x" },
	      "nodes": [
	        {
	          "title": "WIP feature",
	          "url": "https://github.com/org/repo/pull/99",
	          "number": 99,
	          "isDraft": true,
	          "createdAt": "2025-01-15T10:00:00Z",
	          "author": { "login": "alice" },
	          "repository": { "name": "repo", "nameWithOwner": "org/repo" },
	          "reviews": { "nodes": [] },
	          "comments": { "nodes": [] },
	          "commits": { "nodes": [{ "commit": { "committedDate": "2025-01-15T09:00:00Z" } }] },
	          "reviewRequests": { "nodes": [] }
	        }
	      ]
	    }
	  }
	}`
	var result searchResult
	if err := json.Unmarshal([]byte(draftJSON), &result); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	pr := result.Data.Search.Nodes[0]
	if !pr.IsDraft {
		t.Fatal("expected isDraft to be true")
	}
}

func TestParseSearchResult_CommentCreatedAt(t *testing.T) {
	commentJSON := `{
	  "data": {
	    "search": {
	      "pageInfo": { "hasNextPage": false, "endCursor": "x" },
	      "nodes": [
	        {
	          "title": "Test",
	          "url": "https://github.com/org/repo/pull/50",
	          "number": 50,
	          "isDraft": false,
	          "createdAt": "2025-01-15T10:00:00Z",
	          "author": { "login": "alice" },
	          "repository": { "name": "repo", "nameWithOwner": "org/repo" },
	          "reviews": { "nodes": [] },
	          "comments": { "nodes": [
	            { "author": { "login": "bob" }, "createdAt": "2025-01-16T10:00:00Z" }
	          ] },
	          "commits": { "nodes": [{ "commit": { "committedDate": "2025-01-15T09:00:00Z" } }] },
	          "reviewRequests": { "nodes": [] }
	        }
	      ]
	    }
	  }
	}`
	var result searchResult
	if err := json.Unmarshal([]byte(commentJSON), &result); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	comment := result.Data.Search.Nodes[0].Comments.Nodes[0]
	expected := time.Date(2025, 1, 16, 10, 0, 0, 0, time.UTC)
	if !comment.CreatedAt.Equal(expected) {
		t.Fatalf("expected comment createdAt %v, got %v", expected, comment.CreatedAt)
	}
}

func TestParseTeamsResponse(t *testing.T) {
	teamsJSON := `[
		{"slug": "backend", "organization": {"login": "myorg"}},
		{"slug": "frontend", "organization": {"login": "myorg"}},
		{"slug": "other-team", "organization": {"login": "otherorg"}}
	]`
	teams, err := parseUserTeams([]byte(teamsJSON), "myorg")
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	if len(teams) != 2 {
		t.Fatalf("expected 2 teams for myorg, got %d", len(teams))
	}
	if !teams["backend"] {
		t.Fatal("expected 'backend' team")
	}
	if !teams["frontend"] {
		t.Fatal("expected 'frontend' team")
	}
	if teams["other-team"] {
		t.Fatal("expected 'other-team' to be excluded (different org)")
	}
}

func TestParseTeamsResponse_Empty(t *testing.T) {
	teams, err := parseUserTeams([]byte(`[]`), "myorg")
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	if len(teams) != 0 {
		t.Fatalf("expected 0 teams, got %d", len(teams))
	}
}

func TestParseUserResponse(t *testing.T) {
	userJSON := `{"login": "testuser"}`
	var user struct {
		Login string `json:"login"`
	}
	if err := json.Unmarshal([]byte(userJSON), &user); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	if user.Login != "testuser" {
		t.Fatalf("expected 'testuser', got %q", user.Login)
	}
}
