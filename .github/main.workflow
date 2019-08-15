action "Auto Assign" {
  uses = "ipfs/auto-assign@v1.0.0"
  secrets = ["GITHUB_TOKEN"]
}
workflow "Add reviewers/assignees to Pull Requests" {
  on = "pull_request"
  resolves = "Auto Assign"
}
