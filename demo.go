package main

func demoData() []ClassifiedPR {
	return []ClassifiedPR{
		{MyReview: MyNone, OthReview: OthApproved, Activity: ActMine, RepoName: "billing-svc", Number: 342, Author: "samantha", Title: "Add invoice PDF generation endpoint"},
		{MyReview: MyNone, OthReview: OthMixed, Activity: ActOthers, RepoName: "web-app", Number: 891, Author: "danielk", Title: "Fix timezone handling in scheduler"},
		{MyReview: MyStale, OthReview: OthNone, Activity: ActNone, RepoName: "billing-svc", Number: 339, Author: "rchen", Title: "Update Stripe webhook handler for new API version"},
		{MyReview: MyApproved, OthReview: OthApproved, Activity: ActNone, RepoName: "auth-svc", Number: 156, Author: "rchen", Title: "Add SAML SSO support for enterprise accounts"},
		{MyReview: MyChanges, OthReview: OthMixed, Activity: ActNone, RepoName: "billing-svc", Number: 337, Author: "mlopez", Title: "Refactor subscription tier logic"},
		{MyReview: MyNone, OthReview: OthNone, Activity: ActNone, RepoName: "web-app", Number: 885, Author: "jpark", Title: "Dark mode toggle in user preferences"},
		{MyReview: MyNone, OthReview: OthNone, Activity: ActOthers, RepoName: "deploy-tools", Number: 78, Author: "danielk", Title: "Add canary deployment support to rollout script"},
		{MyReview: MyApproved, OthReview: OthChanges, Activity: ActMine, RepoName: "api-gateway", Number: 214, Author: "samantha", Title: "Rate limiting per API key"},
		{MyReview: MyNone, OthReview: OthApproved, Activity: ActNone, RepoName: "web-app", Number: 882, Author: "mlopez", Title: "Migrate user settings page to React 19"},
		{MyReview: MyStale, OthReview: OthNone, Activity: ActMine, RepoName: "auth-svc", Number: 153, Author: "jpark", Title: "Fix session expiry race condition"},
		{MyReview: MyNone, OthReview: OthNone, Activity: ActNone, RepoName: "data-pipeline", Number: 45, Author: "rchen", Title: "Add retry logic for failed ETL jobs"},
		{MyReview: MyChanges, OthReview: OthNone, Activity: ActOthers, RepoName: "web-app", Number: 878, Author: "danielk", Title: "Accessibility improvements for nav components"},
	}
}
